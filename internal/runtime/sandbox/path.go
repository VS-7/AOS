package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// resolve turns whatever the agent asked for into an absolute path on disk.
//
// Normalising happens before containment is checked, and the order is the
// security. Collapsing ".." first turns a traversal attempt into a path that
// simply falls outside the root, which the next step rejects; checking first
// and normalising after would let "root/../../etc/passwd" pass a prefix test.
//
// Symbolic links are resolved too. That part is an addition: the original
// normalises but the reverse engineering records no link resolution, and a link
// inside the root pointing outside it is an escape nobody sees.
func (s *Sandbox) resolve(p string) (string, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.root, p)
	}
	abs = filepath.Clean(abs)

	real, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return real, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	// A file that is about to be created has no link to resolve, so the parent
	// is validated instead. Without this, every write to a new path would be
	// rejected — and resolving the parent is what stops a write through a
	// symlinked directory.
	parent, perr := filepath.EvalSymlinks(filepath.Dir(abs))
	if perr != nil {
		return abs, nil //nolint:nilerr // the parent does not exist either; containment still decides
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

// checkFS is the one gate every filesystem operation passes.
func (s *Sandbox) checkFS(p string, op Op) (string, error) {
	real, err := s.resolve(p)
	if err != nil {
		return "", errPathUnreadable(p, err)
	}

	inRoot := within(real, s.root)
	inTmp := s.tmpDir != "" && within(real, s.tmpDir)

	if !inRoot && !inTmp {
		return "", errPathOutside(p, real, s.root)
	}
	if inTmp && op != OpRead {
		return "", errTmpReadOnly(real, op)
	}
	if isGitPath(real, s.root) && op != OpRead {
		return "", errGitReadOnly(real, op)
	}
	if !s.perms.allows(op) {
		return "", errPermissionDenied(op, real, s.perms)
	}
	return real, nil
}

// within reports whether p is inside root.
//
// It compares the relative path rather than a string prefix, which is the
// difference between "/a/bc is not inside /a/b" and a bug people ship.
func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// isGitPath reports whether a path is inside the repository's own metadata.
//
// Read is allowed, because the master prompt tells the agent to reconstruct
// history from `git log` and reading the directory is sometimes how a tool does
// that. Writing is not: a legitimate Git operation goes through the git
// command, which knows how to keep the repository consistent.
func isGitPath(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}
