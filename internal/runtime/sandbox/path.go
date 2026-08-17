package sandbox

import (
	"path/filepath"
	"strings"

	"github.com/OWNER/aos/internal/core/pathx"
)

// checkFS is the one gate every filesystem operation passes.
func (s *Sandbox) checkFS(p string, op Op) (string, error) {
	real, err := pathx.Resolve(s.root, p)
	if err != nil {
		return "", errPathUnreadable(p, err)
	}

	inRoot := pathx.Contains(s.root, real)
	inTmp := s.tmpDir != "" && pathx.Contains(s.tmpDir, real)

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
