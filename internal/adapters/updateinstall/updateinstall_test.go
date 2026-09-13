package updateinstall_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/updateinstall"
	"github.com/OWNER/aos/internal/domain/update"
)

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func digestOf(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func read(t *testing.T, path string) string {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

func TestStageWritesTheFileAndDigestReadsItBack(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	path, err := i.Stage(ctx, "aos", []byte("new contents"))
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(stageDir, exe("aos")) || read(t, path) != "new contents" {
		t.Fatalf("staged at %q", path)
	}
	sum := sha256.Sum256([]byte("new contents"))
	got, err := i.Digest(ctx, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest = %s", got)
	}

	if err := i.Discard(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Discard should remove the staged copy")
	}
	if _, err := i.Digest(ctx, "aos"); err == nil {
		t.Fatal("a discarded binary has no digest")
	}
	if err := i.Discard(ctx); err != nil {
		t.Fatalf("discarding nothing is not an error, got %v", err)
	}
}

func TestTargetSaysWhetherTheBinaryIsInstalled(t *testing.T) {
	binDir := t.TempDir()
	i := updateinstall.New(t.TempDir(), binDir)
	ctx := context.Background()

	path, ok, err := i.Target(ctx, "aosd")
	if err != nil || ok || path != filepath.Join(binDir, exe("aosd")) {
		t.Fatalf("Target before install = %q, %v, %v", path, ok, err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := i.Target(ctx, "aosd"); err != nil || !ok {
		t.Fatalf("Target after install = %v, %v", ok, err)
	}
}

func TestSwapInReplacesRollbackRestoresAndCommitCleansUp(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	target := filepath.Join(binDir, exe("aos"))
	if err := os.WriteFile(target, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}
	staged, err := i.Stage(ctx, "aos", []byte("new version"))
	if err != nil {
		t.Fatal(err)
	}

	if err := i.SwapIn(ctx, "aos", digestOf("new version")); err != nil {
		t.Fatal(err)
	}
	if read(t, target) != "new version" {
		t.Fatal("SwapIn did not put the new version in place")
	}
	// The staged copy survives the swap: a rolled-back update can be retried
	// without downloading it again.
	if read(t, staged) != "new version" {
		t.Fatal("SwapIn consumed the staged copy")
	}

	if err := i.Rollback(ctx, "aos"); err != nil {
		t.Fatal(err)
	}
	if read(t, target) != "old version" {
		t.Fatal("Rollback did not restore the old version")
	}

	if err := i.SwapIn(ctx, "aos", digestOf("new version")); err != nil {
		t.Fatal(err)
	}
	if err := i.Commit(ctx, "aos"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target + ".prev"); !os.IsNotExist(err) {
		t.Fatal("Commit should remove the backup")
	}
	if err := i.Commit(ctx, "aos"); err != nil {
		t.Fatalf("committing twice is not an error, got %v", err)
	}
	if read(t, target) != "new version" {
		t.Fatal("Commit must leave the new version in place")
	}
}

// Apply verifies the staged copy and then waits — for work in flight, for
// minutes — before it swaps. SwapIn hashes the bytes it actually copies, and
// a copy that is not the verified one replaces nothing.
func TestSwapInRefusesAStagedCopyThatIsNotTheVerifiedOne(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	target := filepath.Join(binDir, exe("aosd"))
	if err := os.WriteFile(target, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Stage(ctx, "aosd", []byte("verified version")); err != nil {
		t.Fatal(err)
	}
	verified := digestOf("verified version")
	if _, err := i.Stage(ctx, "aosd", []byte("replaced after verification")); err != nil {
		t.Fatal(err)
	}

	for _, digest := range []string{verified, ""} {
		err := i.SwapIn(ctx, "aosd", digest)
		if err == nil {
			t.Fatalf("SwapIn with digest %q should refuse a copy that does not match it", digest)
		}
		if digest != "" && !errors.Is(err, update.ErrStagedChanged) {
			t.Fatalf("the refusal should say the staged copy changed, got %v", err)
		}
		if read(t, target) != "old version" {
			t.Fatal("the live binary must be untouched")
		}
		for _, leftover := range []string{target + ".new", target + ".prev"} {
			if _, err := os.Stat(leftover); !os.IsNotExist(err) {
				t.Fatalf("%s should not be left behind", leftover)
			}
		}
	}

	if err := i.SwapIn(ctx, "aosd", strings.ToUpper(digestOf("replaced after verification"))); err != nil {
		t.Fatalf("a digest is a digest in either case, got %v", err)
	}
}

// An update replaces what is installed and adds nothing. Creating an aos
// inside a bundle that never carried one is what broke the bundle's seal.
func TestSwapInDoesNotAddABinaryThatIsNotInstalled(t *testing.T) {
	stageDir, binDir := t.TempDir(), t.TempDir()
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	if _, err := i.Stage(ctx, "aos", []byte("first install")); err != nil {
		t.Fatal(err)
	}
	if err := i.SwapIn(ctx, "aos", digestOf("first install")); err == nil {
		t.Fatal("expected SwapIn to refuse a binary that is not installed")
	}
	if _, err := os.Stat(filepath.Join(binDir, exe("aos"))); !os.IsNotExist(err) {
		t.Fatal("nothing should have been created")
	}
}

func TestSwapInWithNothingStagedLeavesTheBinaryAlone(t *testing.T) {
	binDir := t.TempDir()
	i := updateinstall.New(t.TempDir(), binDir)
	target := filepath.Join(binDir, exe("aosd"))
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := i.SwapIn(context.Background(), "aosd", digestOf("")); err == nil {
		t.Fatal("expected a failure with nothing staged")
	}
	if read(t, target) != "old" {
		t.Fatal("the live binary must be untouched")
	}
	if _, err := os.Stat(target + ".prev"); !os.IsNotExist(err) {
		t.Fatal("no backup should be left for a swap that never happened")
	}
}

// Rollback without a prior SwapIn for this target is a no-op, not an error
// — see the port's own doc comment on why: Apply rolls back every binary it
// may have swapped, whether or not this one got that far.
func TestRollbackWithNoBackupIsANoOp(t *testing.T) {
	binDir := t.TempDir()
	i := updateinstall.New(t.TempDir(), binDir)
	target := filepath.Join(binDir, exe("aos"))
	if err := os.WriteFile(target, []byte("never touched"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := i.Rollback(context.Background(), "aos"); err != nil {
		t.Fatal(err)
	}
	if read(t, target) != "never touched" {
		t.Fatal("a no-op rollback should not touch the file")
	}
}

// Every method resolves paths from a name it knows, and a name it does not
// know reaches no file: "../planted" once put a caller's file beside the
// install directory through update_apply.
func TestEveryMethodRefusesANameThatIsNotAManagedBinary(t *testing.T) {
	root := t.TempDir()
	stageDir, binDir := filepath.Join(root, "stage"), filepath.Join(root, "bin")
	i := updateinstall.New(stageDir, binDir)
	ctx := context.Background()

	for _, name := range []string{"../planted", "aosd/../../planted", "", "sh"} {
		if _, err := i.Stage(ctx, name, []byte("unsigned")); err == nil {
			t.Errorf("Stage(%q) should be refused", name)
		}
		if _, err := i.Digest(ctx, name); err == nil {
			t.Errorf("Digest(%q) should be refused", name)
		}
		if _, _, err := i.Target(ctx, name); err == nil {
			t.Errorf("Target(%q) should be refused", name)
		}
		if err := i.SwapIn(ctx, name, digestOf("unsigned")); err == nil {
			t.Errorf("SwapIn(%q) should be refused", name)
		}
		if err := i.Rollback(ctx, name); err == nil {
			t.Errorf("Rollback(%q) should be refused", name)
		}
		if err := i.Commit(ctx, name); err == nil {
			t.Errorf("Commit(%q) should be refused", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "planted")); !os.IsNotExist(err) {
		t.Fatal("a file was planted outside the install directories")
	}
}

func TestABundleCannotBeUpdatedInPlace(t *testing.T) {
	ctx := context.Background()
	for _, dir := range []string{
		"/Applications/AOS.app/Contents/MacOS",
		"/Users/me/Applications/AOS.app/Contents/Mac",
	} {
		if got := updateinstall.New(t.TempDir(), dir).Reinstall(ctx); got != update.ReinstallBundle {
			t.Errorf("Reinstall(%q) = %q, want a bundle", dir, got)
		}
	}
	// A directory merely named like one is not a bundle.
	root := t.TempDir()
	for _, dir := range []string{"AOS.app.backup/bin", "home/AOS"} {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := updateinstall.New(t.TempDir(), path).Reinstall(ctx); got != "" {
			t.Errorf("Reinstall(%q) = %q, want an in-place install", dir, got)
		}
	}
}

// A directory this account cannot write is where the swap could only fail
// halfway: an AppImage's read-only mount, Program Files without elevation,
// /usr/bin for a package manager. It was offered a terminal command anyway.
func TestADirectoryThisAccountCannotWriteIsReinstalled(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not stop this account here")
	}
	ctx := context.Background()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "aosd"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })

	if got := updateinstall.New(t.TempDir(), bin).Reinstall(ctx); got != update.ReinstallReadOnly {
		t.Fatalf("Reinstall = %q, want read-only", got)
	}
	entries, _ := os.ReadDir(bin)
	if len(entries) != 1 {
		t.Fatalf("asking left something behind: %v", entries)
	}
	if got := updateinstall.New(t.TempDir(), filepath.Join(bin, "missing")).Reinstall(ctx); got != update.ReinstallReadOnly {
		t.Fatalf("a directory that is not there cannot be written either, got %q", got)
	}

	_ = os.Chmod(bin, 0o755)
	if got := updateinstall.New(t.TempDir(), bin).Reinstall(ctx); got != "" {
		t.Fatalf("a writable directory is updated in place, got %q", got)
	}
	entries, _ = os.ReadDir(bin)
	if len(entries) != 1 {
		t.Fatalf("the probe was not removed: %v", entries)
	}
}

func TestStoreKeepsTheRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update", "state.json")
	s := updateinstall.NewStore(path)
	ctx := context.Background()

	empty, err := s.Load(ctx)
	if err != nil || empty.LastCheck != nil || empty.Staged != nil {
		t.Fatalf("a missing file is an empty record, got %+v, %v", empty, err)
	}

	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	want := update.Record{
		LastCheck: &update.LastCheck{At: at, Channel: update.ChannelStable, State: update.StateAvailable, Latest: "v0.16.0"},
		Staged:    &update.StagedRelease{Version: "v0.16.0", Files: map[string]string{"aosd": "aosd_v0.16.0_linux_amd64"}},
	}
	if err := s.Update(ctx, func(r *update.Record) error { *r = want; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, func(r *update.Record) error { r.LastCheck = nil; return errors.New("changed my mind") }); err == nil {
		t.Fatal("a change that fails should fail the update")
	}
	got, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastCheck == nil || !got.LastCheck.At.Equal(at) || got.Staged == nil || got.Staged.Files["aosd"] != "aosd_v0.16.0_linux_amd64" {
		t.Fatalf("round trip lost the record: %+v", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("record mode = %v, want 0600", info.Mode().Perm())
		}
	}

	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx); err == nil {
		t.Fatal("a corrupt record should be reported, not read as empty")
	}
}

// Every workspace's update service in the daemon, and `aosd update apply` in a
// terminal, keep one record. Reading it, changing it and writing it back from
// two of them at once used to keep only one of the two changes.
func TestStoreUpdatesFromSeveralWritersLoseNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update", "state.json")
	ctx := context.Background()
	const writers, each = 4, 25

	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for w := 0; w < writers; w++ {
		// A Store of its own each, as separate services and processes have.
		s := updateinstall.NewStore(path)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < each; n++ {
				errs <- s.Update(ctx, func(r *update.Record) error {
					count := 0
					if r.LastCheck != nil {
						count, _ = strconv.Atoi(r.LastCheck.Latest)
					}
					r.LastCheck = &update.LastCheck{Latest: strconv.Itoa(count + 1)}
					return nil
				})
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := updateinstall.NewStore(path).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastCheck == nil || got.LastCheck.Latest != strconv.Itoa(writers*each) {
		t.Fatalf("%d updates, and the record counted %+v", writers*each, got.LastCheck)
	}
}

// The helper process for TestTheLockIsOneHolderAndOutlivesNoProcess: it takes
// the lock, says so, and holds it until it is killed.
func TestHelperHoldsTheUpdateLock(t *testing.T) {
	path := os.Getenv("AOS_TEST_HOLD_UPDATE_LOCK")
	if path == "" {
		t.Skip("run by TestTheLockIsOneHolderAndOutlivesNoProcess")
	}
	_, ok, err := updateinstall.NewLock(path).TryLock(context.Background())
	if err != nil || !ok {
		t.Fatalf("the helper could not take the lock: %v", err)
	}
	_, _ = os.Stdout.WriteString("held\n")
	time.Sleep(time.Minute)
}

// One Download or Apply at a time, across processes — and a process that is
// killed holding the lock does not leave the installation locked for good.
func TestTheLockIsOneHolderAndOutlivesNoProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update", "update.lock")
	ctx := context.Background()

	unlock, ok, err := updateinstall.NewLock(path).TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("the first holder should get the lock: %v", err)
	}
	if _, ok, err := updateinstall.NewLock(path).TryLock(ctx); err != nil || ok {
		t.Fatalf("a second holder got the lock (ok=%v, err=%v)", ok, err)
	}
	unlock()
	again, ok, err := updateinstall.NewLock(path).TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("a released lock should be free: %v", err)
	}
	again()

	helper := exec.Command(os.Args[0], "-test.run=^TestHelperHoldsTheUpdateLock$")
	helper.Env = append(os.Environ(), "AOS_TEST_HOLD_UPDATE_LOCK="+path)
	out, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill(); _ = helper.Wait() })
	line := make([]byte, 5)
	if _, err := io.ReadFull(out, line); err != nil || string(line) != "held\n" {
		t.Fatalf("the helper did not take the lock: %q, %v", line, err)
	}
	if _, ok, _ := updateinstall.NewLock(path).TryLock(ctx); ok {
		t.Fatal("another process holds the lock, and it was taken anyway")
	}
	_ = helper.Process.Kill()
	_ = helper.Wait()
	freed, ok, err := updateinstall.NewLock(path).TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("a killed holder should leave the lock free: %v", err)
	}
	freed()
}

// Tidy removes what an install leaves only when it could not finish its own
// clean-up — a backup Commit could not remove, a copy a crash left before its
// rename — for the binaries an update manages, and nothing else.
func TestTidyRemovesOnlyWhatAnInstallLeft(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{exe("aosd"), exe("aosd") + ".prev", exe("aos") + ".new", "notes.prev", "aosd-other.prev"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := updateinstall.New(t.TempDir(), bin).Tidy(context.Background()); err != nil {
		t.Fatal(err)
	}
	var left []string
	entries, _ := os.ReadDir(bin)
	for _, e := range entries {
		left = append(left, e.Name())
	}
	want := []string{"aosd-other.prev", exe("aosd"), "notes.prev"}
	sort.Strings(want)
	if strings.Join(left, ",") != strings.Join(want, ",") {
		t.Fatalf("left %v, want %v", left, want)
	}
}
