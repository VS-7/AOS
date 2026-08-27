package fsworkspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fsworkspace"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/testsuite"
	"github.com/OWNER/aos/internal/domain/workspace"
)

func ctx() context.Context { return context.Background() }

// TestFilesSatisfiesTheScaffolderContract runs the same suite as the fake the
// domain tests use. That is the only thing that makes those tests evidence.
func TestFilesSatisfiesTheScaffolderContract(t *testing.T) {
	testsuite.RunScaffolderContract(t, testsuite.ScaffolderContract{
		New: func(t *testing.T) (testsuite.Scaffolder, string) {
			return fsworkspace.NewFiles(), t.TempDir()
		},
	})
}

func sample(id string) *workspace.Workspace {
	return &workspace.Workspace{
		ID:        id,
		Name:      "Project " + id,
		Path:      "/repo/" + id,
		Color:     "#123456",
		Tasks:     workspace.DefaultTaskTypes,
		Labels:    []workspace.Label{{ID: "ui", Label: "UI", Icon: "Palette", Color: "#ec4899"}},
		Worktrees: workspace.DefaultWorktrees(),
		Git:       workspace.DefaultGit(),
		Members:   []workspace.Member{{UserID: "u1", Role: "owner", AddedAt: time.Unix(0, 0).UTC()}},
		Domains:   map[string]workspace.Domain{"example.test": {ArtifactID: "a1", WorkspaceID: id}},
		CreatedAt: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
	}
}

func TestStoreRoundTripsEveryField(t *testing.T) {
	s := fsworkspace.NewStore(t.TempDir())
	want := sample("alpha")
	if err := s.Save(ctx(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.Path != want.Path || got.Color != want.Color {
		t.Fatalf("identity = %+v", got)
	}
	if len(got.Tasks) != len(want.Tasks) || got.Tasks[1].ID != "bug" {
		t.Errorf("tasks = %+v", got.Tasks)
	}
	if len(got.Labels) != 1 || got.Labels[0].Icon != "Palette" {
		t.Errorf("labels = %+v", got.Labels)
	}
	if len(got.Members) != 1 || got.Members[0].Role != "owner" {
		t.Errorf("members = %+v", got.Members)
	}
	if d, ok := got.Domains["example.test"]; !ok || d.ArtifactID != "a1" {
		t.Errorf("domains = %+v", got.Domains)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("createdAt = %v", got.CreatedAt)
	}
}

func TestGetOfAMissingWorkspaceIsNotFound(t *testing.T) {
	s := fsworkspace.NewStore(t.TempDir())
	_, err := s.Get(ctx(), "ghost")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestListOfAnAbsentDirectoryIsEmpty(t *testing.T) {
	s := fsworkspace.NewStore(filepath.Join(t.TempDir(), "never-created"))
	got, err := s.List(ctx())
	if err != nil {
		t.Fatalf("a fresh installation has no workspaces, which is not an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("list = %+v", got)
	}
}

// TestOneCorruptRecordDoesNotHideTheRest: the person needs to reach their other
// workspaces in order to fix the broken one.
func TestOneCorruptRecordDoesNotHideTheRest(t *testing.T) {
	root := t.TempDir()
	s := fsworkspace.NewStore(root)
	for _, id := range []string{"alpha", "zeta"} {
		if err := s.Save(ctx(), sample(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "broken"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken", "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := s.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "alpha" || got[1].ID != "zeta" {
		t.Fatalf("list = %+v", got)
	}
}

// TestTheDirectoryNameIsTheIdentity: a record copied from another installation
// carries the old id in its body, and every reference resolves through the
// directory it is filed under.
func TestTheDirectoryNameIsTheIdentity(t *testing.T) {
	root := t.TempDir()
	s := fsworkspace.NewStore(root)
	if err := os.MkdirAll(filepath.Join(root, "filed-as"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "filed-as", "config.json"),
		[]byte(`{"id":"claims-to-be","name":"X","path":"/repo/x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx(), "filed-as")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "filed-as" {
		t.Fatalf("id = %q, want the directory name", got.ID)
	}
}

// TestAnOlderRecordGetsTheMissingDefaults is the upgrade path: a workspace
// written before a field existed must not come back with a zero taxonomy.
func TestAnOlderRecordGetsTheMissingDefaults(t *testing.T) {
	root := t.TempDir()
	s := fsworkspace.NewStore(root)
	if err := os.MkdirAll(filepath.Join(root, "old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "old", "config.json"),
		[]byte(`{"name":"Old","path":"/repo/old"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ctx(), "old")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tasks) != len(workspace.DefaultTaskTypes) {
		t.Errorf("tasks = %+v, want the defaults", got.Tasks)
	}
	if got.Git.BranchPrefix != workspace.DefaultGit().BranchPrefix {
		t.Errorf("branchPrefix = %q", got.Git.BranchPrefix)
	}
	if got.Worktrees.WorktreeLimit != 15 || got.Color != workspace.DefaultColor {
		t.Errorf("worktrees/color = %+v / %q", got.Worktrees, got.Color)
	}
	if got.Name != "Old" {
		t.Errorf("normalising overwrote a value that was there: %q", got.Name)
	}
}

func TestSaveIsRestrictiveAndAtomic(t *testing.T) {
	root := t.TempDir()
	s := fsworkspace.NewStore(root)
	if err := s.Save(ctx(), sample("alpha")); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(root, "alpha", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %04o, want 0600 — this file lives beside secrets", perm)
	}
	// An atomic write leaves no temporary file behind.
	entries, err := os.ReadDir(filepath.Join(root, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("directory holds %d entries after one save", len(entries))
	}
}

func TestDeleteRemovesTheDerivedStateAndNothingElse(t *testing.T) {
	root := t.TempDir()
	s := fsworkspace.NewStore(root)
	if err := s.Save(ctx(), sample("alpha")); err != nil {
		t.Fatal(err)
	}
	// Derived data lives beside the record and goes with it.
	index := filepath.Join(root, "alpha", "index")
	if err := os.MkdirAll(index, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx(), sample("zeta")); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx(), "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha")); !os.IsNotExist(err) {
		t.Error("the workspace directory survived")
	}
	if _, err := s.Get(ctx(), "zeta"); err != nil {
		t.Errorf("deleting one workspace reached another: %v", err)
	}
	// Deleting what is not there is not an error: the caller asked for a state,
	// and that state holds.
	if err := s.Delete(ctx(), "alpha"); err != nil {
		t.Errorf("deleting twice = %v", err)
	}
}

func TestACancelledContextIsRefused(t *testing.T) {
	s := fsworkspace.NewStore(t.TempDir())
	cancelled, cancel := context.WithCancel(ctx())
	cancel()

	if err := s.Save(cancelled, sample("alpha")); err == nil {
		t.Error("Save ignored a cancelled context")
	}
	if _, err := s.Get(cancelled, "alpha"); err == nil {
		t.Error("Get ignored a cancelled context")
	}
	if _, err := s.List(cancelled); err == nil {
		t.Error("List ignored a cancelled context")
	}
	if err := s.Delete(cancelled, "alpha"); err == nil {
		t.Error("Delete ignored a cancelled context")
	}
}

// An id is a directory name, and it is checked as one.
//
// `filepath.Join(root, "..")` is the parent of the root, with no symlink
// involved and nothing for a containment check to catch — the result is an
// ordinary path that simply is not the one the caller meant. Get read
// ~/.aos/config.json and parsed it as a workspace record (unknown JSON fields
// are ignored), so the service's own existence check waved it through, and
// Delete then took os.RemoveAll to the whole installation: config.json,
// users.json, local.token, the job database and every other workspace.
func TestAnIdThatIsNotOneSegmentIsRefusedRatherThanResolved(t *testing.T) {
	state := t.TempDir()
	// The installation's own files, one level above the workspaces directory.
	if err := os.WriteFile(filepath.Join(state, "config.json"), []byte(`{"id":"not-a-workspace"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "users.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	workspaces := filepath.Join(state, "workspaces")
	if err := os.MkdirAll(workspaces, 0o700); err != nil {
		t.Fatal(err)
	}
	store := fsworkspace.NewStore(workspaces)

	for _, id := range []string{"..", ".", "../..", "a/b", "/etc"} {
		if _, err := store.Get(context.Background(), id); err == nil {
			t.Errorf("Get(%q) resolved to something", id)
		}
		if err := store.Delete(context.Background(), id); err == nil {
			t.Errorf("Delete(%q) was carried out", id)
		}
	}

	if _, err := os.Stat(filepath.Join(state, "config.json")); err != nil {
		t.Fatalf("the installation's own state was deleted: %v", err)
	}
	if _, err := os.Stat(workspaces); err != nil {
		t.Fatalf("the workspaces directory was deleted: %v", err)
	}
}
