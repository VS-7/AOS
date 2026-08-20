package fscollections_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/testsuite"
)

func recordModel(id string) collections.Model[collections.Record] {
	return collections.Model[collections.Record]{
		Name:     id,
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/" + id + "/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
}

// TestRecordRepositoryContract proves a dynamic collection's records satisfy
// the same behavioural contract every native repository does: nothing about
// how a record is stored — the atomic write, the per-file lock, the CAS check
// — may depend on whether its fields were known at compile time.
func TestRecordRepositoryContract(t *testing.T) {
	testsuite.RunRepositoryContract(t, testsuite.RepositoryContract[collections.Record]{
		New: func(t *testing.T) collections.Repository[collections.Record] {
			return fscollections.New(t.TempDir(), recordModel("contacts"))
		},
		Sample: func(i int) *collections.Record {
			return &collections.Record{
				Key:     collections.Key{"id": fmt.Sprintf("r-%02d", i)},
				Fields:  map[string]any{"name": fmt.Sprintf("record %02d", i), "stage": "lead"},
				Content: "body\n",
			}
		},
		KeyOf:   collections.KeyOf[collections.Record],
		Mutate:  func(v *collections.Record) { v.Fields["stage"] = "won" },
		Changed: func(v *collections.Record) bool { return v.Fields["stage"] == "won" },
		Filter:  func(v *collections.Record) (string, any) { return "name", v.Fields["name"] },
	})
}

// TestRecordReposBuildsTheRepositoryTheDeclarationDescribes is what proves the
// controller's decision for this task: RecordRepositories is implemented here,
// not deferred to Task 10, and it derives the right Model[collections.Record]
// — right path, right format — from nothing but the collection's declaration.
func TestRecordReposBuildsTheRepositoryTheDeclarationDescribes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repos := fscollections.NewRecordRepos(root)

	c := collection.Collection{
		ID: "contacts", Scope: collection.ScopeWorkspace, Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	}
	repo, err := repos.For(c)
	if err != nil {
		t.Fatal(err)
	}

	rec := &collections.Record{
		Key: collections.Key{"id": "ada"}, Fields: map[string]any{"name": "Ada"},
		Content: "Conheceu o Babbage numa festa.",
	}
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, collections.Key{"id": "ada"})
	if err != nil {
		t.Fatal(err)
	}
	// encodeRecord forces a trailing newline on the body (ADR-0004: these files
	// are meant to be committed to Git and hand-edited, and a missing final
	// newline is a permanent diff blemish), so the body read back carries one
	// even though the one given to Create did not.
	if got.Fields["name"] != "Ada" || got.Content != "Conheceu o Babbage numa festa.\n" {
		t.Fatalf("round trip = %+v", got)
	}

	// The path is the declaration's own contract (DescriptorFor), not a detail
	// this adapter is free to invent: a record of "contacts" lives at exactly
	// where the collection said it would.
	want := filepath.Join(root, ".aos", "collections", "contacts", "records", "ada.md")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("record not at the declared path: %v", err)
	}
}

// TestRecordReposUsesTheCollectionsFormat: a json-format collection's records
// are stored as JSON, the same choice DescriptorFor makes for the collection
// service's own repository.
func TestRecordReposUsesTheCollectionsFormat(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repos := fscollections.NewRecordRepos(root)

	c := collection.Collection{
		ID: "deals", Scope: collection.ScopeWorkspace, Format: collection.FormatJSON,
		Fields: []collection.Field{{Name: "amount", Type: collection.TypeNumber}},
	}
	repo, err := repos.For(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, &collections.Record{
		Key: collections.Key{"id": "d1"}, Fields: map[string]any{"amount": 100.0},
	}); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".aos", "collections", "deals", "records", "d1.json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("record not at the declared json path: %v", err)
	}
}

// TestRecordReposRefusesAnInvalidCollectionID: DescriptorFor's own guard
// against an id that is not a safe path segment reaches the caller through
// this adapter unchanged.
func TestRecordReposRefusesAnInvalidCollectionID(t *testing.T) {
	repos := fscollections.NewRecordRepos(t.TempDir())
	_, err := repos.For(collection.Collection{ID: "../escape"})
	if err == nil {
		t.Fatal("an id that is not a path segment was accepted")
	}
}
