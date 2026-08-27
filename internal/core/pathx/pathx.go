// Package pathx resolves a user-supplied path against a root the way a
// filesystem boundary must, so that every caller confining access to a
// directory tree — the agent sandbox, the file explorer — shares one
// implementation instead of two chances to get it wrong.
package pathx

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

	// Walk up to the nearest ancestor that does exist and resolve *that*,
	// then re-append what was missing. Resolving only the immediate parent
	// was not enough: for "root/link/new/file", where link is a symlink out
	// of the root and "new" does not exist yet, the parent does not resolve
	// either — and returning `abs` unresolved handed Contains a path that
	// looks inside the root, while a MkdirAll on it would follow the symlink
	// and write outside. Two missing components were all it took.
	missing := []string{filepath.Base(abs)}
	dir := filepath.Dir(abs)
	for {
		real, err := filepath.EvalSymlinks(dir)
		if err == nil {
			slices.Reverse(missing)
			return filepath.Join(append([]string{real}, missing...)...), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root of the volume without finding anything that
			// exists. Nothing to resolve through, so the cleaned path is as
			// good as it gets; containment still decides.
			return abs, nil
		}
		missing = append(missing, filepath.Base(dir))
		dir = parent
	}
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

// ErrUnsafeSegment is returned by Segment for an identifier that cannot be
// used as one path component.
var ErrUnsafeSegment = errors.New("pathx: identifier is not a single safe path segment")

// Segment validates an identifier that a caller is about to join into a file
// path, and returns it unchanged when it is safe.
//
// This exists because "join the id onto the root" is written in a dozen
// adapters, and `filepath.Join(root, "..")` is the parent of the root — with
// no symlink involved and nothing for Resolve or Contains to catch, because
// the result is a perfectly ordinary path that simply is not the one the
// caller meant. Two of those joins were, before this: `workspace delete ..`
// removed the whole installation state directory, and `artifacts delete ..`
// removed a workspace's entire .aos.
//
// The rule is deliberately strict — one component, no separators, no
// traversal, no volume name, no NUL, not empty and not hidden-relative. Ids
// in this system are slugs and UUIDs; anything else is a caller mistake or an
// attack, and both are better refused here than resolved somewhere surprising.
func Segment(id string) (string, error) {
	if id == "" || id == "." || id == ".." {
		return "", ErrUnsafeSegment
	}
	if strings.ContainsAny(id, "/\\") || strings.ContainsRune(id, 0) {
		return "", ErrUnsafeSegment
	}
	// filepath.Clean is the second opinion: anything it rewrites (a trailing
	// dot component, a Windows volume like "C:") was not a plain segment.
	if filepath.Clean(id) != id || filepath.VolumeName(id) != "" {
		return "", ErrUnsafeSegment
	}
	return id, nil
}

// JoinSegment is Segment plus the join every caller does next, so the guard
// cannot be skipped by accident at a new call site.
func JoinSegment(root string, segments ...string) (string, error) {
	out := root
	for _, s := range segments {
		safe, err := Segment(s)
		if err != nil {
			return "", err
		}
		out = filepath.Join(out, safe)
	}
	return out, nil
}
