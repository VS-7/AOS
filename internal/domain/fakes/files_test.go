package fakes_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/testsuite"
)

// TestFilesSatisfiesTheScaffolderContract is what makes the workspace tests mean
// something: they run on this fake, and the same suite runs against the real
// filesystem adapter.
func TestFilesSatisfiesTheScaffolderContract(t *testing.T) {
	testsuite.RunScaffolderContract(t, testsuite.ScaffolderContract{
		New: func(*testing.T) (testsuite.Scaffolder, string) {
			return fakes.NewFiles(), "/root"
		},
	})
}

func TestWritesCounterTracksEachPath(t *testing.T) {
	f := fakes.NewFiles()
	ctx := t.Context()

	if err := f.WriteFile(ctx, "/a/one", "x"); err != nil {
		t.Fatal(err)
	}
	before := f.Snapshot()
	if err := f.WriteFile(ctx, "/a/two", "y"); err != nil {
		t.Fatal(err)
	}
	if f.Writes["/a/one"] != before["/a/one"] {
		t.Error("writing one file moved another's counter")
	}
	if f.Writes["/a/two"] != 1 {
		t.Errorf("writes = %d", f.Writes["/a/two"])
	}
	if got := f.Paths(); len(got) != 2 || got[0] != "/a/one" || got[1] != "/a/two" {
		t.Fatalf("paths = %v", got)
	}
}

func TestWritingOverADirectoryIsRefused(t *testing.T) {
	f := fakes.NewFiles()
	ctx := t.Context()
	if _, err := f.EnsureDir(ctx, "/a/b"); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteFile(ctx, "/a/b", "x"); err == nil {
		t.Fatal("writing a file over a directory must fail")
	}
	if _, err := f.EnsureDir(ctx, "/a/b/c/../c"); err != nil {
		t.Fatalf("a path needing cleaning was rejected: %v", err)
	}
}
