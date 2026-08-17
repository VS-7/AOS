package pathx_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OWNER/aos/internal/core/pathx"
)

// resolvedRoot mirrors what a real caller does once at construction time: a
// raw t.TempDir() on macOS lives under /var, itself a symlink to /private/var,
// and ResolveInside expects root to already be resolved (see its doc comment).
func resolvedRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResolveInsideAcceptsAPathInsideTheRoot(t *testing.T) {
	root := resolvedRoot(t)
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := pathx.ResolveInside(root, "hello.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(root, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveInsideRejectsATraversal(t *testing.T) {
	root := resolvedRoot(t)

	cases := []string{
		"../../etc/passwd",
		"./sub/../../../etc/passwd",
	}
	for _, p := range cases {
		if _, err := pathx.ResolveInside(root, p); !errors.Is(err, pathx.ErrOutside) {
			t.Errorf("%q: got %v, want ErrOutside", p, err)
		}
	}
}

func TestResolveInsideRejectsAnAbsolutePathElsewhere(t *testing.T) {
	root := resolvedRoot(t)
	if _, err := pathx.ResolveInside(root, "/etc/passwd"); !errors.Is(err, pathx.ErrOutside) {
		t.Fatalf("got %v, want ErrOutside", err)
	}
}

func TestResolveInsideAllowsANewFileNotYetOnDisk(t *testing.T) {
	root := resolvedRoot(t)
	got, err := pathx.ResolveInside(root, "new.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(got) != root {
		t.Fatalf("got %q, want a child of %q", got, root)
	}
}

func TestResolveInsideFollowsASymlinkOutOfTheRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need a privilege on Windows that a test should not assume")
	}
	root := resolvedRoot(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	if _, err := pathx.ResolveInside(root, "escape/secret.txt"); !errors.Is(err, pathx.ErrOutside) {
		t.Fatalf("got %v, want ErrOutside", err)
	}
}

func TestContainsComparesTheRelativePathNotAStringPrefix(t *testing.T) {
	if pathx.Contains("/a/b", "/a/bc") {
		t.Fatal("/a/bc must not be considered inside /a/b")
	}
	if !pathx.Contains("/a/b", "/a/b") {
		t.Fatal("a root must be considered inside itself")
	}
	if !pathx.Contains("/a/b", "/a/b/c") {
		t.Fatal("a child must be considered inside its root")
	}
}
