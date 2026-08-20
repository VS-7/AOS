// Package skillfiles is the filesystem implementation of skill.Files: it
// places a package's raw content — an agent's AGENT.md and its memories and
// routines, a template, an instruction, a goal, a reference, a toolset's own
// declaration — under an installed skill's own directory, and removes it
// again when Apply's rollback or Uninstall asks.
package skillfiles

import (
	"context"
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/pathx"
	"github.com/OWNER/aos/internal/domain/skill"
)

// recordMode matches fscollections' own: a skill's files are meant to be read
// and edited by the user and committed to Git, not secrets.
const recordMode os.FileMode = 0o644

// Files writes and removes a skill's raw content on disk.
type Files struct {
	// root is the workspace root — the same one every fscollections.Repo in
	// this process is bound to. A skill's own directory is root/.aos/skills/{id},
	// collections.Root ("`.aos`") being the one constant every native pattern
	// in internal/core/collections/registry.go already shares, so this adapter
	// agrees with the "skills" native about where an installed skill lives
	// without having to be told twice.
	root string
}

// New builds the adapter over one workspace root.
func New(root string) *Files {
	return &Files{root: filepath.Clean(root)}
}

// skillRoot resolves the directory one skill's raw files live under, through
// any symlink in the path. It does not require the directory to already
// exist — a skill being installed for the first time has none yet, and
// pathx.Root returns such a path unchanged rather than erroring, leaving
// containment to decide once something is actually there (see its own doc).
func (f *Files) skillRoot(skillID string) (string, error) {
	dir := filepath.Join(f.root, collections.Root, "skills", skillID)
	return pathx.Root(dir)
}

// Write places every file of the batch, each confined inside the skill's own
// directory with pathx.ResolveInside — the same boundary skillfetch.Local
// reads a package through, applied here on the way back out. It stops at the
// first failure: Apply's rollback (internal/domain/skill/install.go) only
// records a file as written once Write has already reported success for the
// whole batch, so a Write that fails partway through and leaves some of it on
// disk is a contract this type must not violate — every file below the one
// that failed is left unwritten, not attempted.
func (f *Files) Write(_ context.Context, skillID string, files []skill.RawFile) error {
	root, err := f.skillRoot(skillID)
	if err != nil {
		return errResolveFailed(skillID, err)
	}
	for _, file := range files {
		abs, err := pathx.ResolveInside(root, file.Path)
		if err != nil {
			return errPathOutside(skillID, file.Path, err)
		}
		if err := atomicfs.WriteFile(abs, file.Content, recordMode); err != nil {
			return errWriteFailed(skillID, file.Path, err)
		}
	}
	return nil
}

// Remove deletes one file Write placed, addressed the same way: skillID plus
// the file's own skill-relative path. It must not fail when asked to remove
// something already gone — Apply's rollback calls this once per file Write
// reported, in reverse order, after a failure, and a rollback that errors on
// its own undo would bury the original failure under a second one that
// explains nothing.
func (f *Files) Remove(_ context.Context, skillID, path string) error {
	root, err := f.skillRoot(skillID)
	if err != nil {
		return errResolveFailed(skillID, err)
	}
	abs, err := pathx.ResolveInside(root, path)
	if err != nil {
		return errPathOutside(skillID, path, err)
	}
	if err := os.Remove(abs); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errRemoveFailed(skillID, path, err)
	}
	return nil
}
