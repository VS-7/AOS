package gitcli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/domain/task"
)

// A workspace is often a directory inside a repository that is not its own —
// the person's home directory is a repository on the machine these were found
// on, and every workspace under ~/.aos sits inside it. Git discovers the
// enclosing repository from any directory below it, and every call here used
// to accept that answer: the Changes panel listed the home directory
// (.ssh/, Library/, .zsh_history), `workspace create` decided the workspace
// was already versioned and never gave it a repository, and tasks_branch cut
// checkouts from the enclosing repository's unborn main.

// nested is a directory inside a real repository, with a file of its own.
func nested(t *testing.T) (outer, inner string) {
	t.Helper()
	outer = repository(t)
	inner = filepath.Join(outer, "workspaces", "vs")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "notes.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, "cookies.txt"), []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return outer, inner
}

func TestADirectoryInsideAnotherRepositoryIsNotARepositoryOfItsOwn(t *testing.T) {
	outer, inner := nested(t)
	g := gitcli.New()

	if own, err := g.IsRepository(ctx(), inner); err != nil || own {
		t.Fatalf("IsRepository(inner) = %v, %v; want false: the repository above it is somebody else's", own, err)
	}
	if own, err := g.IsRepository(ctx(), outer); err != nil || !own {
		t.Fatalf("IsRepository(outer) = %v, %v; want true", own, err)
	}
}

func TestTheChangesOfANestedWorkspaceAreNotTheEnclosingRepositorys(t *testing.T) {
	_, inner := nested(t)
	g := gitcli.New()

	if changes, err := g.Changes(ctx(), inner); err == nil {
		t.Fatalf("Changes = %+v; want the refusal a directory outside any repository gets", changes)
	}
	if status, err := g.Status(ctx(), inner, "cookies.txt"); err == nil {
		t.Fatalf("Status = %q; want a refusal, not the enclosing repository's answer", status)
	}
	if _, ok, _ := g.Show(ctx(), inner, "HEAD", "README.md"); ok {
		t.Fatal("Show read a file out of the enclosing repository")
	}
	if origin, _ := g.OriginURL(ctx(), inner); origin != "" {
		t.Fatalf("OriginURL = %q; want none, the remote belongs to the enclosing repository", origin)
	}
}

func TestAWorktreeSourceSaysWhyNothingCanBeCut(t *testing.T) {
	requireGit(t)

	t.Run("inside another repository", func(t *testing.T) {
		outer, inner := nested(t)
		got, err := gitcli.NewWorktrees(gitcli.New(), inner).Source(ctx(), task.WorktreeSpec{Branch: "aos/x", Base: "main"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Own || !sameFile(t, got.Toplevel, outer) {
			t.Fatalf("source = %+v; want not its own, inside %s", got, outer)
		}
	})

	t.Run("a repository with no commit", func(t *testing.T) {
		dir := t.TempDir()
		git(t, dir, "init", "-b", "main")
		got, err := gitcli.NewWorktrees(gitcli.New(), dir).Source(ctx(), task.WorktreeSpec{Branch: "aos/x", Base: "main"})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Own || got.BaseExists {
			t.Fatalf("source = %+v; want its own repository with no base to cut from", got)
		}
	})

	t.Run("a base that exists, and one that does not", func(t *testing.T) {
		repo := repository(t)
		trees := gitcli.NewWorktrees(gitcli.New(), repo)
		if got, err := trees.Source(ctx(), task.WorktreeSpec{Branch: "aos/x", Base: "main"}); err != nil || !got.Own || !got.BaseExists {
			t.Fatalf("main: source = %+v, %v", got, err)
		}
		if got, err := trees.Source(ctx(), task.WorktreeSpec{Branch: "aos/x"}); err != nil || !got.BaseExists {
			t.Fatalf("no base (HEAD): source = %+v, %v", got, err)
		}
		if got, err := trees.Source(ctx(), task.WorktreeSpec{Branch: "aos/x", Base: "develop"}); err != nil || got.BaseExists {
			t.Fatalf("develop: source = %+v, %v; want no such base", got, err)
		}
		// A branch that already exists is checked out, so its base no longer
		// matters.
		git(t, repo, "branch", "aos/existing")
		if got, err := trees.Source(ctx(), task.WorktreeSpec{Branch: "aos/existing", Base: "develop"}); err != nil || !got.BaseExists {
			t.Fatalf("existing branch: source = %+v, %v", got, err)
		}
	})
}

// A repository git init just made has no commit, and nothing can be branched
// from it until one exists. The bootstrap commit must work on a machine where
// nobody configured an identity, and must not wait on a signing prompt.
func TestTheFirstCommitNeedsNoIdentity(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	// No identity, no guessing one from the host name — which is what a CI
	// runner looks like — and a signing program that cannot run.
	global := filepath.Join(t.TempDir(), "gitconfig")
	config := "[user]\n\tuseConfigOnly = true\n[commit]\n\tgpgsign = true\n[gpg]\n\tprogram = /nonexistent/gpg\n"
	if err := os.WriteFile(global, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "none"))
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL"} {
		t.Setenv(key, "")
		_ = os.Unsetenv(key)
	}

	if err := gitcli.New().CommitEmpty(ctx(), dir, "Start the workspace"); err != nil {
		t.Fatal(err)
	}
	got, err := gitcli.NewWorktrees(gitcli.New(), dir).Source(ctx(), task.WorktreeSpec{Branch: "aos/x"})
	if err != nil || !got.BaseExists {
		t.Fatalf("source = %+v, %v; want a commit to cut from", got, err)
	}
}
