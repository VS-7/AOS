package fscollections

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/logging"
)

// DefaultDebounce is how long the watcher waits for the burst to end before
// acting. Editors write in bursts — a temp file, a rename, a chmod — and acting
// on the first event of a burst means reindexing a file three times.
const DefaultDebounce = 200 * time.Millisecond

// WatchPatterns names the dynamic artefacts whose change reloads a schema
// rather than a record. Reloading these live is what lets an agent create a
// collection and use it in the same session.
type WatchPatterns struct {
	Collections string // "collections/*/schema.json"
	Views       string // "views/*.view.json"
	AutoWatch   bool
}

// DefaultWatchPatterns mirrors the original's autoWatch configuration.
func DefaultWatchPatterns() WatchPatterns {
	return WatchPatterns{
		Collections: "collections/",
		Views:       "views/",
		AutoWatch:   true,
	}
}

// Watcher observes one workspace's state directory and reports what changed.
//
// It exists because a record is a file the user may edit directly: in an
// editor, with a script, or by checking out a different Git branch. Without a
// watcher, the in-memory index and the search index would describe a workspace
// that no longer exists.
type Watcher struct {
	root      string
	stateDir  string
	debounce  time.Duration
	patterns  WatchPatterns
	bus       collections.Publisher
	onSchema  func(kind, rel string)
	onInvalid func(rel string)

	mu      sync.Mutex
	pending map[string]struct{}
}

// WatcherOption configures a watcher.
type WatcherOption func(*Watcher)

// WithDebounce overrides the burst window.
func WithDebounce(d time.Duration) WatcherOption {
	return func(w *Watcher) { w.debounce = d }
}

// WithWatchPublisher wires the bus that realtime and the search index listen on.
func WithWatchPublisher(p collections.Publisher) WatcherOption {
	return func(w *Watcher) { w.bus = p }
}

// OnSchema is called when a dynamic collection or view definition changes.
func OnSchema(fn func(kind, rel string)) WatcherOption {
	return func(w *Watcher) { w.onSchema = fn }
}

// OnInvalidate is called for every changed record path, so a shared index can
// drop its entry before anyone reads it again.
func OnInvalidate(fn func(rel string)) WatcherOption {
	return func(w *Watcher) { w.onInvalid = fn }
}

// NewWatcher builds a watcher over a workspace root.
func NewWatcher(root string, opts ...WatcherOption) *Watcher {
	w := &Watcher{
		root:     filepath.Clean(root),
		stateDir: filepath.Join(filepath.Clean(root), collections.Root),
		debounce: DefaultDebounce,
		patterns: DefaultWatchPatterns(),
		bus:      collections.NopPublisher{},
		pending:  map[string]struct{}{},
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Watch blocks until ctx is done, reporting changes as they settle.
//
// fsnotify does not recurse, so every directory under the state directory is
// watched individually and new ones are picked up as they appear — which is
// what makes a brand new agent directory visible without a restart.
func (w *Watcher) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errIO("watch", w.stateDir, err)
	}
	defer func() { _ = watcher.Close() }()

	if err := w.addTree(watcher, w.stateDir); err != nil {
		return err
	}

	log := logging.Component(ctx, "collections.watcher")
	timer := time.NewTimer(w.debounce)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	armed := false

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if w.ignore(ev.Name) {
				continue
			}
			// A new directory must be watched before anything is written into
			// it, or the first record inside is missed. Registering the watch
			// is not enough on its own: `mkdir -p a/b/c && cp rec a/b/c/` only
			// notifies about `a`, and by the time the watch on `a/b/c` exists
			// the file may already be there. So the new subtree is also
			// scanned, and whatever it already holds is reported.
			if ev.Has(fsnotify.Create) {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					if addErr := w.addTree(watcher, ev.Name); addErr != nil {
						log.Warn("cannot watch new directory", "path", ev.Name, "error", addErr)
					}
					w.markTree(ev.Name)
				}
			}
			w.mark(ev.Name)
			if !armed {
				armed = true
			} else if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(w.debounce)

		case werr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Warn("watcher error", "error", werr)

		case <-timer.C:
			armed = false
			w.flush(ctx)
		}
	}
}

func (w *Watcher) addTree(watcher *fsnotify.Watcher, root string) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if isIgnoredDir(d.Name()) {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errIO("watch", root, err)
	}
	return nil
}

// ignore drops the events the engine causes itself. Without this, an atomic
// write would wake the watcher, which would reindex, which would touch the
// file — the loop the original avoids only by not having a temp file.
func (w *Watcher) ignore(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".tmp-") || strings.HasSuffix(base, ".lock") {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if isIgnoredDir(part) {
			return true
		}
	}
	return false
}

// markTree reports every file already inside a directory that has just
// appeared, closing the race between mkdir and the watch that follows it.
func (w *Watcher) markTree(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if isIgnoredDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if w.ignore(path) {
			return nil
		}
		w.mark(path)
		return nil
	})
}

func (w *Watcher) mark(path string) {
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return
	}
	w.mu.Lock()
	w.pending[filepath.ToSlash(rel)] = struct{}{}
	w.mu.Unlock()
}

func (w *Watcher) flush(ctx context.Context) {
	w.mu.Lock()
	batch := make([]string, 0, len(w.pending))
	for rel := range w.pending {
		batch = append(batch, rel)
	}
	w.pending = map[string]struct{}{}
	w.mu.Unlock()

	for _, rel := range batch {
		if w.onInvalid != nil {
			w.onInvalid(rel)
		}
		if kind, ok := w.schemaKind(rel); ok {
			if w.patterns.AutoWatch && w.onSchema != nil {
				w.onSchema(kind, rel)
			}
			continue
		}
		name, key, ok := matchNative(rel)
		if !ok {
			continue
		}
		op := "update"
		if _, err := os.Stat(filepath.Join(w.root, filepath.FromSlash(rel))); err != nil {
			op = "delete"
		}
		w.bus.Publish(ctx, collections.Changed{Collection: name, Key: key, Op: op, Path: rel})
	}
}

// schemaKind matches the schema file itself — "collections/{id}/schema.json"
// — not everything under the collection it declares. A naive prefix check
// also matches "collections/{id}/records/{id}.md": every write to a dynamic
// collection's own records would misroute here as a schema reload instead of
// the record-change publish flush already gives every other native, so a
// dynamic collection's records would look inert on the realtime channel.
func (w *Watcher) schemaKind(rel string) (string, bool) {
	switch {
	case w.patterns.Collections != "" && strings.Contains(rel, "/"+w.patterns.Collections) && strings.HasSuffix(rel, "/schema.json"):
		return "collection", true
	case w.patterns.Views != "" && strings.Contains(rel, "/"+w.patterns.Views) && strings.HasSuffix(rel, ".view.json"):
		return "view", true
	}
	return "", false
}

// matchNative attributes a changed path to the collection that owns it.
func matchNative(rel string) (string, collections.Key, bool) {
	for _, desc := range collections.Natives() {
		for _, p := range desc.Patterns {
			if k, ok := p.Match(rel); ok {
				return desc.Name, k, true
			}
		}
	}
	return "", nil, false
}
