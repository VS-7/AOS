package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/task"
)

// taskDirectory stands in for the task aggregate. recorded is what each
// TASK.md says its checkout is; usable is what the task service vouches for
// once that path has been checked. The two differ exactly when the file was
// edited, restored or copied, which is the case a turn must not believe.
type taskDirectory struct {
	recorded map[string]string
	usable   map[string]string
}

func (d taskDirectory) Get(_ context.Context, in task.GetInput) (*task.View, error) {
	path, ok := d.recorded[in.ID]
	if !ok {
		return nil, apperr.New("TASK_NOT_FOUND").Status(apperr.StatusNotFound)
	}
	return &task.View{Task: task.Task{ID: in.ID, Worktree: task.Worktree{Enabled: path != "", Path: path}}}, nil
}

func (d taskDirectory) Checkout(_ context.Context, id string) (string, error) {
	if _, ok := d.recorded[id]; !ok {
		return "", apperr.New("TASK_NOT_FOUND").Status(apperr.StatusNotFound)
	}
	return d.usable[id], nil
}

func writer() *agent.Agent {
	return &agent.Agent{ID: "builder", Sandbox: &agent.Sandbox{Permissions: []string{"read", "write"}}}
}

// tasks_branch promises that "the sandbox root becomes that checkout", and the
// agent's own instructions say the same. Nothing did it: a turn on a task with
// a worktree was confined to the workspace root like any other, so an agent
// executing "on its own branch" edited the main working tree.
func TestATaskBoundTurnIsConfinedToTheTasksCheckout(t *testing.T) {
	workspace, checkout := t.TempDir(), t.TempDir()
	r := &Runner{deps: Deps{
		WorkspaceRoot: workspace,
		Tasks: taskDirectory{
			recorded: map[string]string{"t-1": checkout},
			usable:   map[string]string{"t-1": checkout},
		},
	}}

	box, err := r.sandboxFor(context.Background(), writer(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := box.WriteFile(context.Background(), "library_api.py", []byte("print()\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(checkout, "library_api.py")); err != nil {
		t.Fatalf("the write did not land in the task's checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "library_api.py")); !os.IsNotExist(err) {
		t.Fatal("a task-bound turn wrote into the main working tree")
	}
	if err := box.WriteFile(context.Background(), filepath.Join(workspace, "escape.txt"), []byte("x")); err == nil {
		t.Fatal("a task-bound turn could still reach the workspace root")
	}
}

// TASK.md is a file in the workspace, and any workspace-rooted turn — a DM, the
// orchestrator, a routine — can write it. The recorded path was taken as the
// sandbox root as written, so editing it to the installation's own directory
// let the next task turn write there and read the credentials beside it. The
// root is the checkout the task service vouches for, and a path it does not
// vouch for leaves the turn where any other turn runs.
func TestATaskTurnIsNotRootedWhereAnEditedTaskFileSays(t *testing.T) {
	workspace, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "auth.json"), []byte(`{"token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	real, _ := filepath.EvalSymlinks(workspace)
	r := &Runner{deps: Deps{
		WorkspaceRoot: workspace,
		Tasks: taskDirectory{
			recorded: map[string]string{"t-1": outside},
			usable:   map[string]string{},
		},
	}}

	box, err := r.sandboxFor(context.Background(), writer(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if box.Root() != real {
		t.Fatalf("root = %q, want the workspace %q rather than the recorded path", box.Root(), real)
	}
	if _, err := box.ReadFile(context.Background(), "auth.json"); err == nil {
		t.Fatal("a task turn read a file beside the path its task file was edited to")
	}
	if err := box.WriteFile(context.Background(), filepath.Join(outside, "escaped.txt"), []byte("x")); err == nil {
		t.Fatal("a task turn wrote where its task file was edited to point")
	}
}

// A task with no checkout, a conversation with no task, and a task that no
// longer exists all run where every other turn runs.
func TestATurnWithNoCheckoutRunsInTheWorkspace(t *testing.T) {
	workspace := t.TempDir()
	real, _ := filepath.EvalSymlinks(workspace)
	r := &Runner{deps: Deps{
		WorkspaceRoot: workspace,
		Tasks:         taskDirectory{recorded: map[string]string{"t-2": ""}},
	}}
	for _, id := range []string{"", "t-2", "t-gone"} {
		box, err := r.sandboxFor(context.Background(), writer(), id)
		if err != nil {
			t.Fatalf("task %q: %v", id, err)
		}
		if box.Root() != real {
			t.Errorf("task %q: root = %q, want the workspace", id, box.Root())
		}
	}
}
