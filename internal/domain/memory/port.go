package memory

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/search"
)

// Repository is what this package needs to persist a memory.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Memory, error)
	List(ctx context.Context, q collections.Query) ([]Memory, error)
	Create(ctx context.Context, v *Memory) error
	Update(ctx context.Context, v *Memory, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Index is the optional full-text accelerator.
//
// Optional is the word that matters. When it is nil, Recall scans and filters
// in memory, which is exactly what the original does and is correct up to a few
// thousand records. When it is present, it narrows the candidate set first. The
// answers must agree either way — that is what the shared tokeniser in
// core/search is for — so the index changes how long a recall takes and never
// what it returns.
type Index interface {
	Upsert(ctx context.Context, d search.Document) error
	Delete(ctx context.Context, collection, id string) error
	Search(ctx context.Context, q search.Query) ([]search.Hit, error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new memory.
type IDs interface{ New() string }
