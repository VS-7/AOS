// Package testsuite holds the executable contracts of the ports.
//
// Every implementation of a port runs the same suite. If the fake and the real
// implementation differ in observable behaviour, one of them is wrong — and it
// is better to find out here than in production. Without this, the fakes that
// every domain test runs on are theatre.
package testsuite

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
)

// RepositoryContract describes one implementation well enough to exercise it.
type RepositoryContract[T any] struct {
	// New returns a fresh, empty repository.
	New func(t *testing.T) collections.Repository[T]

	// Sample builds the i-th distinct record. Records must differ in key.
	Sample func(i int) *T

	// KeyOf extracts the key of a record.
	KeyOf func(v *T) collections.Key

	// Mutate changes something Update should persist.
	Mutate func(v *T)

	// Changed reports whether Mutate's change is present in a record read back.
	Changed func(v *T) bool

	// Filter returns a front-matter field and value that select the given
	// record and no other.
	Filter func(v *T) (field string, value any)
}

// RunRepositoryContract exercises the behavioural contract every Repository
// implementation must satisfy.
func RunRepositoryContract[T any](t *testing.T, c RepositoryContract[T]) {
	t.Helper()
	ctx := context.Background()

	t.Run("create then get round-trips", func(t *testing.T) {
		repo := c.New(t)
		want := c.Sample(0)
		if err := repo.Create(ctx, want); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.Get(ctx, c.KeyOf(want))
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if c.KeyOf(got).String() != c.KeyOf(want).String() {
			t.Fatalf("key = %s, want %s", c.KeyOf(got), c.KeyOf(want))
		}
	})

	t.Run("get missing returns not found", func(t *testing.T) {
		repo := c.New(t)
		_, err := repo.Get(ctx, c.KeyOf(c.Sample(0)))
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
		if e, ok := apperr.As(err); !ok || len(e.Actions) == 0 {
			t.Error("a 404 must carry a CTA: the caller can act on it")
		}
	})

	t.Run("create duplicate returns conflict", func(t *testing.T) {
		repo := c.New(t)
		v := c.Sample(0)
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
		err := repo.Create(ctx, c.Sample(0))
		if !errors.Is(err, apperr.ErrConflict) {
			t.Fatalf("error = %v, want ErrConflict", err)
		}
	})

	t.Run("update persists the change", func(t *testing.T) {
		repo := c.New(t)
		v := c.Sample(0)
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
		c.Mutate(v)
		if err := repo.Update(ctx, v, collections.Version{}); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err := repo.Get(ctx, c.KeyOf(v))
		if err != nil {
			t.Fatal(err)
		}
		if !c.Changed(got) {
			t.Fatal("the update was not persisted")
		}
	})

	t.Run("update missing returns not found", func(t *testing.T) {
		repo := c.New(t)
		err := repo.Update(ctx, c.Sample(0), collections.Version{})
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("update with stale version returns conflict", func(t *testing.T) {
		repo := c.New(t)
		v := c.Sample(0)
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
		versioned, ok := repo.(collections.Versioned)
		if !ok {
			t.Skip("implementation does not report versions")
		}
		stale, err := versioned.Version(ctx, c.KeyOf(v))
		if err != nil {
			t.Fatal(err)
		}

		// Someone else writes in between.
		other := c.Sample(0)
		c.Mutate(other)
		if err := repo.Update(ctx, other, collections.Version{}); err != nil {
			t.Fatal(err)
		}

		err = repo.Update(ctx, v, stale)
		if !errors.Is(err, apperr.ErrConflict) {
			t.Fatalf("error = %v, want ErrConflict — a lost update is the defect this prevents", err)
		}
		e, _ := apperr.As(err)
		if e == nil || len(e.Actions) == 0 {
			t.Error("a conflict must tell the caller to reload and reapply")
		}
	})

	t.Run("update with current version succeeds", func(t *testing.T) {
		repo := c.New(t)
		v := c.Sample(0)
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
		versioned, ok := repo.(collections.Versioned)
		if !ok {
			t.Skip("implementation does not report versions")
		}
		current, err := versioned.Version(ctx, c.KeyOf(v))
		if err != nil {
			t.Fatal(err)
		}
		c.Mutate(v)
		if err := repo.Update(ctx, v, current); err != nil {
			t.Fatalf("update with the current version must succeed: %v", err)
		}
	})

	t.Run("delete is idempotent", func(t *testing.T) {
		repo := c.New(t)
		v := c.Sample(0)
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, c.KeyOf(v)); err != nil {
			t.Fatalf("first delete: %v", err)
		}
		if err := repo.Delete(ctx, c.KeyOf(v)); err != nil {
			t.Fatalf("deleting what is not there must succeed: %v", err)
		}
		if _, err := repo.Get(ctx, c.KeyOf(v)); !errors.Is(err, apperr.ErrNotFound) {
			t.Fatalf("get after delete = %v", err)
		}
	})

	t.Run("list returns everything created", func(t *testing.T) {
		repo := c.New(t)
		for i := 0; i < 5; i++ {
			if err := repo.Create(ctx, c.Sample(i)); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.List(ctx, collections.Query{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 5 {
			t.Fatalf("list returned %d records, want 5", len(got))
		}
	})

	t.Run("list applies filters", func(t *testing.T) {
		repo := c.New(t)
		for i := 0; i < 5; i++ {
			if err := repo.Create(ctx, c.Sample(i)); err != nil {
				t.Fatal(err)
			}
		}
		target := c.Sample(2)
		field, value := c.Filter(target)
		got, err := repo.List(ctx, collections.Query{Filters: map[string]any{field: value}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("filter %s=%v returned %d records, want 1", field, value, len(got))
		}
	})

	t.Run("list ordering is stable", func(t *testing.T) {
		repo := c.New(t)
		for i := 0; i < 8; i++ {
			if err := repo.Create(ctx, c.Sample(i)); err != nil {
				t.Fatal(err)
			}
		}
		first, err := repo.List(ctx, collections.Query{})
		if err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 5; attempt++ {
			again, err := repo.List(ctx, collections.Query{})
			if err != nil {
				t.Fatal(err)
			}
			if len(again) != len(first) {
				t.Fatalf("list length changed between runs: %d then %d", len(first), len(again))
			}
			for i := range again {
				if c.KeyOf(&again[i]).String() != c.KeyOf(&first[i]).String() {
					t.Fatalf("list order changed between runs at %d", i)
				}
			}
		}
	})

	t.Run("list paginates", func(t *testing.T) {
		repo := c.New(t)
		for i := 0; i < 6; i++ {
			if err := repo.Create(ctx, c.Sample(i)); err != nil {
				t.Fatal(err)
			}
		}
		all, err := repo.List(ctx, collections.Query{})
		if err != nil {
			t.Fatal(err)
		}
		page, err := repo.List(ctx, collections.Query{Offset: 2, Limit: 3})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) != 3 {
			t.Fatalf("page has %d records, want 3", len(page))
		}
		for i := range page {
			if c.KeyOf(&page[i]).String() != c.KeyOf(&all[i+2]).String() {
				t.Fatalf("page does not line up with the full list at %d", i)
			}
		}
	})

	t.Run("concurrent writers do not corrupt", func(t *testing.T) {
		repo := c.New(t)
		v := c.Sample(0)
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				w := c.Sample(0)
				if i%2 == 0 {
					c.Mutate(w)
				}
				// No expectation: last writer wins, but no reader may ever see
				// a half-written record.
				if err := repo.Update(ctx, w, collections.Version{}); err != nil {
					t.Errorf("update: %v", err)
				}
			}(i)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := repo.Get(ctx, c.KeyOf(v)); err != nil {
					t.Errorf("get during concurrent writes: %v", err)
				}
			}()
		}
		wg.Wait()
	})

	t.Run("cancelled context is respected", func(t *testing.T) {
		repo := c.New(t)
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := repo.List(cancelled, collections.Query{}); err == nil {
			t.Error("List ignored a cancelled context")
		}
		if err := repo.Create(cancelled, c.Sample(0)); err == nil {
			t.Error("Create ignored a cancelled context")
		}
	})
}
