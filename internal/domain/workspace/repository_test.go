package workspace_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/workspace"
)

// Creating a workspace gave it a repository whenever its directory was not the
// top of one, and that included a directory somebody chose inside a repository
// of their own — a monorepo's subfolder got a nested .git and an empty commit,
// task checkouts were cut from that nearly empty repository instead of the
// project, and `git add -A` in the project recorded the folder as an embedded
// repository. A repository is made only where it is this installation's to
// make: a workspace directory it created under its own workspaces directory,
// or a directory inside no repository at all.

func TestAChosenPathInsideSomebodysRepositoryIsLeftToIt(t *testing.T) {
	h := newHarness(t)
	h.git.enclosing[repoRoot] = "/home/me"

	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	if h.git.inits != 0 || h.git.commits[repoRoot] != 0 || out.Scaffold.GitInit {
		t.Fatalf("inits = %d, commits = %v, report = %+v; want the enclosing repository left to its owner",
			h.git.inits, h.git.commits, out.Scaffold)
	}
	if out.Scaffold.GitWarning != "" {
		t.Errorf("warning = %q; a workspace inside a repository is versioned already", out.Scaffold.GitWarning)
	}
}

// Where the person's broken workspace lives: this installation's own
// directory, inside a home directory that is a repository with no commit.
// That one is this installation's to version.
func TestAWorkspaceDirectoryThisInstallationMadeGetsItsOwnRepository(t *testing.T) {
	h := newHarness(t)
	managed := "/state/workspaces/project-alpha/workspace"
	h.git.enclosing[managed] = "/state"

	out := h.create(t, workspace.CreateInput{Name: "Project Alpha"})
	if out.Workspace.Path != managed {
		t.Fatalf("path = %q, want the installation's own %q", out.Workspace.Path, managed)
	}
	if h.git.inits != 1 || h.git.commits[managed] != 1 || !out.Scaffold.GitInit {
		t.Fatalf("inits = %d, commits = %v, report = %+v; want a repository of its own with a first commit",
			h.git.inits, h.git.commits, out.Scaffold)
	}
}

// A workspace this installation made before that rule existed has no
// repository of its own, and it is given one when it is opened — never one
// somebody chose.
func TestAManagedWorkspaceWithoutARepositoryIsGivenOneWhenOpened(t *testing.T) {
	h := newHarness(t)
	managed := "/state/workspaces/vs/workspace"
	h.git.enclosing[managed] = "/state"
	h.git.enclosing[repoRoot] = "/home/me"

	if initialised, warning := h.svc.EnsureManagedRepository(ctx(), managed); !initialised || warning != "" {
		t.Fatalf("managed: initialised = %v, warning = %q", initialised, warning)
	}
	if h.git.commits[managed] != 1 {
		t.Errorf("commits = %v, want the first commit a branch is cut from", h.git.commits)
	}
	if initialised, _ := h.svc.EnsureManagedRepository(ctx(), managed); initialised || h.git.inits != 1 {
		t.Errorf("opening it again initialised it again (inits = %d)", h.git.inits)
	}
	for _, chosen := range []string{repoRoot, "/elsewhere", "/state/workspaces"} {
		if initialised, _ := h.svc.EnsureManagedRepository(ctx(), chosen); initialised {
			t.Errorf("%s is not this installation's to version, and was given a repository", chosen)
		}
	}
	if h.git.inits != 1 {
		t.Errorf("inits = %d, want only the managed workspace's", h.git.inits)
	}
}
