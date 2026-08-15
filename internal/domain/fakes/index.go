package fakes

import (
	"context"
	"sort"
	"sync"

	"github.com/OWNER/aos/internal/core/search"
)

// Index is an in-memory full-text index.
//
// It is the same algorithm the domain's fallback scan uses — conjunction of
// tokens over the weighted fields — which is the point: it and the persistent
// index pass one contract, so a test that runs on this one is evidence about
// the other.
type Index struct {
	mu   sync.RWMutex
	docs map[string]search.Document

	// SearchErr, when set, fails every search. It is how a test reaches the
	// path where a broken index degrades to scanning.
	SearchErr error
}

// NewIndex builds an empty in-memory index.
func NewIndex() *Index { return &Index{docs: map[string]search.Document{}} }

func key(collection, id string) string { return collection + "/" + id }

func (i *Index) Upsert(ctx context.Context, d search.Document) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.docs[key(d.Collection, d.ID)] = d
	return nil
}

func (i *Index) Delete(ctx context.Context, collection, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.docs, key(collection, id))
	return nil
}

func (i *Index) Search(ctx context.Context, q search.Query) ([]search.Hit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if i.SearchErr != nil {
		return nil, i.SearchErr
	}
	i.mu.RLock()
	defer i.mu.RUnlock()

	tokens := search.Tokenize(q.Text)
	var hits []search.Hit
	for _, d := range i.docs {
		if q.Collection != "" && d.Collection != q.Collection {
			continue
		}
		if !matchesFilters(d, q.Filters) {
			continue
		}
		if !search.Matches(d, tokens) {
			continue
		}
		hits = append(hits, search.Hit{ID: d.ID, Score: search.Score(d, tokens)})
	}
	// Score first, then id: map iteration is random, and a ranking that
	// reshuffles equal scores between runs is not a ranking.
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].Score == hits[b].Score {
			return hits[a].ID < hits[b].ID
		}
		return hits[a].Score > hits[b].Score
	})
	if q.Limit > 0 && q.Limit < len(hits) {
		hits = hits[:q.Limit]
	}
	return hits, nil
}

// Len reports how many documents the index holds.
func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.docs)
}

func matchesFilters(d search.Document, filters map[string]string) bool {
	for field, want := range filters {
		if want == "" {
			continue
		}
		if d.Filters[field] != want {
			return false
		}
	}
	return true
}
