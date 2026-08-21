package artifact

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository persists artifacts. It is the engine's repository bound to the
// "artifacts" native.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Artifact, error)
	List(ctx context.Context, q collections.Query) ([]Artifact, error)
	Create(ctx context.Context, v *Artifact) error
	Update(ctx context.Context, v *Artifact, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Files is what this package needs of the artifact's own directory on disk —
// scaffolding an entrypoint on Create, and serving is the transport's job,
// not this package's, so Files stays narrow.
type Files interface {
	// Ensure makes the artifact's directory and, when entrypoint does not
	// already exist there, writes a minimal HTML file at that relative path.
	// It returns the entrypoint actually in place.
	Ensure(ctx context.Context, id, entrypoint string) (string, error)

	// Remove deletes the artifact's directory and everything under it.
	Remove(ctx context.Context, id string) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new record.
type IDs interface{ New() string }

// PasswordHasher is how SetPassword turns a plaintext password into what
// Authorize compares against — argon2id, kept behind an interface so a test
// can swap in something cheaper than the real KDF.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) (bool, error)
}
