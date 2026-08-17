// Package pathx resolves a user-supplied path against a root the way a
// filesystem boundary must, so that every caller confining access to a
// directory tree — the agent sandbox, the file explorer — shares one
// implementation instead of two chances to get it wrong.
package pathx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutside is returned by ResolveInside when the resolved path falls
// outside root.
var ErrOutside = errors.New("pathx: path resolves outside the root")

// Resolve turns p into an absolute path anchored on root, normalizing and
// resolving symbolic links along the way. It does not check containment —
// callers with more than one valid root (a workspace plus a scratch
// directory, say) check that themselves with Contains; ResolveInside is the
// single-root convenience that does both.
//
// root itself is taken as already resolved (a caller normally does this once,
// at construction, via filepath.EvalSymlinks) — Resolve does not re-resolve
// it, so a raw root that is itself a symlink (macOS's /var → /private/var,
// notably) will make every path underneath it look like it escaped.
//
// Normalising happens before symlinks are resolved, and the order matters:
// collapsing ".." first turns a traversal attempt into a path that simply
// falls outside the root, which containment then rejects; resolving links
// first and normalising after would let "root/../../etc/passwd" slip by a
// prefix test.
//
// A path that does not exist yet (a file about to be created) has no link of
// its own to resolve, so its parent directory is resolved instead — without
// this, every write to a new path would fail, and resolving the parent is
// what stops a write through a symlinked directory.
func Resolve(root, p string) (string, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, p)
	}
	abs = filepath.Clean(abs)

	real, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return real, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	parent, perr := filepath.EvalSymlinks(filepath.Dir(abs))
	if perr != nil {
		return abs, nil //nolint:nilerr // the parent does not exist either; containment still decides
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

// Contains reports whether p is inside root.
//
// It compares the relative path rather than a string prefix, which is the
// difference between "/a/bc is not inside /a/b" and a bug people ship.
func Contains(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Root resolves root itself through symlinks once, so every later Resolve or
// Contains call against it compares apples to apples — Contains otherwise
// rejects everything underneath a root that is itself a symlink (macOS's
// /var → /private/var, notably: t.TempDir() lives there). Callers that
// already resolved their root once, at construction, don't need this.
//
// A root that does not exist yet is returned unchanged rather than as an
// error — the same "containment still decides" reasoning Resolve applies to
// a path about to be created. A workspace whose directory was removed out
// from under the daemon fails clearly at the first real operation against
// it, not opaquely here.
func Root(root string) (string, error) {
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return root, nil //nolint:nilerr // see doc comment: containment still decides
		}
		return "", err
	}
	return real, nil
}

// ResolveInside resolves p against root and requires the result to be
// contained in root, returning ErrOutside otherwise. It is Resolve and
// Contains combined, for the common case of a single valid root.
func ResolveInside(root, p string) (string, error) {
	real, err := Resolve(root, p)
	if err != nil {
		return "", err
	}
	if !Contains(root, real) {
		return "", ErrOutside
	}
	return real, nil
}
