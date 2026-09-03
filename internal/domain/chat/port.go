package chat

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist a conversation.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Chat, error)
	List(ctx context.Context, q collections.Query) ([]Chat, error)
	Create(ctx context.Context, v *Chat) error
	Update(ctx context.Context, v *Chat, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Directory answers the two questions routing asks: is this identifier an
// agent, and who answers when nobody was named.
//
// It is a port rather than a dependency on the agent package because those two
// questions are all this aggregate needs, and taking the whole agent service
// would let a conversation create one.
type Directory interface {
	IsAgent(ctx context.Context, id string) bool
	Orchestrator(ctx context.Context) (string, error)
}

// Dispatcher hands a turn to the agent runtime.
//
// Send persists first and dispatches second, so a message survives a worker
// that dies between the two. Until the runtime exists, the composition root
// supplies an implementation that records the intent and returns — which is
// exactly what a queue does from this side of the boundary.
type Dispatcher interface {
	Dispatch(ctx context.Context, in Turn) (jobID string, err error)
}

// Turn is one unit of work handed to the runtime.
type Turn struct {
	ChatID    string
	MessageID string
	AgentID   string
	Task      string
	Routine   string

	// JobID names this attempt, so the run the runtime records at the start
	// and the one it completes at the end are the same run. Empty for a
	// caller that runs the turn itself rather than queueing it.
	JobID string
}

// Canceller ends a turn that is already running.
//
// It is the other half of Dispatcher, and it was missing: a turn was handed
// to the runtime and there was no way to ask for it back. The composer's Stop
// button called a command that did not exist, answered "no active run was
// found to stop", and the agent kept working.
//
// Nil is a legitimate wiring — a daemon with no runtime cannot stop what it
// never started — and Stop says so rather than pretending.
type Canceller interface {
	// Stop reports whether there was a turn to stop. A conversation that has
	// already finished is not an error: it is somebody pressing the button a
	// moment late.
	Stop(ctx context.Context, chatID string) (stopped bool, err error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new conversation or message.
type IDs interface{ New() string }
