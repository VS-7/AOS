package project

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist a Project.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Project, error)
	List(ctx context.Context, q collections.Query) ([]Project, error)
	Create(ctx context.Context, v *Project) error
	Update(ctx context.Context, v *Project, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Unlinker is a domain that references a project by id and must drop that
// reference, not the record it belongs to, when the project is deleted — see
// Service.Delete. Task satisfies this today; Goal is meant to once it exists.
type Unlinker interface {
	// UnlinkProject clears every reference to projectID this domain holds,
	// leaving the referencing records themselves untouched.
	UnlinkProject(ctx context.Context, projectID string) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }
