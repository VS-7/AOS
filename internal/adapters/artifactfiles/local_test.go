package artifactfiles_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/artifactfiles"
	"github.com/OWNER/aos/internal/core/collections"
)

func TestEnsureScaffoldsWhenTheEntrypointIsMissing(t *testing.T) {
	root := t.TempDir()
	f := artifactfiles.New(root)

	got, err := f.Ensure(context.Background(), "dash", "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if got != "index.html" {
		t.Fatalf("got %q", got)
	}
	full := filepath.Join(root, collections.Root, "artifacts", "dash", "index.html")
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("scaffold not written: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("scaffold is empty")
	}
}

func TestEnsureDoesNotOverwriteAnExistingEntrypoint(t *testing.T) {
	root := t.TempDir()
	f := artifactfiles.New(root)
	dir := filepath.Join(root, collections.Root, "artifacts", "dash")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := "<h1>already here</h1>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Ensure(context.Background(), "dash", "index.html"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("existing entrypoint was overwritten: %q", got)
	}
}

func TestEnsureRefusesAnEntrypointThatEscapesTheArtifactDirectory(t *testing.T) {
	root := t.TempDir()
	f := artifactfiles.New(root)

	if _, err := f.Ensure(context.Background(), "dash", "../../../etc/passwd"); err == nil {
		t.Fatal("a path-traversal entrypoint was accepted")
	}
}

func TestRemoveDeletesTheArtifactDirectoryAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	f := artifactfiles.New(root)
	if _, err := f.Ensure(context.Background(), "dash", "index.html"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, collections.Root, "artifacts", "dash")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := f.Remove(context.Background(), "dash"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("directory still exists after Remove: %v", err)
	}

	// Idempotent: removing what is already gone succeeds.
	if err := f.Remove(context.Background(), "dash"); err != nil {
		t.Fatalf("second Remove failed: %v", err)
	}
}

// The artifact id is a directory name, and it is checked as one.
//
// `filepath.Join` cleans, so an id of ".." resolved to the workspace's whole
// .aos directory — agents, tasks, chats, memories, collections, every record
// the workspace has — and Remove took os.RemoveAll to it. No symlink, nothing
// for pathx.Root or ResolveInside to catch: the path was ordinary, it was
// simply not the artifact's.
func TestAnIdThatIsNotOneSegmentIsRefusedRatherThanResolved(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".aos")
	if err := os.MkdirAll(filepath.Join(state, "agents", "atlas"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "agents", "atlas", "AGENT.md"), []byte("# Atlas"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(state, "artifacts", "real"), 0o755); err != nil {
		t.Fatal(err)
	}

	files := artifactfiles.New(root)
	for _, id := range []string{"..", ".", "../..", "a/b", "/etc"} {
		if err := files.Remove(context.Background(), id); err == nil {
			t.Errorf("Remove(%q) was carried out", id)
		}
		if _, err := files.Resolve(id, "index.html"); err == nil {
			t.Errorf("Resolve(%q) resolved to something", id)
		}
		if _, err := files.Ensure(context.Background(), id, "index.html"); err == nil {
			t.Errorf("Ensure(%q) wrote something", id)
		}
	}

	if _, err := os.Stat(filepath.Join(state, "agents", "atlas", "AGENT.md")); err != nil {
		t.Fatalf("the workspace's own records were deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "artifacts", "real")); err != nil {
		t.Fatalf("another artifact was deleted: %v", err)
	}

	// A real id still works, which is the other half of the guard.
	if _, err := files.Ensure(context.Background(), "real", "index.html"); err != nil {
		t.Fatalf("a legitimate artifact was refused: %v", err)
	}
}
