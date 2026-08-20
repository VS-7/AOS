package fscollections

import (
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
)

// RecordRepos implements collection.RecordRepositories over the filesystem:
// given a collection's declaration, it builds the Repo[collections.Record]
// that serves that collection's rows.
//
// It is not cached by collection id. A collection's Format and Scope are what
// its Model[collections.Record] is built from, and both are declared once —
// nothing in this phase updates a declaration after Create — but caching would
// mean a repository built from a stale declaration silently outliving the one
// that replaced it, for a struct cheap enough to allocate on every call that
// the risk buys nothing. The lock, index and event bus underneath it are still
// shared across every collection, which is what keeps two repositories over
// the same workspace serialising against each other instead of racing.
type RecordRepos struct {
	root  string
	lock  *collections.PathLock
	index *Index
	bus   collections.Publisher
}

// RecordReposOption configures a RecordRepos, mirroring Option[T] above.
type RecordReposOption func(*RecordRepos)

// WithRecordLock replaces the lock, so record repositories serialise against
// the same lock the native repositories of a workspace use.
func WithRecordLock(l *collections.PathLock) RecordReposOption {
	return func(r *RecordRepos) { r.lock = l }
}

// WithRecordIndex shares one in-memory index with the native repositories of a
// workspace.
func WithRecordIndex(i *Index) RecordReposOption {
	return func(r *RecordRepos) { r.index = i }
}

// WithRecordPublisher wires the event bus a collection's records publish
// Changed events on.
func WithRecordPublisher(p collections.Publisher) RecordReposOption {
	return func(r *RecordRepos) { r.bus = p }
}

// NewRecordRepos builds the factory for one workspace root.
func NewRecordRepos(root string, opts ...RecordReposOption) *RecordRepos {
	r := &RecordRepos{
		root:  root,
		lock:  collections.NewPathLock(""),
		index: NewIndex(),
		bus:   collections.NopPublisher{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// For returns the repository bound to one collection's records.
func (r *RecordRepos) For(c collection.Collection) (collection.RecordRepo, error) {
	desc, err := collection.DescriptorFor(c)
	if err != nil {
		return nil, err
	}
	model := collections.Model[collections.Record]{
		Name:     desc.Name,
		Patterns: desc.Patterns,
		Format:   desc.Format,
	}
	return New(r.root, model,
		WithLock[collections.Record](r.lock),
		WithIndex[collections.Record](r.index),
		WithPublisher[collections.Record](r.bus),
	), nil
}
