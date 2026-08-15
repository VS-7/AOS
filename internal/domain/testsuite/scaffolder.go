package testsuite

import (
	"context"
	"path"
	"testing"
)

// Scaffolder is the port this contract exercises. It is redeclared here rather
// than imported from internal/domain/workspace so that the suite does not tie
// every implementation to the one aggregate that consumes it today.
type Scaffolder interface {
	EnsureDir(ctx context.Context, path string) (created bool, err error)
	ReadFile(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, path, content string) error
}

// ScaffolderContract describes one implementation well enough to exercise it.
type ScaffolderContract struct {
	// New returns a fresh scaffolder and an absolute root that is empty.
	New func(t *testing.T) (Scaffolder, string)
}

// RunScaffolderContract exercises the behaviour the workspace aggregate relies
// on. Two of these properties are the reason the port exists at all: the
// created flag is how the scaffold reports that it adopted an existing layout,
// and a missing file reading as empty is how the .env splice works on a
// repository that has none.
func RunScaffolderContract(t *testing.T, c ScaffolderContract) {
	t.Helper()
	ctx := context.Background()

	t.Run("EnsureDir reports creation exactly once", func(t *testing.T) {
		fs, root := c.New(t)
		dir := path.Join(root, "a")

		created, err := fs.EnsureDir(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		if !created {
			t.Fatal("the first call must report that it created the directory")
		}
		created, err = fs.EnsureDir(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		if created {
			t.Fatal("the second call must report that it created nothing")
		}
	})

	t.Run("EnsureDir creates intermediate directories", func(t *testing.T) {
		fs, root := c.New(t)
		deep := path.Join(root, "a", "b", "c")
		if _, err := fs.EnsureDir(ctx, deep); err != nil {
			t.Fatal(err)
		}
		// The parent now exists, so ensuring it must report no creation.
		created, err := fs.EnsureDir(ctx, path.Join(root, "a", "b"))
		if err != nil {
			t.Fatal(err)
		}
		if created {
			t.Fatal("an intermediate directory was not created by the deep call")
		}
	})

	t.Run("reading a missing file yields empty, not an error", func(t *testing.T) {
		fs, root := c.New(t)
		got, err := fs.ReadFile(ctx, path.Join(root, "absent.env"))
		if err != nil {
			t.Fatalf("a missing file is the normal case: %v", err)
		}
		if got != "" {
			t.Fatalf("content = %q, want empty", got)
		}
	})

	t.Run("write then read round-trips", func(t *testing.T) {
		fs, root := c.New(t)
		target := path.Join(root, "nested", "dir", "file.env")
		const content = "KEY=value\n# comment\n\ntrailing\n"

		if err := fs.WriteFile(ctx, target, content); err != nil {
			t.Fatal(err)
		}
		got, err := fs.ReadFile(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if got != content {
			t.Fatalf("content = %q, want %q", got, content)
		}
	})

	t.Run("write replaces rather than appends", func(t *testing.T) {
		fs, root := c.New(t)
		target := path.Join(root, "file.env")
		if err := fs.WriteFile(ctx, target, "first"); err != nil {
			t.Fatal(err)
		}
		if err := fs.WriteFile(ctx, target, "second"); err != nil {
			t.Fatal(err)
		}
		got, _ := fs.ReadFile(ctx, target)
		if got != "second" {
			t.Fatalf("content = %q, want the replacement", got)
		}
	})

	t.Run("an empty file is written and read back as empty", func(t *testing.T) {
		// This is the .gitkeep case: the marker's whole job is to exist.
		fs, root := c.New(t)
		target := path.Join(root, "dir", ".gitkeep")
		if err := fs.WriteFile(ctx, target, ""); err != nil {
			t.Fatal(err)
		}
		got, err := fs.ReadFile(ctx, target)
		if err != nil || got != "" {
			t.Fatalf("content = %q, err = %v", got, err)
		}
	})

	t.Run("a cancelled context is refused", func(t *testing.T) {
		fs, root := c.New(t)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := fs.EnsureDir(cancelled, path.Join(root, "x")); err == nil {
			t.Error("EnsureDir ignored a cancelled context")
		}
		if err := fs.WriteFile(cancelled, path.Join(root, "y"), ""); err == nil {
			t.Error("WriteFile ignored a cancelled context")
		}
	})
}
