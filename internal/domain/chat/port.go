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
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new conversation or message.
type IDs interface{ New() string }
