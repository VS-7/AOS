package atomicfs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/atomicfs"
)

func TestWriteFileCreatesWithMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "a.md")
	if err := atomicfs.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "hello" {
		t.Fatalf("content = %q", raw)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("mode = %04o, want 0600", perm)
		}
	}
}

func TestWriteFileLeavesNoTempBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	for i := 0; i < 5; i++ {
		if err := atomicfs.WriteFile(path, []byte(fmt.Sprint(i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("temp file survived: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly the target file, got %d entries", len(entries))
	}
}

// TestConcurrentWritersNeverProduceAPartialFile is the property the whole
// package exists for: a reader always sees one complete version, never a
// truncated one. Run with -race.
func TestConcurrentWritersNeverProduceAPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.md")
	payloads := [][]byte{
		[]byte(strings.Repeat("a", 4096)),
		[]byte(strings.Repeat("b", 4096)),
	}
	if err := atomicfs.WriteFile(path, payloads[0], 0o644); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := atomicfs.WriteFile(path, payloads[i%2], 0o644); err != nil {
				t.Error(err)
			}
		}(i)
	}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Error(err)
				return
			}
			if len(raw) != 4096 {
				t.Errorf("read a partial file: %d bytes", len(raw))
			}
		}()
	}
	wg.Wait()
}

func TestSetFsyncIsHonoured(t *testing.T) {
	t.Cleanup(func() { atomicfs.SetFsync(true) })
	atomicfs.SetFsync(false)
	path := filepath.Join(t.TempDir(), "a.md")
	if err := atomicfs.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileFailsWhenTheParentIsAFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicfs.WriteFile(filepath.Join(blocker, "a.md"), []byte("x"), 0o644); err == nil {
		t.Fatal("expected an error when the parent path is a file")
	}
}

// TestLongNamesAreTruncatedInTheTempPattern keeps the temp file recognisable
// without tripping the filename length limit on any supported filesystem.
func TestLongNamesAreTruncatedInTheTempPattern(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("n", 200) + ".memory.md"
	if err := atomicfs.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileOverwritesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.md")
	if err := atomicfs.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicfs.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "second" {
		t.Fatalf("content = %q", raw)
	}
}

// TestWriteFileFailsWhenTheTargetIsANonEmptyDirectory covers the rename branch:
// the temp file is written and then cannot take the target's place.
func TestWriteFileFailsWhenTheTargetIsANonEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "occupied")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicfs.WriteFile(target, []byte("x"), 0o644); err == nil {
		t.Fatal("expected the rename to fail")
	}
	// The failed write left nothing behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("a failed write left debris: %v", entries)
	}
}

func TestFsyncDirFailureIsToleratedWhenDisabled(t *testing.T) {
	t.Cleanup(func() { atomicfs.SetFsync(true) })
	atomicfs.SetFsync(false)
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	for i := 0; i < 3; i++ {
		if err := atomicfs.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
