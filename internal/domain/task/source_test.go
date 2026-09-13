package task

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
)

// tasks_branch in a workspace with nothing to cut from answered
// AOS_TASK_WORKTREE_FAILED "the isolated checkout could not be created" — no
// reason, no next step — and the executor stopped the task on it. The reason
// was always one of two, and both have a fix somebody can act on.

func TestAWorkspaceInsideAnotherRepositoryIsRefusedWithTheReason(t *testing.T) {
	h := newHarness(t)
	h.worktrees.source = &WorktreeSource{Dir: "/home/me/.aos/workspaces/vs/workspace", Toplevel: "/home/me"}
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true, Base: "main"})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	app, ok := apperr.As(err)
	if !ok || app.Code != "AOS_TASK_WORKTREE_NO_REPOSITORY" {
		t.Fatalf("err = %v, want AOS_TASK_WORKTREE_NO_REPOSITORY", err)
	}
	if !strings.Contains(app.Message, "/home/me") || len(app.Actions) == 0 {
		t.Errorf("error = %+v, want the enclosing repository named and a next step", app)
	}
	if len(h.worktrees.created) != 0 || len(h.worktrees.removed) != 0 {
		t.Errorf("created %v and pruned %v before refusing", h.worktrees.created, h.worktrees.removed)
	}
}

func TestABaseWithNoCommitIsRefusedWithTheReason(t *testing.T) {
	h := newHarness(t)
	h.worktrees.source = &WorktreeSource{Dir: "/w", Toplevel: "/w", Own: true}
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true, Base: "main"})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	app, ok := apperr.As(err)
	if !ok || app.Code != "AOS_TASK_WORKTREE_BASE_MISSING" {
		t.Fatalf("err = %v, want AOS_TASK_WORKTREE_BASE_MISSING", err)
	}
	if !strings.Contains(app.Message, `"main"`) || len(app.Actions) == 0 {
		t.Errorf("error = %+v, want the base named and a next step", app)
	}
	if len(h.worktrees.created) != 0 {
		t.Errorf("created %v before refusing", h.worktrees.created)
	}
}

// When git itself fails, what git said is the reason, and it belongs in the
// message a person reads rather than only in a cause nothing renders.
func TestAGitFailureSaysWhatGitSaid(t *testing.T) {
	h := newHarness(t)
	h.worktrees.failWith = errors.New("fatal: 'aos/x' is already checked out at '/elsewhere'")
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	app, ok := apperr.As(err)
	if !ok || app.Code != "AOS_TASK_WORKTREE_FAILED" {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(app.Message, "already checked out") || len(app.Actions) == 0 {
		t.Errorf("error = %+v, want git's reason and a next step", app)
	}
}
