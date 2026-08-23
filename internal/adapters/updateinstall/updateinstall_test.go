package updateinstall_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OWNER/aos/internal/adapters/updateinstall"
)

func TestStageWritesTheFileAndPathOfFindsTheTarget(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	path, err := i.Stage(ctx, "aos", []byte("new contents"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new contents" {
		t.Fatalf("staged file contents = %q", got)
	}

	target, err := i.PathOf(ctx, "aos")
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := "aos"
	if runtime.GOOS == "windows" {
		wantSuffix = "aos.exe"
	}
	if filepath.Base(target) != wantSuffix {
		t.Fatalf("PathOf = %q, want a path ending in %q", target, wantSuffix)
	}
}

func TestSwapInReplacesAnExistingBinaryAndRollbackRestoresIt(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	target, err := i.PathOf(ctx, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}

	staged, err := i.Stage(ctx, "aos", []byte("new version"))
	if err != nil {
		t.Fatal(err)
	}
	if err := i.SwapIn(ctx, staged, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new version" {
		t.Fatalf("after SwapIn, target = %q", got)
	}

	if err := i.Rollback(ctx, target); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old version" {
		t.Fatalf("after Rollback, target = %q, want the old version restored", got)
	}
}

// A first install has nothing to back up — SwapIn must not fail just
// because there was no previous binary at target.
func TestSwapInWithNoExistingBinaryStillWorks(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	target, err := i.PathOf(ctx, "aos")
	if err != nil {
		t.Fatal(err)
	}
	staged, err := i.Stage(ctx, "aos", []byte("first install"))
	if err != nil {
		t.Fatal(err)
	}
	if err := i.SwapIn(ctx, staged, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first install" {
		t.Fatalf("target = %q", got)
	}
}

// Rollback without a prior SwapIn for this target is a no-op, not an error
// — see the port's own doc comment on why: Apply rolls back every binary it
// may have swapped, whether or not this one got that far.
func TestRollbackWithNoBackupIsANoOp(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	target, err := i.PathOf(ctx, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("never touched"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := i.Rollback(ctx, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "never touched" {
		t.Fatalf("a no-op rollback should not touch the file, got %q", got)
	}
}
