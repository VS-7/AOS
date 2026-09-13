package gitcli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/domain/task"
)

// A task's checkout can disappear under git's feet: the data directory is
// deleted, a backup is restored without it, the workspace moves machines. Git
// keeps its record of the checkout either way, and the task keeps the path.

// Exists is what a task's recorded path is checked against before it becomes a
// sandbox root, so it answers for this repository's checkouts on disk only.
func TestOnlyACheckoutOfThisRepositoryThatIsOnDiskExists(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	where := filepath.Join(t.TempDir(), "wt", "t-1")
	if _, err := trees.Create(ctx(), task.WorktreeSpec{
		TaskID: "t-1", Branch: "aos/fix-it", Base: "main", Path: where,
	}); err != nil {
		t.Fatal(err)
	}

	if !trees.Exists(ctx(), where) {
		t.Fatal("a checkout that is there does not exist")
	}
	plain := t.TempDir()
	if trees.Exists(ctx(), plain) {
		t.Error("a plain directory counts as a checkout")
	}
	if trees.Exists(ctx(), repo) {
		t.Error("the main working tree counts as a task's checkout")
	}
	other := repository(t)
	if trees.Exists(ctx(), other) {
		t.Error("another repository counts as a checkout of this one")
	}

	if err := os.RemoveAll(where); err != nil {
		t.Fatal(err)
	}
	if trees.Exists(ctx(), where) {
		t.Error("a checkout deleted from disk still exists")
	}
	// Nor is it a checkout to count against the limit or to offer the prune.
	listed, err := trees.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Errorf("list reports %v, a checkout that is not on disk", listed)
	}
}

// Git refuses to add a checkout where it still has a record of one, and refuses
// to check out a branch it believes is checked out somewhere that is gone. Both
// are what cutting a lost checkout again runs into.
func TestACheckoutThatWasDeletedCanBeCutAgainWithItsWork(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	base := t.TempDir()
	spec := task.WorktreeSpec{TaskID: "t-1", Branch: "aos/fix-it", Base: "main", Path: filepath.Join(base, "t-1")}

	first, err := trees.Create(ctx(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "WORK.md"), []byte("done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, first, "the work")
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}

	// The same place, the way a task's own path comes back.
	again, err := trees.Create(ctx(), spec)
	if err != nil {
		t.Fatalf("cutting a deleted checkout again at the same path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(again, "WORK.md")); err != nil {
		t.Fatalf("the checkout came back without the work committed on its branch: %v", err)
	}

	// And somewhere else, the way it comes back when the worktree root moved.
	if err := os.RemoveAll(again); err != nil {
		t.Fatal(err)
	}
	spec.Path = filepath.Join(t.TempDir(), "moved", "t-1")
	if _, err := trees.Create(ctx(), spec); err != nil {
		t.Fatalf("cutting a deleted checkout again elsewhere: %v", err)
	}
}

// Forgetting a lost checkout is not forgetting every one: a checkout on a
// drive that is not mounted right now is somebody's work, and `git worktree
// prune` would drop git's record of it.
func TestCuttingAgainForgetsOnlyTheLostCheckoutInTheWay(t *testing.T) {
	repo := repository(t)
	trees := gitcli.NewWorktrees(gitcli.New(), repo)
	unmounted := filepath.Join(t.TempDir(), "usb", "mine")
	git(t, repo, "worktree", "add", "-b", "mine", unmounted)
	spec := task.WorktreeSpec{TaskID: "t-1", Branch: "aos/fix-it", Base: "main", Path: filepath.Join(t.TempDir(), "t-1")}
	if _, err := trees.Create(ctx(), spec); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{unmounted, spec.Path} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := trees.Create(ctx(), spec); err != nil {
		t.Fatal(err)
	}
	if err := trees.Remove(ctx(), filepath.Join(t.TempDir(), "never-there")); err != nil {
		t.Fatal(err)
	}
	if listed := git(t, repo, "worktree", "list", "--porcelain"); !strings.Contains(listed, "refs/heads/mine") {
		t.Fatalf("git no longer knows the unmounted checkout:\n%s", listed)
	}
}
