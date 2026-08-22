package template

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository persists Template declarations. Unlike toolset's, it includes
// Create: an agent or a person authors a template directly, there is no
// install step that reaches disk on its own the way a toolset's
// configuration does.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Template, error)
	List(ctx context.Context, q collections.Query) ([]Template, error)
	Create(ctx context.Context, t *Template) error
	Update(ctx context.Context, t *Template, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Clock is the only source of time this package reads.
type Clock interface{ Now() time.Time }

// Engine is the Liquid renderer Render runs a Template's Content through —
// the "Renderer of two functions" ADR-0014 asks for, so
// github.com/osteele/liquid stays contained behind this package's own
// boundary and is substitutable if it ever proves insufficient.
type Engine interface {
	// Validate parses source without rendering it, so Create and Update can
	// refuse a syntactically broken template at write time instead of at the
	// first render — the same "refuse at write time" rule View already
	// applies to its own tree.
	Validate(source string) error

	// Render parses and renders source against vars, under whatever timeout
	// and output cap the caller enforces around the call — this method
	// itself is not bounded, callers are (see render.go's Render).
	Render(source string, vars map[string]any) (string, error)
}

// Workspaces resolves the directory a render's Output path is relative to —
// the same port file.Service takes in wire.go (see workspaceRoot there).
type Workspaces interface {
	Root(ctx context.Context) (string, error)
}

// Files is the filesystem slice a disk-writing Render needs: confine a path
// to the workspace root, and write to it. It is narrowed to exactly
// file.FS's own Resolve/WriteFile/MkdirAll, so the real osfile.FS adapter
// already wired for the file domain satisfies it with no wrapper — see
// wire.go. It stays its own interface rather than an import of file.FS, the
// same reason every port in this codebase is declared locally: this package
// must not import another domain's.
type Files interface {
	// Resolve confines p to root, the way pathx.ResolveInside — which the
	// real adapter delegates to — confines a file explorer's own reads.
	Resolve(ctx context.Context, root, p string) (string, error)

	WriteFile(ctx context.Context, path string, data []byte) error
	MkdirAll(ctx context.Context, path string) error
}
