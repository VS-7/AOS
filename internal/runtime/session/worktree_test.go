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

// taskDirectory answers the one question a turn asks about its task.
type taskDirectory map[string]*task.View

func (d taskDirectory) Get(_ context.Context, in task.GetInput) (*task.View, error) {
	if found, ok := d[in.ID]; ok {
		return found, nil
	}
	return nil, apperr.New("TASK_NOT_FOUND").Status(apperr.StatusNotFound)
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
		Tasks: taskDirectory{"t-1": {Task: task.Task{
			ID: "t-1", Worktree: task.Worktree{Enabled: true, Branch: "aos/t-1", Path: checkout},
		}}},
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

// A task with no checkout, a conversation with no task, and a task that no
// longer exists all run where every other turn runs.
func TestATurnWithNoCheckoutRunsInTheWorkspace(t *testing.T) {
	workspace := t.TempDir()
	real, _ := filepath.EvalSymlinks(workspace)
	r := &Runner{deps: Deps{
		WorkspaceRoot: workspace,
		Tasks:         taskDirectory{"t-2": {Task: task.Task{ID: "t-2"}}},
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
