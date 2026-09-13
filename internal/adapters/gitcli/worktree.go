package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/OWNER/aos/internal/domain/task"
)

// Worktrees creates and removes the isolated checkouts a task executes in.
//
// It is a separate type from Git so that a caller holding it cannot also
// init a repository: the task machinery needs exactly three operations, and
// handing it the whole driver would let it do more.
type Worktrees struct {
	git *Git

	// repo is the working tree the checkouts are cut from.
	repo string
}

// NewWorktrees builds the driver for one repository.
//
// The root is resolved through symlinks because git reports resolved paths and
// this type compares against them. On macOS a temporary directory is /var/...
// to the caller and /private/var/... to git, and an unresolved comparison makes
// List report the main working tree as one of ours — which would offer the
// user's own checkout to the prune. The real-git suite catches exactly that.
func NewWorktrees(g *Git, repo string) *Worktrees {
	if g == nil {
		g = New()
	}
	return &Worktrees{git: g, repo: resolve(repo)}
}

// resolve canonicalises a path, falling back to a lexical clean when it cannot
// be walked — a checkout that is not there yet still has a comparable name.
func resolve(path string) string {
	cleaned := filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(cleaned); err == nil {
		return real
	}
	return cleaned
}

// Create cuts a branch and checks it out at its own path.
//
// A branch that already exists is checked out rather than recreated: a task
// that was branched, pruned and branched again should return to its own work,
// not fail because the name is taken.
func (w *Worktrees) Create(ctx context.Context, spec task.WorktreeSpec) (string, error) {
	// Never from an enclosing repository: the checkout would hold somebody
	// else's files, on a branch of somebody else's repository.
	if err := w.git.ownRepository(ctx, "worktree add", w.repo); err != nil {
		return "", err
	}
	path := filepath.Clean(spec.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	// A checkout deleted from disk is still registered, and git refuses both to
	// add one where it has that record and to check out a branch it believes is
	// checked out there — which is exactly what cutting a lost checkout again
	// runs into.
	if err := w.forgetMissing(ctx, path, spec.Branch); err != nil {
		return "", err
	}

	args := []string{"worktree", "add"}
	if w.hasBranch(ctx, spec.Branch) {
		args = append(args, path, spec.Branch)
	} else {
		args = append(args, "-b", spec.Branch, path)
		if base := strings.TrimSpace(spec.Base); base != "" {
			args = append(args, base)
		}
	}

	if _, err := w.git.run(ctx, w.repo, args...); err != nil {
		return "", errGitFailed("worktree add", w.repo, err)
	}
	return path, nil
}

// Remove deletes one checkout.
//
// A checkout that is not there is not an error: a prune that fails because
// somebody already cleaned up by hand is a prune that stops working. The branch
// itself is left alone — the work on it is the point of having made it.
func (w *Worktrees) Remove(ctx context.Context, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Still worth forgetting the administrative record, which git keeps
		// separately and which otherwise reports a worktree that is gone. Only
		// this one's: `git worktree prune` also forgets somebody's own checkout
		// on a drive that is not mounted right now.
		_, _ = w.git.run(ctx, w.repo, "worktree", "remove", "--force", path)
		return nil
	}
	if _, err := w.git.run(ctx, w.repo, "worktree", "remove", "--force", path); err != nil {
		return errGitFailed("worktree remove", path, err)
	}
	return nil
}

// List reports the checkouts that exist, excluding the main working tree.
//
// A checkout git still has a record of but whose directory is gone is not one
// that exists: counting it against the limit made room for nothing, and
// removing it is only forgetting the record, which Create does before it adds.
func (w *Worktrees) List(ctx context.Context) ([]string, error) {
	listed, err := w.listed(ctx)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range listed {
		// The main working tree is not one of ours to prune.
		if entry.path == w.repo || entry.prunable || !isDir(entry.path) {
			continue
		}
		paths = append(paths, entry.path)
	}
	return paths, nil
}

// Exists reports whether path is one of this repository's checkouts, other
// than the main working tree, and is on disk.
func (w *Worktrees) Exists(ctx context.Context, path string) bool {
	want := resolve(path)
	listed, err := w.listed(ctx)
	if err != nil {
		return false
	}
	for _, entry := range listed {
		if entry.path == want {
			return entry.path != w.repo && !entry.prunable && isDir(entry.path)
		}
	}
	return false
}

// forgetMissing drops git's record of a checkout whose directory is gone when
// it stands in the way of this one: it is at the same path, or it holds the
// branch. Nothing on disk is touched and the branch keeps its commits.
func (w *Worktrees) forgetMissing(ctx context.Context, path, branch string) error {
	listed, err := w.listed(ctx)
	if err != nil {
		return err
	}
	want := resolve(path)
	for _, entry := range listed {
		if entry.path == w.repo || (!entry.prunable && isDir(entry.path)) {
			continue
		}
		if entry.path != want && (branch == "" || entry.branch != "refs/heads/"+branch) {
			continue
		}
		if _, err := w.git.run(ctx, w.repo, "worktree", "remove", "--force", entry.path); err != nil {
			return errGitFailed("worktree remove", entry.path, err)
		}
	}
	return nil
}

// worktreeEntry is one block of `git worktree list --porcelain`.
type worktreeEntry struct {
	path   string
	branch string

	// prunable is git's own word for a checkout whose directory is gone.
	prunable bool
}

func (w *Worktrees) listed(ctx context.Context) ([]worktreeEntry, error) {
	out, err := w.git.run(ctx, w.repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, errGitFailed("worktree list", w.repo, err)
	}
	var entries []worktreeEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "worktree "); ok {
			entries = append(entries, worktreeEntry{path: resolve(rest)})
			continue
		}
		if len(entries) == 0 {
			continue
		}
		last := &entries[len(entries)-1]
		if line == "prunable" || strings.HasPrefix(line, "prunable ") {
			last.prunable = true
		}
		if ref, ok := strings.CutPrefix(line, "branch "); ok {
			last.branch = ref
		}
	}
	return entries, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Source reports what a checkout for spec would be cut from.
func (w *Worktrees) Source(ctx context.Context, spec task.WorktreeSpec) (task.WorktreeSource, error) {
	top, own, err := w.git.topOf(ctx, w.repo)
	if err != nil {
		return task.WorktreeSource{}, err
	}
	out := task.WorktreeSource{Dir: w.repo, Toplevel: top, Own: own}
	if !own {
		return out, nil
	}
	if w.hasBranch(ctx, spec.Branch) {
		out.BaseExists = true
		return out, nil
	}
	base := strings.TrimSpace(spec.Base)
	if base == "" {
		base = "HEAD"
	}
	_, err = w.git.run(ctx, w.repo, "rev-parse", "--verify", "--quiet", base+"^{commit}")
	out.BaseExists = err == nil
	return out, nil
}

// hasBranch reports whether a branch name already exists.
func (w *Worktrees) hasBranch(ctx context.Context, branch string) bool {
	_, err := w.git.run(ctx, w.repo, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}
