package memory_test

import (
	"errors"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/core/search"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/memory"
)

// newIndexedService is the same service with a search index wired in.
func newIndexedService(t *testing.T) (*memory.Service, *fakes.Index) {
	t.Helper()
	index := fakes.NewIndex()
	svc := memory.NewService(memory.Deps{
		Repo:  fakes.NewRepo[memory.Memory]("memories"),
		Clock: &clockx.Stepping{At: refTime, Step: time.Minute},
		IDs:   &ids.Sequence{Prefix: "m"},
		Index: index,
	})
	return svc, index
}

// corpus is the same set of memories in both services, so the two answers are
// comparable.
func corpus(t *testing.T, svc *memory.Service) {
	t.Helper()
	for _, in := range []memory.StoreInput{
		{
			Title: "UUID migration decision", Description: "Chose v4 over auto-increment",
			Category: memory.CatDecision, Tags: []string{"database"},
		},
		{
			Title: "Gateway restart protocol", Description: "Ask before restarting",
			Category: memory.CatInstruction, Content: "the gateway reloads its config",
		},
		{
			Title: "Postgres pooling", Description: "pgbouncer sits in front",
			Category: memory.CatFact, Tags: []string{"database"},
		},
	} {
		store(t, svc, in)
	}
}

// TestTheIndexChangesTheSpeedAndNotTheAnswer is the claim ADR-0013 rests on.
// If it ever fails, the index has become a source of behaviour instead of a
// source of speed, and a workspace whose index is missing answers differently
// from one whose index is warm.
func TestTheIndexChangesTheSpeedAndNotTheAnswer(t *testing.T) {
	scanning, _ := newService(t)
	indexed, _ := newIndexedService(t)
	corpus(t, scanning)
	corpus(t, indexed)

	queries := []memory.RecallInput{
		{},
		{Query: "uuid"},
		{Query: "gateway"},
		{Query: "database"},
		{Query: "uuid gateway"},
		{Query: "the gateway"},
		{Query: "reloads"},
		{Category: memory.CatDecision},
		{Query: "pooling", Category: memory.CatFact},
		{Query: "absent term"},
	}
	for _, q := range queries {
		withoutIndex, err := scanning.Recall(ctx(), q)
		if err != nil {
			t.Fatal(err)
		}
		withIndex, err := indexed.Recall(ctx(), q)
		if err != nil {
			t.Fatal(err)
		}
		if withIndex.Indexed != (q.Query != "") {
			t.Errorf("%+v: Indexed = %v", q, withIndex.Indexed)
		}
		if got, want := idsOf(withIndex.Memories), idsOf(withoutIndex.Memories); !equal(got, want) {
			t.Errorf("query %+v:\n  indexed = %v\n  scanned = %v", q, got, want)
		}
		if withIndex.Total != withoutIndex.Total {
			t.Errorf("query %+v: totals differ, %d vs %d", q, withIndex.Total, withoutIndex.Total)
		}
	}
}

// TestABrokenIndexDegradesToScanning is ADR-0013's documented fallback: the
// files are the truth, so a search must not go down with its cache.
func TestABrokenIndexDegradesToScanning(t *testing.T) {
	svc, index := newIndexedService(t)
	corpus(t, svc)
	index.SearchErr = errors.New("index corrupted")

	out, err := svc.Recall(ctx(), memory.RecallInput{Query: "uuid"})
	if err != nil {
		t.Fatalf("a broken index must not take the search down: %v", err)
	}
	if out.Total != 1 {
		t.Fatalf("recall = %v", idsOf(out.Memories))
	}
	if out.Indexed {
		t.Error("the result should not claim it came from an index")
	}
}

// TestAMemoryTheIndexHasNotSeenIsStillFound: the index is consulted for order,
// never for membership. A recall that omitted a record because a cache lagged
// would be a silent wrong answer.
func TestAMemoryTheIndexHasNotSeenIsStillFound(t *testing.T) {
	svc, index := newIndexedService(t)
	corpus(t, svc)

	// Simulate a lagging index by emptying it entirely.
	for _, id := range []string{"m-1", "m-2", "m-3"} {
		if err := index.Delete(t.Context(), "memories", id); err != nil {
			t.Fatal(err)
		}
	}
	if index.Len() != 0 {
		t.Fatalf("the index still holds %d documents", index.Len())
	}

	out, err := svc.Recall(ctx(), memory.RecallInput{Query: "uuid"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 {
		t.Fatalf("a record missing from the index was lost: %v", idsOf(out.Memories))
	}
}

// TestWritesReachTheIndex covers the other direction: a store and a forget both
// have to update the derived data, or the ranking drifts from the files.
func TestWritesReachTheIndex(t *testing.T) {
	svc, index := newIndexedService(t)
	m := store(t, svc, memory.StoreInput{Title: "Indexed on write"}).Memory
	if index.Len() != 1 {
		t.Fatalf("the index holds %d documents after a store", index.Len())
	}

	if _, err := svc.Forget(ctx(), memory.ForgetInput{
		Memory: m.ID, Reason: "It stopped being true after the migration.",
	}); err != nil {
		t.Fatal(err)
	}
	// The status is what recall filters on, so the index has to carry the new
	// one rather than the one the document was first written with.
	hits, err := index.Search(t.Context(), search.Query{
		Collection: "memories",
		Filters:    map[string]string{"status": string(memory.StatusDeprecated)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != m.ID {
		t.Fatalf("hits = %+v, want the deprecated memory", hits)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
