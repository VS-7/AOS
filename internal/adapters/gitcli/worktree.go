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
	path := filepath.Clean(spec.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
		// Still worth pruning the administrative record, which git keeps
		// separately and which otherwise reports a worktree that is gone.
		_, _ = w.git.run(ctx, w.repo, "worktree", "prune")
		return nil
	}
	if _, err := w.git.run(ctx, w.repo, "worktree", "remove", "--force", path); err != nil {
		return errGitFailed("worktree remove", path, err)
	}
	return nil
}

// List reports the checkouts that exist, excluding the main working tree.
func (w *Worktrees) List(ctx context.Context) ([]string, error) {
	out, err := w.git.run(ctx, w.repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, errGitFailed("worktree list", w.repo, err)
	}

	var paths []string
	for _, line := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "worktree ")
		if !ok {
			continue
		}
		path := resolve(rest)
		if path == w.repo {
			continue // the main working tree is not one of ours to prune
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// hasBranch reports whether a branch name already exists.
func (w *Worktrees) hasBranch(ctx context.Context, branch string) bool {
	_, err := w.git.run(ctx, w.repo, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}
