package workspace

import (
	"context"
	"time"
)

// Store persists the workspace records that live outside the user's repository,
// under the installation state directory.
//
// It is not a collections.Repository: a workspace is the thing that *has* a
// repository root, so it cannot be stored relative to one.
type Store interface {
	Get(ctx context.Context, id string) (*Workspace, error)
	List(ctx context.Context) ([]Workspace, error)
	Save(ctx context.Context, w *Workspace) error
	Delete(ctx context.Context, id string) error
}

// Scaffolder is the slice of the filesystem this aggregate needs to lay out a
// workspace inside the user's repository.
//
// The domain could call os.MkdirAll directly — nothing in the dependency rule
// forbids it — but then the tests of "is the scaffold idempotent" and "does the
// splice preserve what the user wrote" would need a temporary directory to say
// anything, and a rule about the user's files would be proven against a tmpdir
// rather than against the rule.
type Scaffolder interface {
	// EnsureDir creates a directory if it is missing and reports whether it
	// had to. The bool is what makes the scaffold idempotent *and* observable:
	// a second run reports that it created nothing.
	EnsureDir(ctx context.Context, path string) (created bool, err error)

	// ReadFile returns the contents of a file, or "" when it does not exist.
	// A missing .env is the normal case, not an error.
	ReadFile(ctx context.Context, path string) (string, error)

	// WriteFile replaces the contents of a file, creating it if needed.
	WriteFile(ctx context.Context, path, content string) error
}

// Git is the version-control surface a workspace touches at creation. It is a
// port because it shells out, and internal/domain may not import os/exec.
type Git interface {
	// IsRepository reports whether dir is already under version control.
	IsRepository(ctx context.Context, dir string) (bool, error)

	// Init creates a repository at dir.
	Init(ctx context.Context, dir string) error

	// OriginURL returns the URL of the "origin" remote, or "" when there is
	// none. It is how `workspace introspect` names a workspace after its repo.
	OriginURL(ctx context.Context, dir string) (string, error)
}

// Seeder creates the records a workspace is born with.
//
// The workspace aggregate decides *when* an orchestrator must exist; it does
// not know how an agent is written to disk. Keeping that behind a port is what
// stops this package from importing the agent package and inheriting its
// storage decisions.
type Seeder interface {
	// FindOrchestrator returns the id of the workspace's orchestrator, if one
	// is already there. Creating a workspace over an existing .aos/ directory
	// must adopt the agent that is present rather than add a second one.
	FindOrchestrator(ctx context.Context, root string) (string, bool, error)

	// SeedOrchestrator writes the orchestrator and returns its id.
	SeedOrchestrator(ctx context.Context, in OrchestratorSeed) (string, error)
}

// OrchestratorSeed is everything the seeder needs to write the first agent.
type OrchestratorSeed struct {
	Root         string // the repository root, where .aos/ lives
	ID           string
	Name         string
	Role         string
	Description  string
	Instructions string
}

// Surveyor counts what a workspace holds, by collection. It answers the
// inventory question without loading a single record body.
type Surveyor interface {
	Survey(ctx context.Context, root string) ([]CollectionSummary, error)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }
