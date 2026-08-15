package ids_test

import (
	"regexp"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/ids"
)

var canonical = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestUUIDShapeAndUniqueness checks the two properties a caller depends on: the
// canonical form, which is what a memory file name is built from, and that a
// thousand draws do not repeat.
func TestUUIDShapeAndUniqueness(t *testing.T) {
	var g ids.Generator = ids.UUID{}
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := g.New()
		if !canonical.MatchString(id) {
			t.Fatalf("draw %d is not a canonical v4 UUID: %q", i, id)
		}
		if seen[id] {
			t.Fatalf("draw %d repeated: %q", i, id)
		}
		seen[id] = true
	}
}

func TestFixedAlwaysReturnsTheSameID(t *testing.T) {
	g := ids.Fixed{ID: "m1"}
	if a, b := g.New(), g.New(); a != "m1" || b != "m1" {
		t.Fatalf("got %q and %q", a, b)
	}
}

func TestSequenceCountsFromOne(t *testing.T) {
	g := &ids.Sequence{Prefix: "mem"}
	for i, want := range []string{"mem-1", "mem-2", "mem-3"} {
		if got := g.New(); got != want {
			t.Fatalf("draw %d = %q, want %q", i, got, want)
		}
	}
}

func TestSequenceDefaultsItsPrefix(t *testing.T) {
	g := &ids.Sequence{}
	if got := g.New(); got != "id-1" {
		t.Fatalf("New() = %q", got)
	}
}

// TestSequenceIsSafeUnderConcurrency: the concurrency tests of the memory
// service create records from several goroutines, and a fake that hands the
// same id to two of them would fake the very collision being tested.
func TestSequenceIsSafeUnderConcurrency(t *testing.T) {
	g := &ids.Sequence{Prefix: "c"}
	const n = 200

	var mu sync.Mutex
	seen := make(map[string]bool, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			id := g.New()
			mu.Lock()
			defer mu.Unlock()
			if seen[id] {
				t.Errorf("id %q handed out twice", id)
			}
			seen[id] = true
		}()
	}
	wg.Wait()

	if len(seen) != n {
		t.Fatalf("got %d distinct ids, want %d", len(seen), n)
	}
}
