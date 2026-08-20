package view

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
)

// Repository persists view declarations. It is the engine's repository bound
// to the "views" native (registered in internal/core/collections/registry.go
// since Task 4), the same shape internal/domain/collection.Repository takes
// for its own aggregate: a domain test fakes it directly, with no generic in
// sight.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*View, error)
	List(ctx context.Context, q collections.Query) ([]View, error)
	Create(ctx context.Context, v *View) error
	Update(ctx context.Context, v *View, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Collections is what the view domain needs to know about the data it binds
// to: the declaration, to validate a bind and a scaffold against, and the
// records, to render. It is a port because internal/domain does not reach
// across to another domain's service directly — even one that, unlike a
// repository, already lives in internal/domain: the collection package is
// still a separate feature with its own lifecycle, and a direct import would
// let a change to its service signature break this one without either
// package's tests noticing until the wiring in internal/app failed to build.
type Collections interface {
	// Get resolves a collection by id. skill, when not empty, is the view's
	// own — a skill-scoped view's source is commonly that same skill's own
	// collection, and a collection scoped to a skill is unreachable by id
	// alone (collection.GetInput's own doc explains why: it lives under a
	// second, skill-qualified pattern). skill is a hint, not a requirement:
	// an implementation tries it first and falls back to the workspace-
	// scoped id, so a skill's view sourcing a workspace collection still
	// resolves.
	Get(ctx context.Context, id, skill string) (*collection.Collection, error)
	ListRecords(ctx context.Context, id, skill string, q collection.RecordQuery) ([]collection.Record, error)
}

// Commands is the slice of the command registry an Action needs: whether a
// command exists, and how to run one.
type Commands interface {
	Has(name string) bool
	Invoke(ctx context.Context, name string, in json.RawMessage) (json.RawMessage, error)
}

// Clock is the only source of time this package reads. Injected rather than
// time.Now so a view's CreatedAt/UpdatedAt in a golden file is reproducible.
type Clock interface{ Now() time.Time }
