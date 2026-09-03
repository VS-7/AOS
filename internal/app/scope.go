package app

import (
	"context"
	"sync"
)

// eventScope answers the workspace id every event this process publishes
// belongs to.
//
// It cannot be a string decided at wiring time, and that was the defect. The
// `active` id wire.go computes is only non-empty when the scope was *pinned*:
// a secondary workspace this daemon resolved for a caller, or an operator's
// AOS_WORKSPACE_ID. The primary scope of a desktop or CLI installation has
// neither — the supervisor passes AOS_WORKSPACE_PATH and never an id
// (cmd/aos-desktop's daemonEnv), and the workspace is registered *after* the
// daemon is already up, by the window's own workspace_introspect. So `active`
// was "" in the normal case and every event went out on ChannelFor(""), a
// channel no socket subscribes to.
//
// The consequences were the whole of "nothing updates by itself": the board
// and the inbox never moved, a project or goal an agent created never
// appeared, and — worst — the approval dialog never opened, so every
// approval-gated tool waited out its deadline and was denied. The agent
// reported it could not do the thing; the person was never asked.
//
// Chat events were given a lazy resolver for exactly this reason (see
// publisher.channelFor's own comment); the activity sink, the collection
// publisher and the approval notifier were not. This is that resolver, made
// once and shared by all four, so the next publisher added cannot get it
// wrong.
//
// Resolution is lazy and cached because the registry is readable only after
// boot: anything computed at wiring time would still be empty.
type eventScope struct {
	// fixed is the pinned id, when there is one. A pinned scope never
	// consults the registry: it is the answer.
	fixed string

	mu     sync.RWMutex
	lookup func(context.Context, string) string
}

// newEventScope builds the scope for a set of services. An empty id means the
// primary scope, whose workspace is resolved later through resolveWith.
func newEventScope(fixed string) *eventScope { return &eventScope{fixed: fixed} }

// resolveWith installs the registry lookup, which only exists once the
// workspace service has been built. Until it is installed — and until the
// workspace is registered — ID answers "", which is the honest answer for a
// daemon that is not yet serving one.
func (s *eventScope) resolveWith(lookup func(context.Context, string) string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookup = lookup
}

// ID is the workspace id to publish under, right now.
func (s *eventScope) ID(ctx context.Context) string {
	if s == nil {
		return ""
	}
	if s.fixed != "" {
		return s.fixed
	}
	s.mu.RLock()
	lookup := s.lookup
	s.mu.RUnlock()
	if lookup == nil {
		return ""
	}
	return lookup(ctx, "")
}
