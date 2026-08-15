package fakes_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/testsuite"
)

type record struct {
	ID    string `json:"id"    collection:"path"`
	Agent string `json:"agent" collection:"path"`

	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`

	Content string `json:"content" collection:"content"`
}

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// TestFakeRepositoryContract runs the same suite as the filesystem adapter.
// This is the only reason a domain test that uses the fake proves anything: a
// fake that behaves differently from the real thing makes the whole suite
// theatre.
func TestFakeRepositoryContract(t *testing.T) {
	testsuite.RunRepositoryContract(t, testsuite.RepositoryContract[record]{
		New: func(t *testing.T) collections.Repository[record] {
			return fakes.NewRepo[record]("memories")
		},
		Sample: func(i int) *record {
			return &record{
				Agent: "luara", ID: fmt.Sprintf("m-%02d", i),
				Title: fmt.Sprintf("memory %02d", i), Category: fmt.Sprintf("cat-%02d", i),
				Status: "active", CreatedAt: refTime.Add(time.Duration(i) * time.Hour),
				Content: "body\n",
			}
		},
		KeyOf:   collections.KeyOf[record],
		Mutate:  func(v *record) { v.Status = "deprecated" },
		Changed: func(v *record) bool { return v.Status == "deprecated" },
		Filter:  func(v *record) (string, any) { return "category", v.Category },
	})
}

// TestFakeDoesNotShareStateWithItsCallers: the filesystem cannot be mutated by
// holding a pointer to a record, so neither may the fake.
func TestFakeDoesNotShareStateWithItsCallers(t *testing.T) {
	ctx := t.Context()
	repo := fakes.NewRepo[record]("memories")
	v := &record{Agent: "luara", ID: "m1", Title: "original"}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}

	v.Title = "mutated behind the store's back"
	got, err := repo.Get(ctx, collections.Key{"agent": "luara", "id": "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "original" {
		t.Fatalf("the fake shared its storage with the caller: %q", got.Title)
	}

	got.Title = "mutated through the reader"
	again, err := repo.Get(ctx, collections.Key{"agent": "luara", "id": "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Title != "original" {
		t.Fatalf("a reader mutated the store: %q", again.Title)
	}
}
