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

// IDs hands out the identifier of a new record.
type IDs interface{ New() string }

// RecordRepo persists the records of one collection. Its five methods mirror
// collections.Repository[collections.Record] exactly, so a
// fscollections.Repo[collections.Record] satisfies it with no adapter shim —
// it is declared again here, narrower and in the domain, for the same reason
// Repository above is: a port a domain test can fake without reaching for a
// generic.
type RecordRepo interface {
	Get(ctx context.Context, key collections.Key) (*collections.Record, error)
	List(ctx context.Context, q collections.Query) ([]collections.Record, error)
	Create(ctx context.Context, v *collections.Record) error
	Update(ctx context.Context, v *collections.Record, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// RecordRepositories is the port that hands back the repository bound to one
// collection's records.
//
// The domain cannot construct a disk-backed repository itself —
// internal/architecture's TestDependencyRule forbids internal/domain from
// importing internal/adapters — so this is how RecordService reaches one:
// given the collection's declaration, which carries its format and, for a
// skill-scoped collection, its second read-only pattern, the adapter builds
// the Model[collections.Record] and the Repo that serves it.
type RecordRepositories interface {
	For(c Collection) (RecordRepo, error)
}
