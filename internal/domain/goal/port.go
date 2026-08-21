package goal

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository persists a Goal, bound to the "goals" native the same way every
// other domain's repository is (see internal/domain/toolset.Repository or
// internal/domain/view.Repository for the identical shape).
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Goal, error)
	List(ctx context.Context, q collections.Query) ([]Goal, error)
	Create(ctx context.Context, g *Goal) error
	Update(ctx context.Context, g *Goal, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Tasks is the narrow slice of task.Service Delete needs: clearing the Goal
// field off every task that referenced this goal, without removing the tasks
// themselves — the same rule Project (Go) applies. Kept a separate port
// rather than an import of internal/domain/task directly, for the same
// reason view/port.go's Collections is: a change to task.Service's own
// signature must not silently break this package.
type Tasks interface {
	ClearGoal(ctx context.Context, goalID string) error
}

// Clock is the only source of time this package needs.
type Clock interface{ Now() time.Time }
