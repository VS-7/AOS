package collections

import "context"

// Repository is the port every persistence adapter implements. The contract it
// must satisfy is executable: internal/domain/testsuite.RunRepositoryContract.
//
// Wide port: five methods is the whole CRUD surface of one aggregate, and
// splitting it would force every caller to accept five interfaces to do one
// job. The read-only consumers take a narrower interface of their own.
type Repository[T any] interface {
	Get(ctx context.Context, key Key) (*T, error)
	List(ctx context.Context, q Query) ([]T, error)
	Create(ctx context.Context, v *T) error
	Update(ctx context.Context, v *T, expect Version) error
	Delete(ctx context.Context, key Key) error
}

// Versioned is implemented by repositories that can report the concurrency
// token of a record, so a caller can read, think, and then write with the
// expectation it captured.
type Versioned interface {
	Version(ctx context.Context, key Key) (Version, error)
}

// Refresher scans the disk and rebuilds the in-memory index. It runs at boot
// and after an external change the watcher could not attribute.
type Refresher interface {
	Refresh(ctx context.Context) (int, error)
}
