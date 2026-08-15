package atomicfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// failingFile fails at one chosen step and behaves normally at every other, so
// each error path of WriteFile can be exercised in isolation.
type failingFile struct {
	*os.File
	failWrite bool
	failChmod bool
	failSync  bool
	failClose bool
}

var errInjected = errors.New("injected failure")

func (f *failingFile) Write(p []byte) (int, error) {
	if f.failWrite {
		return 0, errInjected
	}
	return f.File.Write(p)
}

func (f *failingFile) Chmod(m os.FileMode) error {
	if f.failChmod {
		return errInjected
	}
	return f.File.Chmod(m)
}

func (f *failingFile) Sync() error {
	if f.failSync {
		return errInjected
	}
	return f.File.Sync()
}

func (f *failingFile) Close() error {
	if f.failClose {
		_ = f.File.Close()
		return errInjected
	}
	return f.File.Close()
}

func withFailure(t *testing.T, apply func(*failingFile)) {
	t.Helper()
	original := createTemp
	t.Cleanup(func() { createTemp = original })
	createTemp = func(dir, pattern string) (tempFile, error) {
		real, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		ff := &failingFile{File: real}
		apply(ff)
		return ff, nil
	}
}

// TestAnInterruptedWriteLeavesThePreviousFileIntact is the property the whole
// package exists for. It runs once per failure point.
func TestAnInterruptedWriteLeavesThePreviousFileIntact(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*failingFile)
	}{
		{"write", func(f *failingFile) { f.failWrite = true }},
		{"chmod", func(f *failingFile) { f.failChmod = true }},
		{"sync", func(f *failingFile) { f.failSync = true }},
		{"close", func(f *failingFile) { f.failClose = true }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "m.md")
			if err := WriteFile(path, []byte("previous"), 0o600); err != nil {
				t.Fatal(err)
			}

			withFailure(t, c.apply)
			if err := WriteFile(path, []byte("next"), 0o600); !errors.Is(err, errInjected) {
				t.Fatalf("error = %v, want the injected failure", err)
			}

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != "previous" {
				t.Fatalf("the previous content was lost: %q", raw)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("a failed write left debris: %v", entries)
			}
		})
	}
}

func TestCreateTempFailurePropagates(t *testing.T) {
	original := createTemp
	t.Cleanup(func() { createTemp = original })
	createTemp = func(string, string) (tempFile, error) { return nil, errInjected }

	if err := WriteFile(filepath.Join(t.TempDir(), "a.md"), []byte("x"), 0o600); !errors.Is(err, errInjected) {
		t.Fatalf("error = %v", err)
	}
}

func TestRenameFailurePropagates(t *testing.T) {
	original := rename
	t.Cleanup(func() { rename = original })
	rename = func(string, string) error { return errInjected }

	dir := t.TempDir()
	if err := WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o600); !errors.Is(err, errInjected) {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the temp file survived a failed rename: %v", entries)
	}
}

func TestFsyncDirOnAMissingDirectoryReportsTheError(t *testing.T) {
	if err := fsyncDir(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected an error opening a missing directory")
	}
}
