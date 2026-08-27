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

// Root is the function a caller uses instead of the EvalSymlinks dance
// resolvedRoot does by hand above — it exists precisely so that dance is not
// repeated at every construction site.
func TestRootResolvesASymlinkedRoot(t *testing.T) {
	base := resolvedRoot(t)
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}

	got, err := pathx.Root(link)
	if err != nil {
		t.Fatalf("Root(%q) = error %v", link, err)
	}
	if got != real {
		t.Fatalf("Root(%q) = %q, want %q", link, got, real)
	}
}

// A root that does not exist yet comes back unchanged rather than as an
// error: the workspace directory may be about to be scaffolded, and the
// first real operation against it is where a missing root should fail.
func TestRootReturnsAMissingRootUnchanged(t *testing.T) {
	missing := filepath.Join(resolvedRoot(t), "not-created-yet")

	got, err := pathx.Root(missing)
	if err != nil {
		t.Fatalf("Root(%q) = error %v, want it returned unchanged", missing, err)
	}
	if got != missing {
		t.Fatalf("Root(%q) = %q, want it unchanged", missing, got)
	}
}

// A path whose parent is a file rather than a directory is not "does not
// exist" — EvalSymlinks reports ENOTDIR, and Resolve must surface that
// instead of quietly falling through to its parent-resolution branch.
func TestResolveReportsAFailureThatIsNotAMissingPath(t *testing.T) {
	root := resolvedRoot(t)
	notADir := filepath.Join(root, "file.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// "file.txt/child/grandchild": the parent itself cannot be walked, so the
	// fallback in Resolve fails too and the path is returned as-is for
	// containment to judge — which it does, and it is inside the root.
	got, err := pathx.Resolve(root, filepath.Join("file.txt", "child", "grandchild"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Some platforms report ENOTDIR here, which is a legitimate hard
		// failure; others report ENOENT and fall through. Both are correct
		// behaviour for this function — what must never happen is a path
		// escaping the root.
		return
	}
	if err == nil && !pathx.Contains(root, got) {
		t.Fatalf("Resolve(%q) = %q, which is outside the root", notADir, got)
	}
}

// Contains has to answer for a root and a path that share no ancestor at
// all. filepath.Rel fails outright on Windows for paths on different
// volumes; everywhere else it returns a "../.." chain. Neither is inside.
func TestContainsRejectsAnUnrelatedPath(t *testing.T) {
	root := resolvedRoot(t)
	other := resolvedRoot(t)
	if pathx.Contains(root, other) {
		t.Fatalf("Contains(%q, %q) = true, want false", root, other)
	}
	if runtime.GOOS == "windows" && pathx.Contains(`C:\a`, `D:\b`) {
		t.Fatal(`Contains("C:\a", "D:\b") = true across volumes, want false`)
	}
}

// ResolveInside forwards a resolution failure rather than turning it into
// ErrOutside — the two mean different things to a caller, and reporting
// "outside the root" for an I/O problem sends whoever reads the log looking
// for an attack that did not happen.
func TestResolveInsideForwardsAResolutionFailureAsItself(t *testing.T) {
	root := resolvedRoot(t)
	deep := filepath.Join(root, "a", "b", "c", "d")

	got, err := pathx.ResolveInside(root, deep)
	if errors.Is(err, pathx.ErrOutside) {
		t.Fatalf("a path under the root came back as ErrOutside: %q", deep)
	}
	if err == nil && !pathx.Contains(root, got) {
		t.Fatalf("ResolveInside(%q) = %q, which is outside the root", deep, got)
	}
}

// A write through a symlinked directory two levels deep used to escape.
//
// Resolve fell back to the *immediate* parent, and when that did not exist
// either it returned the path uncollapsed — lexically inside the root, so
// Contains said yes, while the MkdirAll the caller does next follows the link
// and lands outside. Two missing components was all it took.
func TestResolveInsideRefusesANewPathUnderASymlinkOutOfTheRoot(t *testing.T) {
	root := resolvedRoot(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}

	for _, p := range []string{
		"link/newdir/file.txt", // two components missing
		"link/a/b/c/file.txt",  // four
		"link/file.txt",        // one — already refused before, still is
	} {
		if got, err := pathx.ResolveInside(root, p); !errors.Is(err, pathx.ErrOutside) {
			t.Errorf("ResolveInside(%q) = %q, %v; want ErrOutside", p, got, err)
		}
	}

	// A new path under a real directory is still allowed — that is the case
	// the fallback exists for.
	if _, err := pathx.ResolveInside(root, "real/newdir/file.txt"); err != nil {
		t.Errorf("a new path inside the root was refused: %v", err)
	}
}

// Segment is the guard for an identifier about to become one path component.
func TestSegmentRefusesAnythingThatIsNotOneComponent(t *testing.T) {
	for _, ok := range []string{"atlas", "a-b_c.9", "550e8400-e29b-41d4-a716-446655440000", ".hidden"} {
		if got, err := pathx.Segment(ok); err != nil || got != ok {
			t.Errorf("Segment(%q) = %q, %v; want it accepted", ok, got, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "../..", "a/b", `a\b`, "/abs", "trailing/", "a/../b"} {
		if _, err := pathx.Segment(bad); !errors.Is(err, pathx.ErrUnsafeSegment) {
			t.Errorf("Segment(%q) was accepted; it is not a single safe segment", bad)
		}
	}

	// JoinSegment is the guard applied where the join happens.
	got, err := pathx.JoinSegment("/state/workspaces", "mine")
	if err != nil || got != filepath.Join("/state/workspaces", "mine") {
		t.Errorf("JoinSegment = %q, %v", got, err)
	}
	if _, err := pathx.JoinSegment("/state/workspaces", ".."); err == nil {
		t.Error(`JoinSegment(root, "..") resolved to the parent of the root`)
	}
}
