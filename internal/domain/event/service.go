package event

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/safe"
)

// DefaultHookTimeout bounds one handler.
//
// It is an addition. A hook is an arbitrary program, often a shell script
// somebody wrote in five minutes, and a turn that hangs because one of them
// blocked on stdin is a turn nobody can explain.
const DefaultHookTimeout = 30 * time.Second

// Deps is what the bus is built from.
type Deps struct {
	Log   Log
	Clock Clock
	IDs   IDs

	// Timeout bounds one handler. Zero means DefaultHookTimeout.
	Timeout time.Duration

	// Strict turns a failing hook into a failing turn. It is off by default:
	// a broken hook should not stop the agent from working, and the record of
	// the failure is in the log either way.
	Strict bool

	Logger *slog.Logger
}

// Service is the hook bus: registration, dispatch and the audit trail.
//
// The bus lives in the domain rather than in core, which is a divergence from
// the specification. The reason is the dependency rule: the bus is written in
// terms of Event and Outcome, so putting it in core would make core import the
// domain — the arrow this architecture spends a test keeping pointed the other
// way.
type Service struct {
	log     Log
	clock   Clock
	ids     IDs
	timeout time.Duration
	strict  bool
	logger  *slog.Logger

	mu       sync.RWMutex
	handlers map[Type][]Handler
}

// NewService builds an empty bus.
func NewService(d Deps) *Service {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultHookTimeout
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		log:      d.Log,
		clock:    d.Clock,
		ids:      d.IDs,
		timeout:  timeout,
		strict:   d.Strict,
		logger:   logger,
		handlers: map[Type][]Handler{},
	}
}

// Register attaches a handler to the events it declares.
//
// A handler that declares an event it cannot block is registered anyway: the
// bus is what refuses to honour the decision, and it records that it did.
func (s *Service) Register(h Handler) {
	if h == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range h.Handles() {
		s.handlers[t] = append(s.handlers[t], h)
	}
}

// Deregister removes every handler named in ids, from every event type it was
// registered against.
//
// It exists for a skill's own hooks: Register was already safe to call at any
// time, not only at boot (the mutex above predates this method), but nothing
// could undo a registration until now — a skill's Uninstall had a Deregister
// call to make and nothing under it to make it with. Calling it with an id
// nothing registered is a no-op, not an error: a skill's own uninstall path
// depends on that the same way Repository.Delete's idempotence does.
func (s *Service) Deregister(ids ...string) {
	if len(ids) == 0 {
		return
	}
	remove := make(map[string]bool, len(ids))
	for _, id := range ids {
		remove[id] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for t, handlers := range s.handlers {
		kept := handlers[:0]
		for _, h := range handlers {
			if !remove[h.ID()] {
				kept = append(kept, h)
			}
		}
		if len(kept) == 0 {
			delete(s.handlers, t)
		} else {
			s.handlers[t] = kept
		}
	}
}

// Handlers reports the ids registered for one event, in dispatch order. It
// exists so a person can find out what is going to run before it runs.
func (s *Service) Handlers(t Type) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.handlers[t]))
	for _, h := range s.handlers[t] {
		out = append(out, h.ID())
	}
	return out
}

// Emit runs the handlers for an event in registration order.
//
// Three rules, each of which has a test. Context accumulates across handlers.
// The first block ends the chain — later handlers do not get to widen a
// decision an earlier one narrowed. And every invocation is recorded, including
// the ones that returned nothing, because "no handler decided" is an answer
// somebody will need.
func (s *Service) Emit(ctx context.Context, e Event) (Outcome, error) {
	if e.At.IsZero() && s.clock != nil {
		e.At = s.clock.Now()
	}

	s.mu.RLock()
	handlers := append([]Handler(nil), s.handlers[e.Type]...)
	s.mu.RUnlock()

	var acc Outcome
	for _, h := range handlers {
		out, elapsed, err := s.invoke(ctx, h, e)
		if out.HookID == "" {
			out.HookID = h.ID()
		}
		// What the hook said is recorded before the runtime decides what to do
		// with it, so a dropped overreach reads as "the hook asked and was
		// refused" rather than as if it had never asked.
		s.record(ctx, e, h.ID(), out, err, elapsed)

		if err != nil {
			if s.strict {
				return acc, errHookFailed(h.ID(), e.Type, err)
			}
			s.logger.Warn("a hook failed and the turn continued",
				"hook", h.ID(), "event", string(e.Type), "err", err)
			continue
		}

		// A decision the event cannot carry is dropped rather than obeyed, and
		// the drop is logged: honouring it would give SessionStart the power to
		// abort a turn, which is not what the contract says it has.
		if out.Blocked() && !e.Type.CanBlock() {
			s.logger.Warn("a hook tried to block an event that cannot be blocked",
				"hook", h.ID(), "event", string(e.Type))
			out.Decision = ""
			out.PermissionDecision = ""
		}

		acc.Merge(out)
		if out.Blocked() {
			acc.HookID = h.ID()
			return acc, nil
		}
	}
	return acc, nil
}

// invoke runs one handler under its own timeout and its own panic boundary. A
// hook is third-party code on the hottest path in the system; a panic in one
// must cost that hook's opinion and nothing else.
func (s *Service) invoke(ctx context.Context, h Handler, e Event) (Outcome, time.Duration, error) {
	start := s.now()
	hctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var out Outcome
	err := safe.Do(hctx, "hook "+h.ID(), func(c context.Context) error {
		var err error
		out, err = h.Handle(c, e)
		return err
	})
	return out, s.now().Sub(start), err
}

func (s *Service) record(ctx context.Context, e Event, hookID string, out Outcome, cause error, elapsed time.Duration) {
	if s.log == nil {
		return
	}
	r := Record{
		Agent:     e.Agent,
		SessionID: e.SessionID,
		Type:      e.Type,
		Hook:      hookID,
		Payload:   payloadOf(e),
		Outcome:   out,
		Duration:  elapsed.Milliseconds(),
		CreatedAt: s.now(),
	}
	if s.ids != nil {
		r.ID = s.ids.New()
	}
	if cause != nil {
		r.Error = cause.Error()
	}
	// The log is best-effort on the write path for the same reason the loop
	// does not stop for a hook: an audit sink that cannot be written is a
	// problem to report, not a reason to refuse to work.
	if err := s.log.Append(ctx, r); err != nil {
		s.logger.Error("could not record a hook invocation",
			"hook", hookID, "event", string(e.Type), "err", err)
	}
}

// payloadOf keeps in the record what a person needs to understand the decision
// later: which tool, with which input, on which prompt. The event's own
// timestamp is not repeated — the record carries one.
func payloadOf(e Event) json.RawMessage {
	fields := map[string]any{}
	if e.Tool != "" {
		fields["tool"] = e.Tool
	}
	if len(e.Input) > 0 {
		fields["input"] = e.Input
	}
	if len(e.Output) > 0 {
		fields["output"] = e.Output
	}
	if e.Prompt != "" {
		fields["prompt"] = e.Prompt
	}
	if e.Reason != "" {
		fields["error"] = e.Reason
	}
	if e.Trigger != "" {
		fields["trigger"] = e.Trigger
	}
	if len(fields) == 0 {
		return nil
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return raw
}

func (s *Service) now() time.Time {
	if s.clock == nil {
		return time.Time{}
	}
	return s.clock.Now()
}

// FuncHandler adapts a function to the Handler interface, for the handlers the
// runtime registers itself and for tests.
type FuncHandler struct {
	Name   string
	Events []Type
	Fn     func(ctx context.Context, e Event) (Outcome, error)
}

func (f FuncHandler) ID() string      { return f.Name }
func (f FuncHandler) Handles() []Type { return f.Events }
func (f FuncHandler) Handle(ctx context.Context, e Event) (Outcome, error) {
	return f.Fn(ctx, e)
}

// SortedTypes returns the event types a set of handlers covers, sorted. It is
// used by the diagnostics surface and by tests that assert on coverage.
func SortedTypes(handlers []Handler) []Type {
	seen := map[Type]bool{}
	for _, h := range handlers {
		for _, t := range h.Handles() {
			seen[t] = true
		}
	}
	out := make([]Type, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
