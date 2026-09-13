package osfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/osfile"
)

// CopyFile is the file domain's guarantee that a paste never replaces what is
// already at its destination; the exclusive open is what keeps that true
// between the domain's check and the write.
func TestCopyFileCopiesAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.bin")
	dst := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(src, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	fs := osfile.New()

	if err := fs.CopyFile(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst) //nolint:gosec // a path this test built under t.TempDir
	if err != nil || string(got) != string([]byte{0, 1, 2, 3}) {
		t.Fatalf("copy = %v, %v", got, err)
	}

	if err := os.WriteFile(dst, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fs.CopyFile(t.Context(), src, dst); err == nil {
		t.Fatal("CopyFile replaced an existing destination")
	}
	if got, _ := os.ReadFile(dst); string(got) != "kept" { //nolint:gosec // as above
		t.Fatalf("the refused copy changed the destination: %q", got)
	}
}
