//go:build !race

// The scale assertion is excluded from the race build on purpose. The race
// detector multiplies wall time by a factor that varies with the machine, and
// the budget in the specification is about the engine — "Refresh() of a
// workspace with 10,000 records under 2 s" — not about the detector. Measuring
// under -race would test the wrong thing and would fail for the wrong reason.
//
// The test still runs on every plain `go test`, which is what the coverage gate
// executes.

package fscollections_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/testx/fixture"
)

// TestRefreshOfALargeWorkspaceIsUnderTwoSeconds is the measured trigger the
// specification sets for reopening the SQLite-mirror decision: ten thousand
// memories must index in under two seconds, or the in-memory index is no
// longer the right answer.
func TestRefreshOfALargeWorkspaceIsUnderTwoSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("writes ten thousand files")
	}
	generated := time.Now()
	fx := fixture.Workspace(t, fixture.Large)
	t.Logf("generated %d records in %v", fx.Total(), time.Since(generated))

	repo := fscollections.New(fx.Root, modelFor(t, "memories"))

	start := time.Now()
	n, err := repo.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	t.Logf("Refresh indexed %d memories in %v", n, elapsed)

	if n != 10_005 { // 10 agents x 1000, plus one per skill-scoped agent
		t.Fatalf("Refresh indexed %d memories, expected 10005", n)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Refresh took %v, budget is 2s — reopen the SQLite mirror decision", elapsed)
	}

	// A warm List must not re-parse: the index is what keeps it cheap.
	warm := time.Now()
	got, err := repo.List(context.Background(), collections.Query{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("warm List of 50 records in %v", time.Since(warm))
	if len(got) != 50 {
		t.Fatalf("list returned %d records", len(got))
	}
}
