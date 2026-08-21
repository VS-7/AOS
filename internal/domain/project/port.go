package project

import (
	"context"
	"os"
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

// PathStat is the one filesystem read validateSource needs: whether Source
// exists and is a directory. Kept behind a port rather than an inline
// os.Stat — os.Stat itself is not on the architecture's forbidden-import
// list, but every other domain that reads the host filesystem does it
// through an injected dependency (see file.Service's FS, workspace.Service's
// FS), and a real filesystem check inline is also a check no domain test can
// exercise on all four outcomes without touching real disk. osStat below is
// the default; a test swaps in a fake.
type PathStat interface {
	Stat(path string) (os.FileInfo, error)
}

// osStat is the production PathStat: the real filesystem.
type osStat struct{}

func (osStat) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }
