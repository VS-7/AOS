package todo

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist a step.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Todo, error)
	List(ctx context.Context, q collections.Query) ([]Todo, error)
	Create(ctx context.Context, v *Todo) error
	Update(ctx context.Context, v *Todo, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Parent answers the one question this package asks of the task aggregate: does
// the task exist.
//
// It is a port rather than an import of the task service because a subcollection
// that could reach the whole parent could also move it, and a todo has no
// business changing a task's status.
type Parent interface {
	Exists(ctx context.Context, taskID string) (bool, error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new step.
type IDs interface{ New() string }
