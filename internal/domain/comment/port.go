package comment

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist a comment.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Comment, error)
	List(ctx context.Context, q collections.Query) ([]Comment, error)
	Create(ctx context.Context, v *Comment) error
	Update(ctx context.Context, v *Comment, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Parent answers whether the task exists.
type Parent interface {
	Exists(ctx context.Context, taskID string) (bool, error)
}

// Moderator reports whether the current actor may edit somebody else's comment.
//
// The original has a "super" user who can. It is a port rather than a check on
// a role string because who counts as a moderator is an installation's policy,
// and a domain that hardcoded one would be deciding it for everybody.
type Moderator interface {
	MayModerate(ctx context.Context) bool
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new comment.
type IDs interface{ New() string }
