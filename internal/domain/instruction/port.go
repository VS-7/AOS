package instruction

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository persists instructions. It is the engine's repository bound to
// the "instructions" native, the same shape collection.Repository and
// toolset.Repository already declare at their own package boundary.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Instruction, error)
	List(ctx context.Context, q collections.Query) ([]Instruction, error)
	Create(ctx context.Context, v *Instruction) error
	Update(ctx context.Context, v *Instruction, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }
