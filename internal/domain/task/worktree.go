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
	policy, err := s.worktreePolicy(ctx)
	if err != nil {
		return nil, err
	}

	// A recorded checkout that is still one of ours is the answer. One that is
	// gone — deleted by hand, lost with the data directory, a workspace copied
	// to another machine — is cut again: this used to return the recorded path
	// as long as there was one, so the command that makes a checkout could not
	// bring a lost one back, and the task's conversation had no way out.
	recorded := current.Worktree.Path
	if recorded != "" {
		if s.ownCheckout(ctx, policy, current.ID, recorded) {
			return &current.Worktree, nil
		}
		s.log.Warn("a task's recorded checkout is not there; cutting it again",
			"task", current.ID, "path", recorded)
	}

	// The branch the task already has, when it has one, so a checkout cut
	// again comes back to the work committed on it rather than to a new
	// branch named after whatever the task is called today.
	branch := strings.TrimSpace(in.Branch)
	if branch == "" {
		branch = current.Worktree.Branch
	}
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
	spec := WorktreeSpec{
		TaskID: current.ID, Branch: branch, Base: base, Path: filepath.Join(policy.Root, current.ID),
	}

	// Asked before anything is pruned or created. The ways a workspace has
	// nothing to cut from — it is in no repository, its base has no commit, or
	// it is a folder the project it sits in never committed — all reached git,
	// and what came back was TASK_WORKTREE_FAILED with the reason in a cause
	// nothing renders, which is where an executor agent stopped the task for
	// good.
	//
	// A workspace that is a folder of somebody's project is cut from that
	// project: the checkout is of the project, and the task's turns are rooted
	// in the folder inside it (see Checkout). Workspace creation leaves such a
	// folder to the project rather than nesting a repository in it.
	source, err := s.worktrees.Source(ctx, spec)
	if err != nil {
		return nil, errWorktreeFailed(current.ID, branch, err)
	}
	if source.Toplevel == "" {
		return nil, errWorktreeNoRepository(current.ID, source)
	}
	if !source.BaseExists {
		return nil, errWorktreeBaseMissing(current.ID, base, source)
	}
	if !source.Own && !source.SubdirCommitted {
		return nil, errWorktreeNotCommitted(current.ID, source)
	}

	if policy.Limit > 0 {
		if err := s.pruneToLimit(ctx, policy); err != nil {
			return nil, err
		}
	}

	created, err := s.worktrees.Create(ctx, spec)
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
		if err := s.setup.Run(ctx, current.Assigned, workspaceInCheckout(created, source), script); err != nil {
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

// Checkout is the directory a turn on this task is confined to: the task's
// isolated checkout, or "" when it has none that can be used — and then the
// turn runs in the workspace, as it did before the task was branched.
//
// The recorded path is read back from TASK.md, which lives in a workspace that
// is versioned, copied and edited, and it becomes a sandbox root. So it counts
// only when it is the checkout this installation placed for this task and it
// is still there. A path edited to point anywhere else used to become the root
// as written, and a checkout that had gone failed every turn in the task's
// conversation for good.
func (s *Service) Checkout(ctx context.Context, id string) (string, error) {
	current, err := s.load(ctx, id)
	if err != nil {
		return "", err
	}
	recorded := current.Worktree.Path
	if recorded == "" || s.worktrees == nil {
		return "", nil
	}
	policy, err := s.worktreePolicy(ctx)
	if err != nil {
		return "", err
	}
	if !s.ownCheckout(ctx, policy, current.ID, recorded) {
		s.log.Warn("a task's recorded checkout is not one of this workspace's; its turn runs in the workspace",
			"task", current.ID, "path", recorded, "worktreeRoot", policy.Root)
		return "", nil
	}
	// A workspace that is a folder of a project has a checkout of the whole
	// project, and its turns belong in the folder, where the paths the agent
	// knows the workspace by still mean the same files.
	//
	// Asked of where the workspace sits in its repository, not of the task's
	// branch. Source answered it, on every turn — up to four git processes —
	// and stopped before the folder whenever the branch and its base were
	// gone, a branch renamed with its checkout still there, which rooted the
	// turn in the whole project.
	dir, found, err := s.worktrees.WorkspaceIn(ctx, recorded)
	if err != nil {
		return "", errReadFailed("Checkout", err)
	}
	if !found {
		// Still the task's own isolated checkout, only without the folder:
		// the agent moved or removed it on its branch.
		s.log.Warn("a task's checkout does not hold the workspace's folder; its turn runs at the checkout's top",
			"task", current.ID, "path", recorded)
	}
	return dir, nil
}

// workspaceInCheckout is where the workspace is inside a checkout cut from
// source: the checkout itself when the workspace is its own repository, the
// workspace's folder inside it when the workspace is a folder of a project.
func workspaceInCheckout(checkout string, source WorktreeSource) string {
	if source.Own || source.Subdir == "" || !filepath.IsLocal(source.Subdir) {
		return checkout
	}
	return filepath.Join(checkout, source.Subdir)
}

// ownCheckout reports whether a recorded path is the checkout this workspace
// placed for the task — the one place Branch puts it, the task's id under the
// worktree root — and git still has it on disk.
//
// Under the root is not enough. Every task's checkout is under it, and TASK.md
// is a file any agent that can write in the workspace can edit: a task given
// another task's checkout took it as its own, rooted its turns in that task's
// branch, and Delete removed it with --force, uncommitted work and all.
//
// A checkout placed before each workspace had its own root, at the task's id
// under the installation-wide one, is still the task's: its turns go on in it
// and Delete still takes it.
func (s *Service) ownCheckout(ctx context.Context, policy WorktreePolicy, taskID, path string) bool {
	recorded := filepath.Clean(path)
	for _, root := range []string{policy.Root, policy.LegacyRoot} {
		if place := checkoutOf(root, taskID); place != "" && place == recorded {
			return s.worktrees.Exists(ctx, root, path)
		}
	}
	return false
}

// checkoutOf is where a task's checkout is under root, or "" when there is no
// such place.
func checkoutOf(root, taskID string) string {
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) || taskID == "" || !filepath.IsLocal(taskID) || filepath.Base(taskID) != taskID {
		return ""
	}
	return filepath.Join(root, taskID)
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
	root := strings.TrimSpace(policy.Root)
	if !policy.DeleteOld || root == "" {
		return nil
	}

	// Only this workspace's own checkouts: the ones under its own root.
	// `git worktree list` reports every worktree of the repository — the
	// person's own branches, and the checkouts of every other workspace that is
	// a folder of the same repository. Counting those against a limit that
	// bounds this workspace's checkouts let them consume the budget, and then
	// the prune reached for them: branching a task in one folder of a monorepo
	// removed an unfinished task's checkout in another, with --force.
	mine, err := s.worktrees.List(ctx, root)
	if err != nil {
		return errWorktreeFailed("", "", err)
	}
	var legacy []string
	if old := strings.TrimSpace(policy.LegacyRoot); old != "" && filepath.Clean(old) != filepath.Clean(root) {
		if legacy, err = s.worktrees.List(ctx, old); err != nil {
			return errWorktreeFailed("", "", err)
		}
	}
	if len(mine)+len(legacy) < policy.Limit {
		return nil
	}

	all, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return errReadFailed("pruneToLimit", err)
	}
	// A checkout belongs to the task whose id it is named after, which is
	// where Branch put it — not to whichever task records its path. A
	// finished task whose TASK.md was edited to name an unfinished task's
	// checkout made that checkout look like finished work, and it was
	// pruned out from under the task still working in it.
	byID := map[string]*Task{}
	for i := range all {
		byID[all[i].ID] = &all[i]
	}

	type candidate struct {
		path string
		task *Task
	}
	existing := len(mine)
	var removable []candidate
	for _, path := range mine {
		owner, ok := byID[filepath.Base(path)]
		if !ok || filepath.Dir(path) != filepath.Clean(root) {
			// A checkout under this workspace's root that no task of it is
			// named after: a leftover from a run that did not finish. Nothing
			// is executing in it and nothing points at it — the root is this
			// workspace's alone, so it is nobody else's either.
			removable = append(removable, candidate{path: path})
			continue
		}
		if owner.Status.Terminal() {
			removable = append(removable, candidate{path: path, task: owner})
		}
	}
	// Under the old shared root only a checkout named after one of this
	// workspace's tasks is this workspace's; the rest may be any workspace's.
	for _, path := range legacy {
		owner, ok := byID[filepath.Base(path)]
		if !ok || filepath.Dir(path) != filepath.Clean(policy.LegacyRoot) {
			continue
		}
		existing++
		if owner.Status.Terminal() {
			removable = append(removable, candidate{path: path, task: owner})
		}
	}
	if existing < policy.Limit {
		return nil
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

	need := existing - policy.Limit + 1
	for _, c := range removable {
		if need <= 0 {
			break
		}
		if err := s.worktrees.Remove(ctx, c.path); err != nil {
			s.log.Warn("an old worktree could not be pruned", "path", c.path, "err", err)
			continue
		}
		need--
		// Only a task recording the checkout that was removed loses the
		// record: one that names some other path was never pointing at it.
		if c.task == nil || filepath.Clean(c.task.Worktree.Path) != c.path {
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
		return errWorktreeLimit(policy.Limit, existing)
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
