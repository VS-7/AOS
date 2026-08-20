package skillfiles_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/skillfiles"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/skill"
)

func ctx() context.Context { return context.Background() }

// resolvedRoot mirrors pathx_test.go's own helper: a raw t.TempDir() on
// macOS lives under /var, itself a symlink to /private/var, and this
// adapter's own root is expected to already be resolved the way a real
// caller's workspace root is by the time it reaches here.
func resolvedRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestWritePlacesEveryFileOfTheBatch(t *testing.T) {
	root := resolvedRoot(t)
	f := skillfiles.New(root)

	files := []skill.RawFile{
		{Path: "agents/closer/AGENT.md", Content: []byte("# Closer\n")},
		{Path: "templates/note.template.md", Content: []byte("template")},
	}
	if err := f.Write(ctx(), "crm", files); err != nil {
		t.Fatal(err)
	}

	for _, want := range files {
		abs := filepath.Join(root, collections.Root, "skills", "crm", want.Path)
		got, err := os.ReadFile(abs) //nolint:gosec // test-controlled path
		if err != nil {
			t.Fatalf("reading %q: %v", abs, err)
		}
		if string(got) != string(want.Content) {
			t.Fatalf("content of %q = %q, want %q", want.Path, got, want.Content)
		}
	}
}

func TestRemoveTakesBackWhatWriteWrote(t *testing.T) {
	root := resolvedRoot(t)
	f := skillfiles.New(root)

	files := []skill.RawFile{{Path: "agents/closer/AGENT.md", Content: []byte("# Closer\n")}}
	if err := f.Write(ctx(), "crm", files); err != nil {
		t.Fatal(err)
	}

	if err := f.Remove(ctx(), "crm", "agents/closer/AGENT.md"); err != nil {
		t.Fatal(err)
	}

	abs := filepath.Join(root, collections.Root, "skills", "crm", "agents/closer/AGENT.md")
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatalf("the file survived Remove: err = %v", err)
	}
}

// TestRemoveIsIdempotent is the exact property Apply's rollback depends on:
// undoing a batch that only partially wrote must not itself fail on the
// files that were never there.
func TestRemoveIsIdempotent(t *testing.T) {
	root := resolvedRoot(t)
	f := skillfiles.New(root)

	if err := f.Remove(ctx(), "crm", "agents/closer/AGENT.md"); err != nil {
		t.Fatalf("removing something already gone must succeed: %v", err)
	}
	// And a second time, after the skill's directory itself does not exist
	// either.
	if err := f.Remove(ctx(), "never-installed", "anything.md"); err != nil {
		t.Fatalf("removing from a skill that was never installed must succeed: %v", err)
	}
}

// TestWriteRefusesAPathThatEscapesTheSkillsOwnDirectory is the boundary this
// adapter exists to hold: a package's declared path is data the package
// itself supplies, and "../../../.env" written by one skill must not reach
// another skill's directory, let alone the workspace root.
func TestWriteRefusesAPathThatEscapesTheSkillsOwnDirectory(t *testing.T) {
	root := resolvedRoot(t)
	f := skillfiles.New(root)

	err := f.Write(ctx(), "crm", []skill.RawFile{
		{Path: "../../../etc/passwd", Content: []byte("nope")},
	})
	if err == nil {
		t.Fatal("a path escaping the skill's own directory was written")
	}
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err = %v, want an *apperr.Error", err)
	}
	if e.Code != "AOS_SKILLFILES_PATH_OUTSIDE" {
		t.Fatalf("code = %q, want AOS_SKILLFILES_PATH_OUTSIDE", e.Code)
	}
}

// TestRemoveRefusesAPathThatEscapesTheSkillsOwnDirectory: the same boundary,
// on the way back out — a rollback must not become a second way to reach
// outside the skill's directory.
func TestRemoveRefusesAPathThatEscapesTheSkillsOwnDirectory(t *testing.T) {
	root := resolvedRoot(t)
	f := skillfiles.New(root)

	err := f.Remove(ctx(), "crm", "../../../etc/passwd")
	if err == nil {
		t.Fatal("a path escaping the skill's own directory was accepted by Remove")
	}
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err = %v, want an *apperr.Error", err)
	}
	if e.Code != "AOS_SKILLFILES_PATH_OUTSIDE" {
		t.Fatalf("code = %q, want AOS_SKILLFILES_PATH_OUTSIDE", e.Code)
	}
}

func TestWriteStopsAtTheFirstFailureAndLeavesLaterFilesUnwritten(t *testing.T) {
	root := resolvedRoot(t)
	f := skillfiles.New(root)

	err := f.Write(ctx(), "crm", []skill.RawFile{
		{Path: "ok.md", Content: []byte("fine")},
		{Path: "../escape.md", Content: []byte("nope")},
		{Path: "never-reached.md", Content: []byte("nope")},
	})
	if err == nil {
		t.Fatal("a batch containing an escaping path reported success")
	}
	if _, statErr := os.Stat(filepath.Join(root, collections.Root, "skills", "crm", "never-reached.md")); !os.IsNotExist(statErr) {
		t.Fatal("a file after the failing one was still written")
	}
}
