package gitcli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/domain/task"
)

// These run against a real git. Everything else about worktrees is proved over
// a fake driver in the task suite; what is proved here is that the translation
// to `git worktree add/remove/list --porcelain` is the right one — which is
// exactly the part a fake cannot tell you.

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
}

// repository builds a real repository with one commit, which is the minimum a
// worktree can be cut from.
func repository(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx(), "git", args...)
		cmd.Dir = dir
		// A machine with no committer configured would fail the commit below,
		// and this test is not about the user's global git configuration.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=aos", "GIT_AUTHOR_EMAIL=aos@example.invalid",
			"GIT_COMMITTER_NAME=aos", "GIT_COMMITTER_EMAIL=aos@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "first")
	return dir
}

func ctx() context.Context { return context.Background() }

// TestAWorktreeIsARealCheckoutOnItsOwnBranch.
func TestAWorktreeIsARealCheckoutOnItsOwnBranch(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	where := filepath.Join(t.TempDir(), "wt", "t-1")

	path, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/fix-it", Base: "main", Path: where,
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != where {
		t.Fatalf("the checkout is at %q, want %q", path, where)
	}

	// It is a checkout: the committed file is there.
	if _, err := os.Stat(filepath.Join(path, "README.md")); err != nil {
		t.Fatalf("the checkout has no content: %v", err)
	}
	// And it is on its own branch, not the one the repository is on.
	if got := branchOf(t, path); got != "aos/fix-it" {
		t.Fatalf("the checkout is on %q", got)
	}
	if got := branchOf(t, repo); got != "main" {
		t.Fatalf("cutting a worktree moved the main checkout to %q", got)
	}

	listed, err := trees.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %v — the main working tree must not be in it", listed)
	}
	if !sameFile(t, listed[0], path) {
		t.Fatalf("list returned %q, want %q", listed[0], path)
	}
}

// TestABranchThatAlreadyExistsIsCheckedOutRatherThanRecreated. A task that was
// branched, pruned and branched again should return to its own work.
func TestABranchThatAlreadyExistsIsCheckedOutRatherThanRecreated(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	base := t.TempDir()

	first, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/fix-it", Base: "main", Path: filepath.Join(base, "one"),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Work lands on the branch, and the checkout goes away.
	if err := os.WriteFile(filepath.Join(first, "WORK.md"), []byte("done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, first, "the work")
	if err := trees.Remove(ctx(), first); err != nil {
		t.Fatal(err)
	}

	second, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/fix-it", Base: "main", Path: filepath.Join(base, "two"),
	})
	if err != nil {
		t.Fatalf("re-cutting an existing branch failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(second, "WORK.md")); err != nil {
		t.Fatalf("the branch came back without its work: %v", err)
	}
}

// TestRemovingACheckoutThatIsAlreadyGoneIsNotAnError. A prune that fails
// because somebody cleaned up by hand is a prune that stops working.
func TestRemovingACheckoutThatIsAlreadyGoneIsNotAnError(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	where := filepath.Join(t.TempDir(), "gone")

	if _, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/gone", Base: "main", Path: where,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(where); err != nil {
		t.Fatal(err)
	}

	if err := trees.Remove(ctx(), where); err != nil {
		t.Fatalf("removing a checkout somebody already deleted failed: %v", err)
	}
	// The administrative record git keeps separately went with it, so the list
	// does not report a worktree that is not there.
	listed, err := trees.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("list still reports %v", listed)
	}
}

// TestRemovingTakesTheCheckoutAndLeavesTheBranch. The work on it is the point
// of having made it.
func TestRemovingTakesTheCheckoutAndLeavesTheBranch(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	where := filepath.Join(t.TempDir(), "wt")

	path, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/keep-me", Base: "main", Path: where,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := trees.Remove(ctx(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the checkout is still on disk: %v", err)
	}
	if !hasBranch(t, repo, "aos/keep-me") {
		t.Fatal("removing the checkout deleted the branch with the work on it")
	}
}

// TestANonRepositoryIsReportedRatherThanSilentlyDoingNothing.
func TestANonRepositoryIsReportedRatherThanSilentlyDoingNothing(t *testing.T) {
	requireGit(t)
	trees := gitcli.NewWorktrees(gitcli.New(), t.TempDir())

	if _, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/x", Path: filepath.Join(t.TempDir(), "wt"),
	}); err == nil {
		t.Fatal("cutting a worktree outside a repository reported success")
	}
	if _, err := trees.List(ctx()); err == nil {
		t.Fatal("listing worktrees outside a repository reported success")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=aos", "GIT_AUTHOR_EMAIL=aos@example.invalid",
		"GIT_COMMITTER_NAME=aos", "GIT_COMMITTER_EMAIL=aos@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func branchOf(t *testing.T, dir string) string {
	t.Helper()
	return git(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func hasBranch(t *testing.T, repo, branch string) bool {
	t.Helper()
	cmd := exec.CommandContext(ctx(), "git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = repo
	return cmd.Run() == nil
}

func commit(t *testing.T, dir, message string) {
	t.Helper()
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", message)
}

// sameFile compares paths through the filesystem, because macOS reports
// /var/... as /private/var/... and a string comparison fails on a match.
func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}
