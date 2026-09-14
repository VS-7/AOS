package gitcli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/domain/task"
)

// A workspace somebody chose inside a repository of their own — a monorepo's
// subfolder — is versioned by that repository, and its tasks are cut from it:
// the checkout is of the project, and the workspace is the subfolder inside
// it. Workspace creation no longer nests a repository there, so refusing to cut
// from the enclosing one would leave such a workspace with no tasks at all.

func TestATaskInAWorkspaceInsideAProjectIsCutFromTheProject(t *testing.T) {
	outer, inner := nested(t)
	commit(t, outer, "the workspace is part of the project")
	trees := gitcli.NewWorktrees(gitcli.New(), inner)
	spec := task.WorktreeSpec{TaskID: "t-1", Branch: "aos/fix-it"}

	source, err := trees.Source(ctx(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if source.Own || !sameFile(t, source.Toplevel, outer) || !source.BaseExists {
		t.Fatalf("source = %+v; want the project above it, with a commit to cut from", source)
	}
	if source.Subdir != filepath.Join("workspaces", "vs") || !source.SubdirCommitted {
		t.Fatalf("source = %+v; want the workspace found committed at workspaces/vs", source)
	}

	root := filepath.Join(t.TempDir(), "wt")
	spec.Path = filepath.Join(root, "t-1")
	path, err := trees.Create(ctx(), spec)
	if err != nil {
		t.Fatalf("cutting a checkout of the project: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "workspaces", "vs", "notes.md")); err != nil {
		t.Fatalf("the checkout does not hold the workspace: %v", err)
	}
	if !trees.Exists(ctx(), root, path) {
		t.Error("the checkout cut from the project does not exist")
	}
	listed, err := trees.List(ctx(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || !sameFile(t, listed[0], path) {
		t.Errorf("list = %v, want the task's checkout and not the project's own working tree", listed)
	}
	if trees.Exists(ctx(), filepath.Dir(outer), outer) {
		t.Error("the project's own working tree counts as a task's checkout")
	}
	if err := trees.Remove(ctx(), path); err != nil {
		t.Fatal(err)
	}
}

// A subfolder the project has not committed is not in a checkout of it, and a
// project with no commit has nothing to check out; both are reported before
// anything is created, so the refusal can say which.
func TestASourceSaysWhenTheProjectCannotHoldTheWorkspace(t *testing.T) {
	t.Run("the workspace is not committed in the project", func(t *testing.T) {
		outer, inner := nested(t)
		source, err := gitcli.NewWorktrees(gitcli.New(), inner).Source(ctx(), task.WorktreeSpec{Branch: "aos/x"})
		if err != nil {
			t.Fatal(err)
		}
		if source.Own || !sameFile(t, source.Toplevel, outer) || !source.BaseExists || source.SubdirCommitted {
			t.Fatalf("source = %+v; want a base that does not hold the workspace", source)
		}
	})

	t.Run("the project has no commit", func(t *testing.T) {
		requireGit(t)
		outer := t.TempDir()
		git(t, outer, "init", "-b", "main")
		inner := filepath.Join(outer, "workspaces", "vs")
		if err := os.MkdirAll(inner, 0o755); err != nil {
			t.Fatal(err)
		}
		source, err := gitcli.NewWorktrees(gitcli.New(), inner).Source(ctx(), task.WorktreeSpec{Branch: "aos/x"})
		if err != nil {
			t.Fatal(err)
		}
		if source.Own || !sameFile(t, source.Toplevel, outer) || source.BaseExists {
			t.Fatalf("source = %+v; want the commit-less project named, with nothing to cut from", source)
		}
	})

	t.Run("no repository anywhere", func(t *testing.T) {
		requireGit(t)
		dir := t.TempDir()
		trees := gitcli.NewWorktrees(gitcli.New(), dir)
		source, err := trees.Source(ctx(), task.WorktreeSpec{Branch: "aos/x"})
		if err != nil {
			t.Fatal(err)
		}
		if source.Toplevel != "" || source.BaseExists {
			t.Fatalf("source = %+v; want no repository", source)
		}
		if _, err := trees.Create(ctx(), task.WorktreeSpec{TaskID: "t-1", Branch: "aos/x", Path: filepath.Join(t.TempDir(), "t-1")}); err == nil {
			t.Fatal("a checkout was cut from a directory in no repository")
		}
	})
}

func TestEnclosingRepositoryNamesOnlySomebodyElsesRepository(t *testing.T) {
	outer, inner := nested(t)
	g := gitcli.New()
	if top, err := g.EnclosingRepository(ctx(), inner); err != nil || !sameFile(t, top, outer) {
		t.Fatalf("EnclosingRepository(inner) = %q, %v; want %s", top, err, outer)
	}
	if top, err := g.EnclosingRepository(ctx(), outer); err != nil || top != "" {
		t.Fatalf("EnclosingRepository(outer) = %q, %v; want none, it is its own", top, err)
	}
	if top, err := g.EnclosingRepository(ctx(), t.TempDir()); err != nil || top != "" {
		t.Fatalf("EnclosingRepository(plain) = %q, %v; want none", top, err)
	}
}
