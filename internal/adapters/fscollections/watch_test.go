package fscollections_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/collections"
)

const settle = 2 * time.Second

// startWatcher runs a watcher over root until the test ends.
func startWatcher(t *testing.T, root string, opts ...fscollections.WatcherOption) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".aos"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := fscollections.NewWatcher(root, append(opts, fscollections.WithDebounce(20*time.Millisecond))...)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Watch(ctx); err != nil {
			t.Errorf("watch: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(settle):
			t.Error("the watcher did not stop when its context was cancelled")
		}
	})
	// Give the watcher a moment to register its initial set of directories.
	time.Sleep(50 * time.Millisecond)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(settle)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestWatcherSeesARecordWrittenFromOutside is the reason the watcher exists: a
// record is a file, and the user may create it with an editor or a git checkout.
func TestWatcherSeesARecordWrittenFromOutside(t *testing.T) {
	root := t.TempDir()
	bus := &recordingBus{}
	startWatcher(t, root, fscollections.WithWatchPublisher(bus))

	path := filepath.Join(root, ".aos", "agents", "luara", "memories", "m1.memory.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\ntitle: written by hand\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the memory event", func() bool {
		for _, ev := range bus.snapshot() {
			if ev.Collection == "memories" && ev.Key["id"] == "m1" && ev.Key["agent"] == "luara" {
				return true
			}
		}
		return false
	})
}

// TestWatcherReloadsADynamicSchemaInUnderASecond: an agent creates a collection
// and uses it in the same session, without a restart.
func TestWatcherReloadsADynamicSchemaInUnderASecond(t *testing.T) {
	root := t.TempDir()
	var mu sync.Mutex
	var kinds []string
	startWatcher(t, root, fscollections.OnSchema(func(kind, _ string) {
		mu.Lock()
		defer mu.Unlock()
		kinds = append(kinds, kind)
	}))

	start := time.Now()
	schema := filepath.Join(root, ".aos", "collections", "pull_requests", "schema.json")
	if err := os.MkdirAll(filepath.Dir(schema), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`{"fields":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the schema reload", func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, k := range kinds {
			if k == "collection" {
				return true
			}
		}
		return false
	})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the schema took %v to reload, budget is 1s", elapsed)
	}
}

func TestWatcherReportsViewChanges(t *testing.T) {
	root := t.TempDir()
	var mu sync.Mutex
	seen := false
	startWatcher(t, root, fscollections.OnSchema(func(kind, _ string) {
		if kind == "view" {
			mu.Lock()
			seen = true
			mu.Unlock()
		}
	}))

	view := filepath.Join(root, ".aos", "views", "board.view.json")
	if err := os.MkdirAll(filepath.Dir(view), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(view, []byte(`{"kind":"board"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the view reload", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seen
	})
}

// TestWatcherDoesNotFireForItsOwnTemporaryFiles: without this, every atomic
// write would wake the watcher, which would reindex, which would touch the
// file. The original avoids the loop only by not having a temp file.
func TestWatcherDoesNotFireForItsOwnTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	bus := &recordingBus{}
	startWatcher(t, root, fscollections.WithWatchPublisher(bus))

	dir := filepath.Join(root, ".aos", "agents", "luara", "memories")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, ".tmp-m1-123456")
	if err := os.WriteFile(tmp, []byte("half written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	for _, ev := range bus.snapshot() {
		t.Errorf("a temporary file produced an event: %+v", ev)
	}
}

// TestWatcherReportsADeleteAsADelete distinguishes the two cases a naive
// implementation conflates.
func TestWatcherReportsADeleteAsADelete(t *testing.T) {
	root := t.TempDir()
	bus := &recordingBus{}

	path := filepath.Join(root, ".aos", "agents", "luara", "memories", "m1.memory.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\ntitle: t\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	startWatcher(t, root, fscollections.WithWatchPublisher(bus))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the delete event", func() bool {
		for _, ev := range bus.snapshot() {
			if ev.Collection == "memories" && ev.Op == "delete" {
				return true
			}
		}
		return false
	})
}

// TestWatcherPicksUpDirectoriesCreatedAfterItStarted: a brand new agent must be
// visible without a restart, and fsnotify does not recurse on its own.
func TestWatcherPicksUpDirectoriesCreatedAfterItStarted(t *testing.T) {
	root := t.TempDir()
	bus := &recordingBus{}
	startWatcher(t, root, fscollections.WithWatchPublisher(bus))

	dir := filepath.Join(root, ".aos", "agents", "brand-new", "memories")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // let the new directories be registered
	if err := os.WriteFile(filepath.Join(dir, "m1.memory.md"), []byte("---\ntitle: t\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the event from a directory created after start", func() bool {
		for _, ev := range bus.snapshot() {
			if ev.Key["agent"] == "brand-new" {
				return true
			}
		}
		return false
	})
}

func TestWatcherInvalidatesTheSharedIndex(t *testing.T) {
	root := t.TempDir()
	var mu sync.Mutex
	var invalidated []string
	startWatcher(t, root, fscollections.OnInvalidate(func(rel string) {
		mu.Lock()
		defer mu.Unlock()
		invalidated = append(invalidated, rel)
	}))

	path := filepath.Join(root, ".aos", "agents", "luara", "memories", "m1.memory.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\ntitle: t\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the index invalidation", func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, rel := range invalidated {
			if rel == ".aos/agents/luara/memories/m1.memory.md" {
				return true
			}
		}
		return false
	})
}

// TestWatcherDoesNotMisreadADynamicRecordAsASchemaChange is the regression
// for a schema-path check that used to match any path containing
// "collections/", including a dynamic collection's own records — a write to
// "collections/{id}/records/{id}.md" would misfire onSchema and reload a
// schema that never changed, on every record write to every dynamic
// collection.
func TestWatcherDoesNotMisreadADynamicRecordAsASchemaChange(t *testing.T) {
	root := t.TempDir()
	var mu sync.Mutex
	var kinds []string
	startWatcher(t, root, fscollections.OnSchema(func(kind, _ string) {
		mu.Lock()
		defer mu.Unlock()
		kinds = append(kinds, kind)
	}))

	path := filepath.Join(root, ".aos", "collections", "pull_requests", "records", "r1.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\ntitle: t\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, k := range kinds {
		t.Errorf("a record write inside a dynamic collection fired onSchema(%q, ...); it is not a schema.json", k)
	}
}

func (b *recordingBus) snapshot() []collections.Changed {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]collections.Changed, len(b.events))
	copy(out, b.events)
	return out
}
