package task

import (
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

// A task's worktree.path is read back from TASK.md, inside a workspace that is
// versioned, copied and shared. It is the root a task's turns are confined to,
// so a path that is no longer a checkout, or never was one of ours, must not be
// taken at its word.

// record writes a checkout path onto a task the way a stale or edited TASK.md
// arrives: straight into the stored file, past Branch.
func (h *harness) record(t *testing.T, id, path string) {
	t.Helper()
	stored, err := h.repo.Get(ctx(), collections.Key{"id": id})
	if err != nil {
		t.Fatal(err)
	}
	stored.Worktree.Enabled = true
	stored.Worktree.Path = path
	if err := h.repo.Update(ctx(), stored, collections.Version{}); err != nil {
		t.Fatal(err)
	}
}

func TestATurnIsConfinedOnlyToACheckoutThatIsThere(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})

	if got, err := h.svc.Checkout(ctx(), task.ID); err != nil || got != "" {
		t.Fatalf("an unbranched task: checkout = %q, %v; want none", got, err)
	}

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := h.svc.Checkout(ctx(), task.ID); err != nil || got != tree.Path {
		t.Fatalf("a branched task: checkout = %q, %v; want %q", got, err, tree.Path)
	}

	// Deleted by hand, lost with the data directory, or never restored from a
	// backup: the task is back to having no checkout, and its turns run where
	// they ran before it was branched — not nowhere.
	h.worktrees.existing = nil
	if got, err := h.svc.Checkout(ctx(), task.ID); err != nil || got != "" {
		t.Fatalf("a checkout that is gone: checkout = %q, %v; want none", got, err)
	}
}

func TestACheckoutRecordedOutsideTheWorktreeRootIsNeverTheRoot(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	// Both are "there" as far as git is concerned: one is somebody's own
	// worktree, one is a sibling directory whose name merely starts the same.
	for _, foreign := range []string{"/home/someone/my-own-branch", "/tmp/wt-other/" + task.ID} {
		h.worktrees.existing = []string{foreign}
		h.record(t, task.ID, foreign)

		if got, err := h.svc.Checkout(ctx(), task.ID); err != nil || got != "" {
			t.Errorf("recorded %s: checkout = %q, %v; want none", foreign, got, err)
		}
	}
}

// tasks_branch returned the recorded path as long as there was one, so a
// checkout that had gone could not be brought back by the command that makes
// it. It is cut again, on the task's own branch, so what was committed there
// comes back with it.
func TestBranchingAgainBringsBackACheckoutThatIsGone(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	first, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	h.worktrees.existing = nil

	again, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.created) != 2 {
		t.Fatalf("created = %+v, want the checkout cut again", h.worktrees.created)
	}
	if recut := h.worktrees.created[1]; recut.Branch != first.Branch || recut.Path != first.Path {
		t.Errorf("cut again as %+v, want branch %q at %q", recut, first.Branch, first.Path)
	}
	if again.Path != first.Path {
		t.Errorf("path = %q, want %q", again.Path, first.Path)
	}
	if got, _ := h.svc.Checkout(ctx(), task.ID); got != first.Path {
		t.Errorf("checkout after branching again = %q, want %q", got, first.Path)
	}
}

// The branch recorded on the task is the one it comes back on, even when the
// workspace's prefix or the task's name has changed since.
func TestACheckoutIsCutAgainOnTheBranchTheTaskRecorded(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	stored, err := h.repo.Get(ctx(), collections.Key{"id": task.ID})
	if err != nil {
		t.Fatal(err)
	}
	stored.Worktree = Worktree{Enabled: true, Branch: "feature/the-old-name", Path: "/tmp/wt/" + task.ID}
	if err := h.repo.Update(ctx(), stored, collections.Version{}); err != nil {
		t.Fatal(err)
	}

	if _, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.created) != 1 || h.worktrees.created[0].Branch != "feature/the-old-name" {
		t.Fatalf("created = %+v, want the recorded branch", h.worktrees.created)
	}
}

func TestBranchingATaskWhosePathIsNotOursCutsOneUnderTheRoot(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	foreign := "/home/someone/my-own-branch"
	h.worktrees.existing = []string{foreign}
	h.record(t, task.ID, foreign)

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Path != "/tmp/wt/"+task.ID {
		t.Errorf("path = %q, want a checkout under the workspace's worktree root", tree.Path)
	}
	for _, removed := range h.worktrees.removed {
		if removed == foreign {
			t.Fatal("a worktree outside the workspace's root was removed")
		}
	}
}

// Delete removed whatever path the task recorded, with --force.
func TestDeleteLeavesACheckoutTheWorkspaceDidNotPlace(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	foreign := "/home/someone/my-own-branch"
	h.worktrees.existing = []string{foreign}
	h.record(t, task.ID, foreign)

	if _, err := h.svc.Delete(ctx(), DeleteInput{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.removed) != 0 {
		t.Fatalf("removed = %v, want nothing outside the workspace's root", h.worktrees.removed)
	}
}

// Under the root by its spelling is not enough either. A link placed there
// that leads to somebody's own checkout reads as one of the workspace's, and
// `git worktree remove --force` follows it and deletes the checkout it leads
// to. Delete removes only a checkout the adapter vouches for: registered, on
// disk, and under the root once links are resolved.
func TestDeleteLeavesAPathUnderTheRootThatIsNotOneOfItsCheckouts(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	link := "/tmp/wt/" + task.ID
	h.worktrees.existing = nil // the adapter does not vouch for it
	h.record(t, task.ID, link)

	if _, err := h.svc.Delete(ctx(), DeleteInput{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.removed) != 0 {
		t.Fatalf("removed = %v, want nothing the adapter did not vouch for", h.worktrees.removed)
	}
}
