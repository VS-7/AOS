package task

import (
	"errors"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
)

// A checkout is the task's own only at the place Branch puts it: the worktree
// root, then the task's id. Being a checkout of the workspace under that root
// is not enough — every task's checkout is one — and TASK.md, where the path
// is read back from, is a file any agent that can write in the workspace can
// edit. Task dbf263d7 was given 0186fedb's checkout that way: Delete ran
// `git worktree remove --force` on it and 0186fedb's uncommitted work was
// gone, and a turn on dbf263d7 would have been rooted in 0186fedb's branch.

func twoBranched(t *testing.T) (h *harness, owner, other *View, ownPath string) {
	t.Helper()
	h = newHarness(t)
	owner = h.create(t, CreateInput{Name: "Keep my work", Status: Todo, Worktree: true})
	other = h.create(t, CreateInput{Name: "Borrow a checkout", Status: Todo, Worktree: true})
	tree, err := h.svc.Branch(ctx(), BranchInput{ID: owner.ID})
	if err != nil {
		t.Fatal(err)
	}
	return h, owner, other, tree.Path
}

func TestAnotherTasksCheckoutIsNeverThisTasksRoot(t *testing.T) {
	h, _, other, theirs := twoBranched(t)
	h.record(t, other.ID, theirs)

	if got, err := h.svc.Checkout(ctx(), other.ID); err != nil || got != "" {
		t.Fatalf("a task recording another's checkout: checkout = %q, %v; want none", got, err)
	}
}

func TestDeleteNeverRemovesAnotherTasksCheckout(t *testing.T) {
	h, owner, other, theirs := twoBranched(t)
	h.record(t, other.ID, theirs)

	if _, err := h.svc.Delete(ctx(), DeleteInput{ID: other.ID}); err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.removed) != 0 {
		t.Fatalf("removed = %v, the checkout of task %s", h.worktrees.removed, owner.ID)
	}
	if got, err := h.svc.Checkout(ctx(), owner.ID); err != nil || got != theirs {
		t.Fatalf("the owner's checkout = %q, %v; want it still %q", got, err, theirs)
	}
}

func TestBranchingATaskThatRecordsAnothersCheckoutCutsItsOwn(t *testing.T) {
	h, _, other, theirs := twoBranched(t)
	h.record(t, other.ID, theirs)

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Path != "/tmp/wt/"+other.ID {
		t.Fatalf("branched into %q, want its own %q", tree.Path, "/tmp/wt/"+other.ID)
	}
}

// The prune found a checkout's owner by the path tasks record, so a finished
// task that recorded an unfinished one's checkout made that checkout look like
// finished work, and the prune took it.
func TestThePruneKeepsAnUnfinishedTasksCheckoutWhateverAnotherTaskRecords(t *testing.T) {
	h, owner, other, theirs := twoBranched(t)
	h.record(t, other.ID, theirs)
	finished, err := h.repo.Get(ctx(), collections.Key{"id": other.ID})
	if err != nil {
		t.Fatal(err)
	}
	finished.Status = Finished
	if err := h.repo.Update(ctx(), finished, collections.Version{}); err != nil {
		t.Fatal(err)
	}
	third := h.create(t, CreateInput{Name: "Needs room", Status: Todo, Worktree: true})
	if _, err := h.svc.Branch(ctx(), BranchInput{ID: third.ID}); err != nil {
		t.Fatal(err)
	}

	fourth := h.create(t, CreateInput{Name: "Needs more room", Status: Todo, Worktree: true})
	_, err = h.svc.Branch(ctx(), BranchInput{ID: fourth.ID})
	for _, removed := range h.worktrees.removed {
		if removed == theirs {
			t.Fatalf("the prune removed %s, the checkout of unfinished task %s", removed, owner.ID)
		}
	}
	var app *apperr.Error
	if err == nil || !errors.As(err, &app) {
		t.Fatalf("branching past the limit with nothing finished to prune = %v, want the limit's refusal", err)
	}
}
