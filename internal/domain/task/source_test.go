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

func TestAWorkspaceInNoRepositoryIsRefusedWithTheReason(t *testing.T) {
	h := newHarness(t)
	h.worktrees.source = &WorktreeSource{Dir: "/home/me/notes"}
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true, Base: "main"})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	app, ok := apperr.As(err)
	if !ok || app.Code != "AOS_TASK_WORKTREE_NO_REPOSITORY" {
		t.Fatalf("err = %v, want AOS_TASK_WORKTREE_NO_REPOSITORY", err)
	}
	if !strings.Contains(app.Message, "/home/me/notes") || len(app.Actions) == 0 {
		t.Errorf("error = %+v, want the workspace named and a next step", app)
	}
	if len(h.worktrees.created) != 0 || len(h.worktrees.removed) != 0 {
		t.Errorf("created %v and pruned %v before refusing", h.worktrees.created, h.worktrees.removed)
	}
}

// The workspace this was found on: a directory inside a home directory that is
// a repository with no commit. Nothing can be checked out of that repository,
// and the refusal names it and says both ways out.
func TestAWorkspaceInsideARepositoryWithNoCommitIsRefusedWithTheReason(t *testing.T) {
	h := newHarness(t)
	h.worktrees.source = &WorktreeSource{Dir: "/home/me/.aos/workspaces/vs/workspace", Toplevel: "/home/me", Subdir: ".aos/workspaces/vs/workspace"}
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	app, ok := apperr.As(err)
	if !ok || app.Code != "AOS_TASK_WORKTREE_BASE_MISSING" {
		t.Fatalf("err = %v, want AOS_TASK_WORKTREE_BASE_MISSING", err)
	}
	if !strings.Contains(app.Message, "/home/me") || len(app.Actions) < 2 {
		t.Errorf("error = %+v, want the enclosing repository named, and a commit there or a repository of its own offered", app)
	}
	if len(h.worktrees.created) != 0 || len(h.worktrees.removed) != 0 {
		t.Errorf("created %v and pruned %v before refusing", h.worktrees.created, h.worktrees.removed)
	}
}

// A workspace somebody chose inside their project is a folder of that project,
// and its task is a checkout of the project with its turns rooted in the folder.
func TestAWorkspaceInsideAProjectBranchesFromTheProject(t *testing.T) {
	h := newHarness(t)
	h.worktrees.source = &WorktreeSource{
		Dir: "/code/mono/services/api", Toplevel: "/code/mono", BaseExists: true,
		Subdir: "services/api", SubdirCommitted: true,
	}
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true})

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Path != "/tmp/wt/"+task.ID {
		t.Fatalf("checkout = %q, want the task's own under the root", tree.Path)
	}
	if got, err := h.svc.Checkout(ctx(), task.ID); err != nil || got != "/tmp/wt/"+task.ID+"/services/api" {
		t.Fatalf("checkout for a turn = %q, %v; want the workspace's folder inside the project's checkout", got, err)
	}
}

func TestAWorkspaceTheProjectNeverCommittedIsRefusedWithTheReason(t *testing.T) {
	h := newHarness(t)
	h.worktrees.source = &WorktreeSource{
		Dir: "/code/mono/scratch", Toplevel: "/code/mono", BaseExists: true, Subdir: "scratch",
	}
	task := h.create(t, CreateInput{Name: "Build the library API", Status: Todo, Worktree: true})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	app, ok := apperr.As(err)
	if !ok || app.Code != "AOS_TASK_WORKTREE_NOT_COMMITTED" {
		t.Fatalf("err = %v, want AOS_TASK_WORKTREE_NOT_COMMITTED", err)
	}
	if !strings.Contains(app.Message, "/code/mono") || !strings.Contains(app.Message, "scratch") || len(app.Actions) == 0 {
		t.Errorf("error = %+v, want the project and the folder named, and a next step", app)
	}
	if len(h.worktrees.created) != 0 {
		t.Errorf("created %v before refusing", h.worktrees.created)
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
