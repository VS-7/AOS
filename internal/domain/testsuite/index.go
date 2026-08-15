package testsuite

import (
	"context"
	"testing"

	"github.com/OWNER/aos/internal/core/search"
)

// IndexContract describes one full-text implementation well enough to exercise
// it.
type IndexContract struct {
	// New returns a fresh, empty index.
	New func(t *testing.T) search.Index
}

// RunIndexContract exercises the behaviour the memory aggregate relies on.
//
// The properties are chosen for one reason: recall must return the same set of
// memories whether an index is open or not. Everything below is a way in which
// the two could disagree.
func RunIndexContract(t *testing.T, c IndexContract) {
	t.Helper()
	ctx := context.Background()

	doc := func(id, title, description, content, tags string, filters map[string]string) search.Document {
		return search.Document{
			ID: id, Collection: "memories",
			Text:    map[string]string{"title": title, "description": description, "content": content, "tags": tags},
			Filters: filters,
		}
	}

	seed := func(t *testing.T) search.Index {
		t.Helper()
		idx := c.New(t)
		docs := []search.Document{
			doc("m1", "UUID migration decision", "Chose v4 over auto-increment", "", "database design",
				map[string]string{"agent": "atlas", "category": "decision", "status": "active"}),
			doc("m2", "Gateway restart protocol", "Ask before restarting", "the gateway reloads config", "gateway",
				map[string]string{"agent": "atlas", "category": "instruction", "status": "active"}),
			doc("m3", "Postgres pooling", "pgbouncer in front", "", "database",
				map[string]string{"agent": "luara", "category": "fact", "status": "deprecated"}),
		}
		for _, d := range docs {
			if err := idx.Upsert(ctx, d); err != nil {
				t.Fatalf("seed %s: %v", d.ID, err)
			}
		}
		return idx
	}

	t.Run("a term in the title is found", func(t *testing.T) {
		hits, err := seed(t).Search(ctx, search.Query{Collection: "memories", Text: "uuid"})
		if err != nil {
			t.Fatal(err)
		}
		if !hasID(hits, "m1") {
			t.Fatalf("hits = %v", idsOf(hits))
		}
	})

	t.Run("a term in the body is found", func(t *testing.T) {
		hits, err := seed(t).Search(ctx, search.Query{Collection: "memories", Text: "reloads"})
		if err != nil {
			t.Fatal(err)
		}
		if !hasID(hits, "m2") {
			t.Fatalf("hits = %v", idsOf(hits))
		}
	})

	t.Run("a term in the tags is found", func(t *testing.T) {
		hits, err := seed(t).Search(ctx, search.Query{Collection: "memories", Text: "pgbouncer"})
		if err != nil {
			t.Fatal(err)
		}
		if !hasID(hits, "m3") {
			t.Fatalf("hits = %v", idsOf(hits))
		}
	})

	t.Run("every term is required", func(t *testing.T) {
		// "uuid" is in m1 and "gateway" in m2, so a disjunction would return
		// both. Recall means the conjunction, and the two paths must agree.
		hits, err := seed(t).Search(ctx, search.Query{Collection: "memories", Text: "uuid gateway"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Fatalf("hits = %v, want none", idsOf(hits))
		}
	})

	t.Run("stop words do not narrow the result", func(t *testing.T) {
		idx := seed(t)
		with, err := idx.Search(ctx, search.Query{Collection: "memories", Text: "the uuid"})
		if err != nil {
			t.Fatal(err)
		}
		without, err := idx.Search(ctx, search.Query{Collection: "memories", Text: "uuid"})
		if err != nil {
			t.Fatal(err)
		}
		if len(with) != len(without) {
			t.Fatalf("with a stop word %v, without %v", idsOf(with), idsOf(without))
		}
	})

	t.Run("an empty query matches everything in the collection", func(t *testing.T) {
		hits, err := seed(t).Search(ctx, search.Query{Collection: "memories"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 3 {
			t.Fatalf("hits = %v, want all three", idsOf(hits))
		}
	})

	t.Run("filters are exact", func(t *testing.T) {
		idx := seed(t)
		hits, err := idx.Search(ctx, search.Query{
			Collection: "memories", Filters: map[string]string{"agent": "luara"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].ID != "m3" {
			t.Fatalf("hits = %v", idsOf(hits))
		}

		// A value with an underscore must survive as one term: a status of
		// "ttl_expired" analysed as prose becomes two tokens and stops matching.
		if err := idx.Upsert(ctx, doc("m4", "Expired", "d", "", "",
			map[string]string{"agent": "atlas", "category": "fact", "status": "ttl_expired"})); err != nil {
			t.Fatal(err)
		}
		hits, err = idx.Search(ctx, search.Query{
			Collection: "memories", Filters: map[string]string{"status": "ttl_expired"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].ID != "m4" {
			t.Fatalf("hits = %v, want only the expired one", idsOf(hits))
		}
	})

	t.Run("another collection is not searched", func(t *testing.T) {
		idx := seed(t)
		other := doc("m1", "UUID something else", "d", "", "", nil)
		other.Collection = "instructions"
		if err := idx.Upsert(ctx, other); err != nil {
			t.Fatal(err)
		}
		hits, err := idx.Search(ctx, search.Query{Collection: "memories", Text: "uuid"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 {
			t.Fatalf("hits = %v — the same id in another collection leaked in", idsOf(hits))
		}
	})

	t.Run("upsert replaces rather than duplicates", func(t *testing.T) {
		idx := seed(t)
		if err := idx.Upsert(ctx, doc("m1", "Completely different", "d", "", "", nil)); err != nil {
			t.Fatal(err)
		}
		hits, err := idx.Search(ctx, search.Query{Collection: "memories", Text: "uuid"})
		if err != nil {
			t.Fatal(err)
		}
		if hasID(hits, "m1") {
			t.Fatal("the replaced document still matches its old text")
		}
		hits, err = idx.Search(ctx, search.Query{Collection: "memories", Text: "different"})
		if err != nil {
			t.Fatal(err)
		}
		if count(hits, "m1") != 1 {
			t.Fatalf("m1 appears %d times", count(hits, "m1"))
		}
	})

	t.Run("delete removes the document", func(t *testing.T) {
		idx := seed(t)
		if err := idx.Delete(ctx, "memories", "m1"); err != nil {
			t.Fatal(err)
		}
		hits, err := idx.Search(ctx, search.Query{Collection: "memories", Text: "uuid"})
		if err != nil {
			t.Fatal(err)
		}
		if hasID(hits, "m1") {
			t.Fatalf("hits = %v", idsOf(hits))
		}
		// Deleting what is not there is not an error: the caller asked for a
		// state, and that state holds.
		if err := idx.Delete(ctx, "memories", "m1"); err != nil {
			t.Errorf("deleting twice = %v", err)
		}
	})

	t.Run("a write is visible to the next search", func(t *testing.T) {
		// This is what lets `store` recall first to avoid writing a duplicate.
		idx := c.New(t)
		if err := idx.Upsert(ctx, doc("fresh", "Just written", "d", "", "", nil)); err != nil {
			t.Fatal(err)
		}
		hits, err := idx.Search(ctx, search.Query{
			Collection: "memories", Text: "written", Consistency: search.Strong,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !hasID(hits, "fresh") {
			t.Fatalf("hits = %v", idsOf(hits))
		}
	})

	t.Run("the limit bounds the result", func(t *testing.T) {
		hits, err := seed(t).Search(ctx, search.Query{Collection: "memories", Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 2 {
			t.Fatalf("hits = %v, want two", idsOf(hits))
		}
	})

	t.Run("searching an empty index is not an error", func(t *testing.T) {
		hits, err := c.New(t).Search(ctx, search.Query{Collection: "memories", Text: "anything"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Fatalf("hits = %v", idsOf(hits))
		}
	})

	t.Run("a cancelled context is refused", func(t *testing.T) {
		idx := c.New(t)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if err := idx.Upsert(cancelled, doc("x", "t", "d", "", "", nil)); err == nil {
			t.Error("Upsert ignored a cancelled context")
		}
		if _, err := idx.Search(cancelled, search.Query{Collection: "memories"}); err == nil {
			t.Error("Search ignored a cancelled context")
		}
	})
}

func idsOf(hits []search.Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.ID
	}
	return out
}

func hasID(hits []search.Hit, want string) bool { return count(hits, want) > 0 }

func count(hits []search.Hit, want string) int {
	n := 0
	for _, h := range hits {
		if h.ID == want {
			n++
		}
	}
	return n
}
