package file

import (
	"context"
	"time"
)

// Info is what the filesystem reports about one entry, before it is turned
// into a Node.
type Info struct {
	Name    string
	Dir     bool
	Size    int64
	ModTime time.Time
}

// FS is the filesystem slice this domain touches: path containment, reads,
// writes, and directory listings. It exists so the domain can be tested
// against a fake — see workspace.Scaffolder for the same reasoning, and File
// (Go)'s own note that internal/domain may not import os/exec directly.
//
// wide port: Resolve plus the six plain filesystem operations is more than
// six methods, but Tree, Read, Write, Move, Delete, and Diff each need a
// different subset of them — splitting this by consumer would just move the
// reassembly into Service instead of removing it.
type FS interface {
	// Resolve confines p to root the way the sandbox confines a tool call —
	// see pathx.ResolveInside, which the real adapter delegates to — and
	// returns the path every other method here is called with. An error
	// satisfying errors.Is(err, ErrOutside) is what Service turns into
	// FILE_OUTSIDE_WORKSPACE; anything else is a lower-level resolution
	// failure.
	Resolve(ctx context.Context, root, p string) (string, error)

	Stat(ctx context.Context, path string) (Info, error)
	ReadDir(ctx context.Context, path string) ([]Info, error)

	// ReadFile reads up to limit bytes of path. truncated reports whether the
	// file held more than that — the caller decides what a truncated read
	// means to the person looking at it.
	ReadFile(ctx context.Context, path string, limit int64) (data []byte, truncated bool, err error)

	WriteFile(ctx context.Context, path string, data []byte) error
	MkdirAll(ctx context.Context, path string) error
	Rename(ctx context.Context, from, to string) error
	Remove(ctx context.Context, path string) error
}

// Git is the version-control read surface Diff needs.
type Git interface {
	// Status reports path's working-tree status relative to HEAD: "added",
	// "modified", "deleted", "untracked", or "" when there is no change.
	Status(ctx context.Context, root, path string) (string, error)

	// Changes lists every path the working tree differs from HEAD at, in one
	// call. Status answers the same question for one path, and is what a diff
	// the user has already opened uses; this is what the list of them needs,
	// and building it out of Status would mean walking the whole repository
	// and asking about every file in it.
	//
	// A directory that is not a repository is not an error here: a workspace
	// does not have to be one, and the panel still has to open.
	Changes(ctx context.Context, root string) ([]Change, error)

	// Show returns path's content at ref. ok is false when the path does not
	// exist at that ref — a new file has no HEAD version, a deleted file has
	// no working-tree version, and neither is an error.
	Show(ctx context.Context, root, ref, path string) (data []byte, ok bool, err error)
}

// Workspaces resolves the repository root this domain's operations confine
// themselves to.
type Workspaces interface {
	Root(ctx context.Context) (string, error)
}
