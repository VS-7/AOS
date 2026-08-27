package task

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
)

// DefaultBranchPrefix is used when the workspace declares none.
const DefaultBranchPrefix = "aos"

// Branch creates the isolated checkout a task executes in.
//
// The sandbox root becomes this path, which is the point: an agent working on a
// task is confined to its own branch and cannot touch the main working tree.
// The workspace's onCreateScript runs inside it afterwards — under the assigned
// agent's sandbox policy, not with free rein, because a setup script is
// third-party code in most workspaces.
func (s *Service) Branch(ctx context.Context, in BranchInput) (*Worktree, error) {
	current, err := s.load(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if s.worktrees == nil {
		return nil, errWorktreesUnavailable(current.ID)
	}
	if current.Worktree.Path != "" {
		return &current.Worktree, nil
	}

	policy, err := s.worktreePolicy(ctx)
	if err != nil {
		return nil, err
	}
	if policy.Limit > 0 {
		if err := s.pruneToLimit(ctx, policy); err != nil {
			return nil, err
		}
	}

	branch := strings.TrimSpace(in.Branch)
	if branch == "" {
		branch = BranchNameFor(policy.BranchPrefix, current)
	}
	base := strings.TrimSpace(in.Base)
	if base == "" {
		base = current.Worktree.Base
	}
	if base == "" {
		base = policy.DefaultBase
	}

	path := filepath.Join(policy.Root, current.ID)
	created, err := s.worktrees.Create(ctx, WorktreeSpec{
		TaskID: current.ID, Branch: branch, Base: base, Path: path,
	})
	if err != nil {
		return nil, errWorktreeFailed(current.ID, branch, err)
	}

	current.Worktree = Worktree{Enabled: true, Base: base, Branch: branch, Path: created}
	current.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		// The checkout exists and the task does not know about it. Removing it
		// is the only way to leave the two consistent, and a failure to remove
		// is reported rather than swallowed.
		if rmErr := s.worktrees.Remove(ctx, created); rmErr != nil {
			s.log.Error("a worktree was created for a task that could not record it",
				"task", current.ID, "path", created, "err", rmErr)
		}
		return nil, errWriteFailed("Branch", err)
	}

	if script := strings.TrimSpace(policy.OnCreateScript); script != "" && s.setup != nil {
		if err := s.setup.Run(ctx, current.Assigned, created, script); err != nil {
			// The checkout is usable; the setup did not run. That is worth
			// saying out loud rather than failing: the agent can install what
			// it needs, and destroying the branch over a failed script would
			// lose whatever the script did manage to do.
			s.log.Warn("the workspace setup script failed in a new worktree",
				"task", current.ID, "path", created, "err", err)
			return &current.Worktree, errSetupFailed(current.ID, created, err)
		}
	}
	s.notify(ctx, "branched", current, map[string]any{"branch": branch, "path": created})
	return &current.Worktree, nil
}

// BranchNameFor builds the branch of a task from the workspace prefix and the
// task's slug. It is exported because the desktop shows the name before the
// branch exists, and two places deriving it separately is how they drift.
func BranchNameFor(prefix string, t *Task) string {
	if prefix == "" {
		prefix = DefaultBranchPrefix
	}
	name := t.Slug
	if name == "" {
		name = t.ID
	}
	return prefix + "/" + name
}

// pruneToLimit removes the oldest finished tasks' checkouts until there is room
// for one more.
//
// Oldest finished first is deliberate: a checkout belonging to work still in
// progress is never taken, even when that means the limit is not reached and
// the create below fails with a clear reason instead.
func (s *Service) pruneToLimit(ctx context.Context, policy WorktreePolicy) error {
	if !policy.DeleteOld {
		return nil
	}
	listed, err := s.worktrees.List(ctx)
	if err != nil {
		return errWorktreeFailed("", "", err)
	}

	// Only this workspace's own checkouts, when it says where it puts them.
	// `git worktree list` reports every worktree of the repository, the
	// person's own branches included — and counting those against a limit
	// that exists to bound *this system's* checkouts made an unrelated
	// worktree consume the budget, then made the pruner reach for it. Both
	// halves of that are wrong, and this is the line that fixes both.
	root := strings.TrimSpace(policy.Root)
	existing := listed
	if root != "" {
		existing = existing[:0:0]
		for _, path := range listed {
			if underRoot(root, path) {
				existing = append(existing, path)
			}
		}
	}
	if len(existing) < policy.Limit {
		return nil
	}

	all, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return errReadFailed("pruneToLimit", err)
	}
	held := map[string]*Task{}
	for i := range all {
		if all[i].Worktree.Path != "" {
			held[all[i].Worktree.Path] = &all[i]
		}
	}

	type candidate struct {
		path string
		task *Task
	}
	var removable []candidate
	for _, path := range existing {
		owner, ok := held[path]
		if !ok {
			// A checkout of ours that no task claims: a leftover from a run
			// that did not finish. Nothing is executing in it and nothing
			// points at it. (`existing` is already confined to the
			// workspace's own root above, so this can no longer be
			// somebody's own worktree.)
			removable = append(removable, candidate{path: path})
			continue
		}
		if owner.Status.Terminal() {
			removable = append(removable, candidate{path: path, task: owner})
		}
	}
	sort.SliceStable(removable, func(i, j int) bool {
		if removable[i].task == nil {
			return true
		}
		if removable[j].task == nil {
			return false
		}
		return removable[i].task.UpdatedAt.Before(removable[j].task.UpdatedAt)
	})

	need := len(existing) - policy.Limit + 1
	for _, c := range removable {
		if need <= 0 {
			break
		}
		if err := s.worktrees.Remove(ctx, c.path); err != nil {
			s.log.Warn("an old worktree could not be pruned", "path", c.path, "err", err)
			continue
		}
		need--
		if c.task == nil {
			continue
		}
		c.task.Worktree.Path = ""
		c.task.UpdatedAt = s.clock.Now()
		if err := s.repo.Update(ctx, c.task, collections.Version{}); err != nil {
			s.log.Warn("a pruned worktree is still recorded on its task",
				"task", c.task.ID, "err", err)
		}
	}
	if need > 0 {
		return errWorktreeLimit(policy.Limit, len(existing))
	}
	return nil
}

func (s *Service) worktreePolicy(ctx context.Context) (WorktreePolicy, error) {
	if s.policy == nil {
		return WorktreePolicy{BranchPrefix: DefaultBranchPrefix}, nil
	}
	policy, err := s.policy.Worktrees(ctx)
	if err != nil {
		return WorktreePolicy{}, errReadFailed("worktreePolicy", err)
	}
	if policy.BranchPrefix == "" {
		policy.BranchPrefix = DefaultBranchPrefix
	}
	return policy, nil
}

// underRoot reports whether a checkout sits inside the directory the workspace
// places its worktrees in. Compared as a relative path rather than a string
// prefix, so "/w/trees-of-mine" is not inside "/w/trees".
func underRoot(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
