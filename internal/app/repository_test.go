package app_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/env"
)

// The person's workspace lives at ~/.aos/workspaces/vs/workspace, inside a home
// directory that is a repository with no commit, and it was made before
// creating a workspace gave it a repository of its own. Every task branched
// there was refused for good. Opening a workspace this installation made gives
// it one — and a directory somebody chose inside a repository of theirs is
// left exactly as it was.
func TestOpeningAWorkspaceThisInstallationMadeGivesItARepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	home := t.TempDir()
	state := filepath.Join(home, ".aos")
	gitIn(t, home, "init", "-b", "main")

	managed := filepath.Join(state, "workspaces", "vs", "workspace")
	chosen := filepath.Join(home, "code", "mono", "services", "api")
	for _, dir := range []string{managed, chosen} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, root := range []string{managed, chosen} {
		a, err := app.New(app.Options{
			Env:           env.New(env.Map(map[string]string{env.KeyHome: state})),
			WorkspaceRoot: root,
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = a.Close()
	}

	if top := strings.TrimSpace(gitIn(t, managed, "rev-parse", "--show-toplevel")); !sameDir(t, top, managed) {
		t.Fatalf("the managed workspace's repository is %s, want its own", top)
	}
	if out := gitIn(t, managed, "rev-list", "--count", "HEAD"); strings.TrimSpace(out) != "1" {
		t.Errorf("the managed workspace's repository has %s commits, want the first one a task is cut from", out)
	}
	if _, err := os.Stat(filepath.Join(chosen, ".git")); err == nil {
		t.Error("a directory inside somebody's repository was given a repository of its own")
	}
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=aos", "GIT_AUTHOR_EMAIL=aos@example.invalid",
		"GIT_COMMITTER_NAME=aos", "GIT_COMMITTER_EMAIL=aos@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(fa, fb)
}
