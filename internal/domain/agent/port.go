package agent

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist an agent. The interface
// lives here, where it is consumed; the filesystem implementation satisfies it
// structurally, without importing this package.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Agent, error)
	List(ctx context.Context, q collections.Query) ([]Agent, error)
	Create(ctx context.Context, v *Agent) error
	Update(ctx context.Context, v *Agent, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Clock is the only source of time in this package. Injected, so that a golden
// file built from an agent record does not change every second.
type Clock interface{ Now() time.Time }
