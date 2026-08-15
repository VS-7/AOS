// Package fakes holds the in-memory implementations that domain tests run on.
//
// Every fake here passes the same contract suite as the real adapter
// (internal/domain/testsuite). That is the only reason a domain test proves
// anything: a fake that behaves differently from the real thing turns the whole
// suite into theatre.
package fakes

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repo is an in-memory Repository. Records are stored by key, deep-copied on
// the way in and out, so a caller holding a pointer cannot mutate the store
// behind its back — the filesystem adapter cannot be mutated that way either.
type Repo[T any] struct {
	name  string
	mu    sync.RWMutex
	items map[string]entry[T]
	order []string
	rev   int64
	keyOf func(*T) collections.Key
}

type entry[T any] struct {
	value   T
	version collections.Version
}

// NewRepo builds an empty fake for one collection.
func NewRepo[T any](name string) *Repo[T] {
	return &Repo[T]{
		name:  name,
		items: map[string]entry[T]{},
		keyOf: collections.KeyOf[T],
	}
}

// WithKeyFunc overrides how a key is read out of a record, for entities that do
// not use the collection:"path" tags.
func (r *Repo[T]) WithKeyFunc(fn func(*T) collections.Key) *Repo[T] {
	r.keyOf = fn
	return r
}

func (r *Repo[T]) Get(ctx context.Context, key collections.Key) (*T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.items[key.String()]
	if !ok {
		return nil, collections.NotFoundError(r.name, key)
	}
	v := e.value
	return &v, nil
}

func (r *Repo[T]) Version(ctx context.Context, key collections.Key) (collections.Version, error) {
	if err := ctx.Err(); err != nil {
		return collections.Version{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.items[key.String()]
	if !ok {
		return collections.Version{}, collections.NotFoundError(r.name, key)
	}
	return e.version, nil
}

func (r *Repo[T]) Create(ctx context.Context, v *T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := r.keyOf(v)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[key.String()]; exists {
		return collections.AlreadyExistsError(r.name, key)
	}
	r.items[key.String()] = entry[T]{value: *v, version: r.nextVersion()}
	r.order = append(r.order, key.String())
	sort.Strings(r.order)
	return nil
}

func (r *Repo[T]) Update(ctx context.Context, v *T, expect collections.Version) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := r.keyOf(v)
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.items[key.String()]
	if !ok {
		return collections.NotFoundError(r.name, key)
	}
	if !expect.IsZero() && !e.version.Equal(expect) {
		return collections.ConflictError(r.name, key.String(), expect, e.version)
	}
	r.items[key.String()] = entry[T]{value: *v, version: r.nextVersion()}
	return nil
}

func (r *Repo[T]) Delete(ctx context.Context, key collections.Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[key.String()]; !ok {
		return nil // idempotent, like the real thing
	}
	delete(r.items, key.String())
	for i, k := range r.order {
		if k == key.String() {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func (r *Repo[T]) List(ctx context.Context, q collections.Query) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]T, 0, len(r.order))
	for _, k := range r.order {
		e := r.items[k]
		v := e.value
		if !r.keyOf(&v).Covers(q.Key) {
			continue
		}
		if !matches(&v, q.Filters) {
			continue
		}
		out = append(out, v)
	}
	if q.OrderBy != "" {
		sort.SliceStable(out, func(i, j int) bool {
			a, aok := collections.FieldOf(&out[i], q.OrderBy)
			b, bok := collections.FieldOf(&out[j], q.OrderBy)
			if !aok || !bok {
				return false
			}
			if q.Desc {
				return render(b) < render(a)
			}
			return render(a) < render(b)
		})
	}
	if q.Offset > 0 {
		if q.Offset >= len(out) {
			return []T{}, nil
		}
		out = out[q.Offset:]
	}
	if q.Limit > 0 && q.Limit < len(out) {
		out = out[:q.Limit]
	}
	return out, nil
}

// nextVersion synthesises a token that changes on every write, so the fake
// enforces the same compare-and-swap the filesystem does with modtime and size.
func (r *Repo[T]) nextVersion() collections.Version {
	r.rev++
	return collections.Version{ModTime: time.Unix(0, r.rev), Size: r.rev}
}

// Len reports how many records the fake holds.
func (r *Repo[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
