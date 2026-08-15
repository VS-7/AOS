package collections_test

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

func TestLockSerialisesWritersOnTheSamePath(t *testing.T) {
	l := collections.NewPathLock("")
	path := filepath.Join(t.TempDir(), "m.md")

	var inside, maxInside atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := l.With(context.Background(), path, func() error {
				n := inside.Add(1)
				for {
					prev := maxInside.Load()
					if n <= prev || maxInside.CompareAndSwap(prev, n) {
						break
					}
				}
				inside.Add(-1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if maxInside.Load() != 1 {
		t.Fatalf("%d writers were inside the lock at once", maxInside.Load())
	}
}

// TestLockKeysOnTheCanonicalPath: "./a/../b.md" and "b.md" are the same file
// and must contend on the same mutex.
func TestLockKeysOnTheCanonicalPath(t *testing.T) {
	l := collections.NewPathLock("")
	dir := t.TempDir()
	direct := filepath.Join(dir, "b.md")
	indirect := filepath.Join(dir, "sub", "..", "b.md")

	held := make(chan struct{})
	released := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = l.With(context.Background(), direct, func() error {
			close(held)
			<-released
			return nil
		})
	}()

	<-held
	done := make(chan struct{})
	go func() {
		_ = l.With(context.Background(), indirect, func() error { return nil })
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("the second path did not contend with the first")
	default:
	}
	close(released)
	<-done
	wg.Wait()
}

func TestLockDoesNotSerialiseDifferentPaths(t *testing.T) {
	l := collections.NewPathLock("")
	dir := t.TempDir()

	first := make(chan struct{})
	second := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = l.With(context.Background(), filepath.Join(dir, "a.md"), func() error {
			close(first)
			<-second // only completes if the other path is not blocked
			return nil
		})
	}()
	go func() {
		defer wg.Done()
		<-first
		_ = l.With(context.Background(), filepath.Join(dir, "b.md"), func() error {
			close(second)
			return nil
		})
	}()
	wg.Wait()
}

// TestWithAllOrdersLocks: acquiring in lexicographic order is what makes a
// multi-file operation deadlock-free, and it is enforced by the lock rather
// than remembered by the caller.
func TestWithAllOrdersLocks(t *testing.T) {
	l := collections.NewPathLock("")
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = l.WithAll(context.Background(), []string{a, b}, func() error { return nil })
		}()
		go func() {
			defer wg.Done()
			_ = l.WithAll(context.Background(), []string{b, a}, func() error { return nil })
		}()
	}
	go func() { wg.Wait(); close(done) }()
	<-done
}

func TestLockRespectsACancelledContext(t *testing.T) {
	l := collections.NewPathLock("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := l.With(ctx, filepath.Join(t.TempDir(), "a.md"), func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Error("a cancelled context must not take the lock")
	}
	if called {
		t.Error("the critical section ran despite cancellation")
	}
}

// TestLockReleasesItsMapEntries keeps the lock from growing to one entry per
// record ever written in a daemon that runs for weeks.
func TestLockReleasesItsMapEntries(t *testing.T) {
	l := collections.NewPathLock("")
	dir := t.TempDir()
	for i := 0; i < 1000; i++ {
		path := filepath.Join(dir, string(rune('a'+i%26))+"-"+filepath.Base(dir)+".md")
		if err := l.With(context.Background(), path, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if n := l.HeldCount(); n != 0 {
		t.Fatalf("%d lock entries survived", n)
	}
}
