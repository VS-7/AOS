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
