package collection

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository persists collection declarations. It is the engine's repository
// bound to the "collections" native.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Collection, error)
	List(ctx context.Context, q collections.Query) ([]Collection, error)
	Create(ctx context.Context, v *Collection) error
	Update(ctx context.Context, v *Collection, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }
