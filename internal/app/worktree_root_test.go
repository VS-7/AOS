package app_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// Two workspaces that are folders of one repository — services/api and
// services/web of a monorepo — both cut their tasks' checkouts from that
// repository. The checkouts all went into the installation's one worktree
// directory, so web's prune listed api's checkout among the repository's
// worktrees under "its" root, found no task of its own named after it, took it
// for a leftover and removed it with --force: branching a task in web lost the
// uncommitted work of an unfinished task in api.
func TestBranchingInOneWorkspaceNeverPrunesAnotherWorkspacesCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	home, first := realTempDir(t), t.TempDir()
	mono := filepath.Join(t.TempDir(), "mono")
	api := filepath.Join(mono, "services", "api")
	web := filepath.Join(mono, "services", "web")
	for _, dir := range []string{api, web} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# service\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, mono, "init", "-b", "main")
	gitIn(t, mono, "add", "-A")
	gitIn(t, mono, "commit", "-m", "Two services")

	a, err := app.New(app.Options{
		Env:           env.New(env.Map(map[string]string{env.KeyHome: home})),
		WorkspaceRoot: first,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	for name, dir := range map[string]string{"Api": api, "Web": web} {
		if _, err := a.Workspaces.Create(t.Context(), workspace.CreateInput{Name: name, Path: dir}); err != nil {
			t.Fatal(err)
		}
	}
	// Committed so the workspaces' own .aos files do not count as work a
	// checkout of the project is missing.
	gitIn(t, mono, "add", "-A")
	gitIn(t, mono, "commit", "-m", "Register both workspaces")

	invokeIn(inWorkspace("web"), t, a, "workspace_update",
		`{"_reasoning":"a test keeps one checkout in web","set":{"worktrees.worktreeLimit":1,"worktrees.deleteOldWorktrees":true}}`)

	branch := func(workspaceID, name string) string {
		var created struct {
			ID string `json:"id"`
		}
		raw := invokeIn(inWorkspace(workspaceID), t, a, "tasks_create",
			`{"_reasoning":"a test needs a task with a checkout","name":"`+name+`","status":"todo","worktree":true}`)
		if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
			t.Fatalf("tasks_create answered %s (%v)", raw, err)
		}
		var tree struct {
			Path string `json:"path"`
		}
		raw = invokeIn(inWorkspace(workspaceID), t, a, "tasks_branch",
			`{"_reasoning":"a test needs the checkout","id":"`+created.ID+`"}`)
		if err := json.Unmarshal(raw, &tree); err != nil || tree.Path == "" {
			t.Fatalf("tasks_branch answered %s (%v)", raw, err)
		}
		return tree.Path
	}

	apiCheckout := branch("api", "Unfinished api work")
	wip := filepath.Join(apiCheckout, "services", "api", "wip.txt")
	if err := os.WriteFile(wip, []byte("not committed yet\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	webCheckout := branch("web", "New web work")
	if _, err := os.Stat(webCheckout); err != nil {
		t.Fatalf("web's own checkout is not there: %v", err)
	}
	if _, err := os.Stat(wip); err != nil {
		t.Fatalf("branching in web removed api's unfinished checkout and its uncommitted work: %v", err)
	}
}

// realTempDir is a temporary directory spelled the way git reports it: on
// macOS t.TempDir() is under /var, which is a link to /private/var.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
