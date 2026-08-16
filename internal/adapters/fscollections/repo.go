// Package fscollections stores collection records as files in the user's
// repository: the implementation of the Repository port over a filesystem.
package fscollections

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/core/collections"
)

// recordMode is the permission of a record file. Records are meant to be read
// and edited by the user and committed to Git; they are not secrets.
const recordMode os.FileMode = 0o644

// Repo is the filesystem implementation of collections.Repository.
type Repo[T any] struct {
	root  string
	model collections.Model[T]
	lock  *collections.PathLock
	index *Index
	bus   collections.Publisher
}

// Option configures a repository.
type Option[T any] func(*Repo[T])

// WithLock replaces the lock, so several repositories over the same workspace
// serialise against each other rather than each on its own map.
func WithLock[T any](l *collections.PathLock) Option[T] {
	return func(r *Repo[T]) { r.lock = l }
}

// WithPublisher wires the event bus that the watcher, the search index and the
// realtime channel listen on.
func WithPublisher[T any](p collections.Publisher) Option[T] {
	return func(r *Repo[T]) { r.bus = p }
}

// WithIndex shares one in-memory index between repositories of a workspace.
func WithIndex[T any](i *Index) Option[T] {
	return func(r *Repo[T]) { r.index = i }
}

// New builds a repository for one collection under one workspace root.
func New[T any](root string, model collections.Model[T], opts ...Option[T]) *Repo[T] {
	r := &Repo[T]{
		root:  filepath.Clean(root),
		model: model,
		// In-process locking by default. A deployment that shares a workspace
		// between the daemon and the CLI passes a lock with a directory —
		// see WithLock and cmd/.
		lock:  collections.NewPathLock(""),
		index: NewIndex(),
		bus:   collections.NopPublisher{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Root returns the workspace root this repository is bound to.
func (r *Repo[T]) Root() string { return r.root }

// resolve turns a workspace-relative path into an absolute one, refusing
// anything that escapes the root. The check happens after resolution, which is
// what closes path traversal — comparing the unresolved string would not.
func (r *Repo[T]) resolve(rel string) (string, error) {
	abs := filepath.Clean(filepath.Join(r.root, filepath.FromSlash(rel)))
	if abs != r.root && !strings.HasPrefix(abs, r.root+string(filepath.Separator)) {
		return "", errOutside(rel, r.root)
	}
	return abs, nil
}

func (r *Repo[T]) relOf(abs string) string {
	rel, err := filepath.Rel(r.root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// pathForKey finds the file of a record. Writable patterns are tried first,
// because that is where a record created by this system lives; the wildcard
// patterns (skill-scoped variants) need a scan, which only happens when the
// direct lookup misses.
func (r *Repo[T]) pathForKey(key collections.Key) (string, bool, error) {
	for _, p := range r.model.Patterns {
		if !p.Writable() || !covers(key, p.Fields()) {
			continue
		}
		rel, err := p.Build(key)
		if err != nil {
			return "", false, err
		}
		abs, err := r.resolve(rel)
		if err != nil {
			return "", false, err
		}
		if _, statErr := os.Stat(abs); statErr == nil {
			return abs, true, nil
		}
	}
	// Scan the wildcard patterns.
	var found string
	err := r.walk(func(abs, rel string, _ *collections.Pattern, k collections.Key) error {
		if found == "" && k.Covers(key) && len(k) == len(key) {
			found = abs
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	if found != "" {
		return found, true, nil
	}
	return "", false, nil
}

// pathForWrite builds the path a new or updated record must occupy.
func (r *Repo[T]) pathForWrite(v *T) (string, collections.Key, error) {
	key := collections.KeyOf(v)
	p, err := r.model.WritePatternFor(key)
	if err != nil {
		return "", nil, err
	}
	rel, err := p.Build(key)
	if err != nil {
		return "", nil, err
	}
	abs, err := r.resolve(rel)
	if err != nil {
		return "", nil, err
	}
	return abs, key, nil
}

func covers(key collections.Key, fields []string) bool {
	for _, f := range fields {
		if key[f] == "" {
			return false
		}
	}
	return true
}

// Get reads one record, body included.
func (r *Repo[T]) Get(ctx context.Context, key collections.Key) (*T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, ok, err := r.pathForKey(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNotFound(r.model.Name, key)
	}
	v, _, err := r.readAt(abs)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Version reports the optimistic-concurrency token of a record, so a caller can
// read, think, and then write with the expectation it captured.
func (r *Repo[T]) Version(ctx context.Context, key collections.Key) (collections.Version, error) {
	if err := ctx.Err(); err != nil {
		return collections.Version{}, err
	}
	abs, ok, err := r.pathForKey(key)
	if err != nil {
		return collections.Version{}, err
	}
	if !ok {
		return collections.Version{}, errNotFound(r.model.Name, key)
	}
	return statVersion(abs)
}

func (r *Repo[T]) readAt(abs string) (*T, collections.Version, error) {
	rel := r.relOf(abs)
	_, key, ok := r.model.MatchPath(rel)
	if !ok {
		return nil, collections.Version{}, errNotOwned(r.model.Name, rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, collections.Version{}, errNotFound(r.model.Name, key)
		}
		return nil, collections.Version{}, errIO("read", rel, err)
	}
	version, err := statVersion(abs)
	if err != nil {
		return nil, collections.Version{}, err
	}
	v, err := collections.Decode(data, key, r.model)
	if err != nil {
		return nil, collections.Version{}, err
	}
	return v, version, nil
}

func statVersion(abs string) (collections.Version, error) {
	info, err := os.Stat(abs)
	if err != nil {
		return collections.Version{}, err
	}
	return collections.Version{ModTime: info.ModTime(), Size: info.Size()}, nil
}

// Create writes a new record, failing when one already exists at its path.
//
// The OnCreated hook runs before the path is computed, because the hook is what
// normalises the key — lowercasing an agent id, for one — and the path must
// reflect the normalised value.
func (r *Repo[T]) Create(ctx context.Context, v *T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.model.OnCreated != nil {
		if err := r.model.OnCreated(ctx, v); err != nil {
			return err
		}
	}
	abs, key, err := r.pathForWrite(v)
	if err != nil {
		return err
	}
	data, err := collections.Encode(v, r.model)
	if err != nil {
		return err
	}
	return r.lock.With(ctx, abs, func() error {
		if _, err := os.Stat(abs); err == nil {
			return errAlreadyExists(r.model.Name, key)
		}
		if err := atomicfs.WriteFile(abs, data, recordMode); err != nil {
			return errIO("create", r.relOf(abs), err)
		}
		r.index.invalidate(r.relOf(abs))
		r.publish(ctx, "create", key, abs)
		return nil
	})
}

// Update rewrites a record. The invariant order is: lock, compare-and-swap,
// hook, encode, atomic write, invalidate the index, publish the event.
func (r *Repo[T]) Update(ctx context.Context, v *T, expect collections.Version) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, key, err := r.pathForWrite(v)
	if err != nil {
		return err
	}
	return r.lock.With(ctx, abs, func() error {
		current, statErr := statVersion(abs)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				return errNotFound(r.model.Name, key)
			}
			return errIO("stat", r.relOf(abs), statErr)
		}
		if !expect.IsZero() && !current.Equal(expect) {
			return errConflict(r.model.Name, r.relOf(abs), expect, current)
		}
		if r.model.OnUpdated != nil {
			old, _, readErr := r.readAt(abs)
			if readErr != nil {
				return readErr
			}
			if err := r.model.OnUpdated(ctx, old, v); err != nil {
				return err
			}
		}
		data, err := collections.Encode(v, r.model)
		if err != nil {
			return err
		}
		if err := atomicfs.WriteFile(abs, data, recordMode); err != nil {
			return errIO("update", r.relOf(abs), err)
		}
		r.index.invalidate(r.relOf(abs))
		r.publish(ctx, "update", key, abs)
		return nil
	})
}

// Delete removes a record. It is idempotent: deleting what is not there
// succeeds, which is the contract every implementation of the port shares.
//
// For a collection whose record is the index file of a directory — TASK.md,
// AGENT.md, ROUTINE.md — the whole directory goes, taking todos, comments and
// runs with it.
func (r *Repo[T]) Delete(ctx context.Context, key collections.Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, ok, err := r.pathForKey(key)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return r.lock.With(ctx, abs, func() error {
		if r.model.OnDeleted != nil {
			v, _, readErr := r.readAt(abs)
			if readErr == nil {
				if err := r.model.OnDeleted(ctx, v); err != nil {
					return err
				}
			}
		}
		target := abs
		recursive := false
		if r.model.CascadeDir != nil {
			if dir := r.model.CascadeDir(abs); dir != "" && dir != r.root {
				target, recursive = dir, true
			}
		}
		var removeErr error
		if recursive {
			removeErr = os.RemoveAll(target)
		} else {
			removeErr = os.Remove(target)
			if errors.Is(removeErr, fs.ErrNotExist) {
				removeErr = nil
			}
		}
		if removeErr != nil {
			return errIO("delete", r.relOf(target), removeErr)
		}
		r.index.invalidatePrefix(r.relOf(target))
		r.publish(ctx, "delete", key, abs)
		return nil
	})
}

// List returns the records matching a query.
//
// Bodies are not loaded. The in-memory index keeps decoded front matter so a
// filtered list does not re-parse the disk, and a body can be tens of kilobytes
// while the workspace inventory only needs names. Set Query.IncludeContent when
// the body is actually wanted.
func (r *Repo[T]) List(ctx context.Context, q collections.Query) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type row struct {
		rel string
		v   *T
	}
	var rows []row

	err := r.walk(func(abs, rel string, _ *collections.Pattern, key collections.Key) error {
		if !key.Covers(q.Key) {
			return nil
		}
		v, err := r.load(abs, rel, key, q.IncludeContent)
		if err != nil {
			return err
		}
		if !matchesFilters(v, q.Filters) {
			return nil
		}
		rows = append(rows, row{rel: rel, v: v})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Ordering is always total: the requested field first, path second, so two
	// records with the same value never swap places between runs.
	sort.SliceStable(rows, func(i, j int) bool {
		if q.OrderBy != "" {
			a, aok := collections.FieldOf(rows[i].v, q.OrderBy)
			b, bok := collections.FieldOf(rows[j].v, q.OrderBy)
			if aok && bok {
				if c := compareValues(a, b); c != 0 {
					if q.Desc {
						return c > 0
					}
					return c < 0
				}
			}
		}
		if q.Desc && q.OrderBy == "" {
			return rows[i].rel > rows[j].rel
		}
		return rows[i].rel < rows[j].rel
	})

	out := make([]T, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row.v)
	}
	return page(out, q.Offset, q.Limit), nil
}

func page[T any](in []T, offset, limit int) []T {
	if offset > 0 {
		if offset >= len(in) {
			return []T{}
		}
		in = in[offset:]
	}
	if limit > 0 && limit < len(in) {
		in = in[:limit]
	}
	return in
}

// load returns a record from the index when the file has not changed, and
// decodes it otherwise.
func (r *Repo[T]) load(abs, rel string, key collections.Key, withBody bool) (*T, error) {
	version, err := statVersion(abs)
	if err != nil {
		return nil, errIO("stat", rel, err)
	}
	if !withBody {
		if cached, ok := r.index.get(rel, version); ok {
			if v, ok := cached.(*T); ok {
				clone := *v
				return &clone, nil
			}
		}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, errIO("read", rel, err)
	}
	v, err := collections.Decode(data, key, r.model)
	if err != nil {
		return nil, err
	}
	if !withBody {
		stripped := collections.WithoutBody(v)
		r.index.put(rel, version, stripped)
		return stripped, nil
	}
	return v, nil
}

// Refresh walks the whole collection and fills the index, so the first List
// after boot does not pay for decoding every record.
func (r *Repo[T]) Refresh(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	count := 0
	err := r.walk(func(abs, rel string, _ *collections.Pattern, key collections.Key) error {
		if _, err := r.load(abs, rel, key, false); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

// walk visits every file that belongs to this collection. It starts from the
// static prefix of each pattern rather than the workspace root, so a repository
// with a large node_modules is never traversed.
func (r *Repo[T]) walk(fn func(abs, rel string, p *collections.Pattern, key collections.Key) error) error {
	seen := map[string]bool{}
	roots := map[string]bool{}
	for _, p := range r.model.Patterns {
		roots[p.Prefix()] = true
	}
	ordered := make([]string, 0, len(roots))
	for prefix := range roots {
		ordered = append(ordered, prefix)
	}
	sort.Strings(ordered)

	for _, prefix := range ordered {
		base, err := r.resolve(prefix)
		if err != nil {
			return err
		}
		if _, statErr := os.Stat(base); errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		walkErr := filepath.WalkDir(base, func(abs string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if isIgnoredDir(name) {
					return filepath.SkipDir
				}
				return nil
			}
			if isIgnoredFile(name) {
				return nil
			}
			rel := r.relOf(abs)
			if seen[rel] {
				return nil
			}
			p, key, ok := r.model.MatchPath(rel)
			if !ok {
				return nil
			}
			seen[rel] = true
			return fn(abs, rel, p, key)
		})
		if walkErr != nil {
			return walkErr
		}
	}
	return nil
}

// isIgnoredDir keeps the walk out of places that cannot hold records and,
// crucially, out of the derived index — which lives outside the repository but
// may be symlinked in during development.
func isIgnoredDir(name string) bool {
	switch name {
	case ".git", "node_modules", "index", "dist", ".venv", "__pycache__":
		return true
	}
	return false
}

// isIgnoredFile skips the temp files of an atomic write. Without this, a write
// in progress would be read as a record and the watcher would loop on itself.
func isIgnoredFile(name string) bool {
	return strings.HasPrefix(name, ".tmp-") || strings.HasSuffix(name, ".lock")
}

func (r *Repo[T]) publish(ctx context.Context, op string, key collections.Key, abs string) {
	r.bus.Publish(ctx, collections.Changed{
		Collection: r.model.Name,
		Key:        key.Clone(),
		Op:         op,
		Path:       r.relOf(abs),
	})
}
