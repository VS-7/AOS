package bleveindex_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/OWNER/aos/internal/adapters/bleveindex"
	"github.com/OWNER/aos/internal/core/search"
	"github.com/OWNER/aos/internal/domain/testsuite"
)

func open(t *testing.T) *bleveindex.Index {
	t.Helper()
	idx, err := bleveindex.Open(filepath.Join(t.TempDir(), "index"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	return idx
}

// TestSatisfiesTheIndexContract runs the same suite as the in-memory fake the
// domain tests use. Two implementations that disagree turn every test above
// into theatre.
func TestSatisfiesTheIndexContract(t *testing.T) {
	testsuite.RunIndexContract(t, testsuite.IndexContract{
		New: func(t *testing.T) search.Index { return open(t) },
	})
}

func doc(id, title string) search.Document {
	return search.Document{
		ID: id, Collection: "memories",
		Text: map[string]string{"title": title},
	}
}

// TestTheIndexSurvivesAReopen is the difference from an in-memory index and the
// reason ADR-0013 chose a persistent one: the cost of a rebuild at boot grows
// with the history, and accumulating history is the product.
func TestTheIndexSurvivesAReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "index")
	first, err := bleveindex.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Upsert(t.Context(), doc("m1", "persisted across a restart")); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := bleveindex.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()

	hits, err := second.Search(t.Context(), search.Query{Collection: "memories", Text: "persisted"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "m1" {
		t.Fatalf("hits = %+v", hits)
	}
}

// TestRebuildIsTheRecoveryProcedure: the index is derived data, so throwing it
// away and reconstructing from the files must always be safe.
func TestRebuildIsTheRecoveryProcedure(t *testing.T) {
	idx := open(t)
	if err := idx.Upsert(t.Context(), doc("stale", "this record no longer exists on disk")); err != nil {
		t.Fatal(err)
	}

	err := idx.Rebuild(t.Context(), func(yield func(search.Document) bool) {
		for _, d := range []search.Document{doc("m1", "real one"), doc("m2", "real two")} {
			if !yield(d) {
				return
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	hits, err := idx.Search(t.Context(), search.Query{Collection: "memories"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %+v, want exactly the two rebuilt", hits)
	}
	for _, h := range hits {
		if h.ID == "stale" {
			t.Fatal("a record that is no longer on disk survived the rebuild")
		}
	}
}

func TestACancelledRebuildDoesNotHang(t *testing.T) {
	idx := open(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := idx.Rebuild(ctx, func(yield func(search.Document) bool) {
		for i := 0; ; i++ {
			if !yield(doc(strconv.Itoa(i), "endless")) {
				return
			}
			if i > 10_000 {
				t.Error("the iterator was never told to stop")
				return
			}
		}
	})
	if err == nil {
		t.Fatal("a cancelled rebuild should report the cancellation")
	}
}

func TestAClosedIndexRefusesRatherThanPanics(t *testing.T) {
	idx, err := bleveindex.Open(filepath.Join(t.TempDir(), "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}
	// Closing twice is not an error: the caller asked for a state and it holds.
	if err := idx.Close(); err != nil {
		t.Errorf("closing twice = %v", err)
	}
	if err := idx.Upsert(t.Context(), doc("m1", "t")); err == nil {
		t.Error("Upsert on a closed index must fail")
	}
	if _, err := idx.Search(t.Context(), search.Query{Collection: "memories"}); err == nil {
		t.Error("Search on a closed index must fail")
	}
	if err := idx.Delete(t.Context(), "memories", "m1"); err == nil {
		t.Error("Delete on a closed index must fail")
	}
}

// TestRankingFollowsTheDeclaredWeights: the fallback scan ranks by the same
// table, so the two must not order the same documents differently.
func TestRankingFollowsTheDeclaredWeights(t *testing.T) {
	idx := open(t)
	inTitle := search.Document{ID: "title", Collection: "memories",
		Text: map[string]string{"title": "gateway", "content": "unrelated prose"}}
	inBody := search.Document{ID: "body", Collection: "memories",
		Text: map[string]string{"title": "unrelated", "content": "gateway"}}
	for _, d := range []search.Document{inBody, inTitle} {
		if err := idx.Upsert(t.Context(), d); err != nil {
			t.Fatal(err)
		}
	}

	hits, err := idx.Search(t.Context(), search.Query{Collection: "memories", Text: "gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].ID != "title" {
		t.Fatalf("hits = %+v, want the title match first", hits)
	}
}
