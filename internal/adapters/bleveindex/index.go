// Package bleveindex is the persistent full-text index (ADR-0013).
//
// The index is derived data and never a source of truth: the Markdown files in
// the user's repository are. Deleting the index directory and rebuilding is the
// supported recovery procedure, which is why it lives under the installation
// state directory rather than inside the repository — it must never be
// committed.
package bleveindex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"github.com/OWNER/aos/internal/core/apperr"
	coresearch "github.com/OWNER/aos/internal/core/search"
)

// defaultLimit bounds a search that did not ask for one. Bleve defaults to ten,
// which would silently truncate a recall that meant to scan.
const defaultLimit = 1000

// Index is the outbound implementation of the full-text port.
type Index struct {
	dir string

	mu  sync.RWMutex
	idx bleve.Index
}

// Open opens the index at dir, creating it when it is not there.
//
// A corrupt index is not a fatal condition: the caller is expected to fall back
// to scanning, and the error says so rather than pretending the data is gone.
func Open(dir string) (*Index, error) {
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return nil, errUnavailable(dir, err)
	}

	idx, err := bleve.Open(dir)
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		idx, err = bleve.New(dir, buildMapping())
	}
	if err != nil {
		return nil, errUnavailable(dir, err)
	}
	return &Index{dir: dir, idx: idx}, nil
}

// Close releases the index.
func (i *Index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return nil
	}
	err := i.idx.Close()
	i.idx = nil
	return err
}

// Upsert indexes one document.
//
// Writes are synchronous. ADR-0013 describes an asynchronous queue with a
// Strong consistency level that drains it; that arrives with the event bus,
// which is what would feed the queue. Until then every write is already
// visible to the next search, so both consistency levels are satisfied — and a
// queue with no producer would be machinery pretending to be a feature.
func (i *Index) Upsert(ctx context.Context, d coresearch.Document) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return errClosed()
	}
	return i.idx.Index(docKey(d.Collection, d.ID), indexed{
		Collection: d.Collection,
		Title:      d.Text["title"],
		Descriptin: d.Text["description"],
		Tags:       d.Text["tags"],
		Content:    d.Text["content"],
		Agent:      d.Filters["agent"],
		Category:   d.Filters["category"],
		Status:     d.Filters["status"],
	})
}

// Delete removes one document.
func (i *Index) Delete(ctx context.Context, collection, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return errClosed()
	}
	return i.idx.Delete(docKey(collection, id))
}

// Search returns the matching document ids, most relevant first.
//
// Every term is required. That is the same rule the fallback scan applies, and
// the two must agree: a query that means one thing when the index is open and
// another when it is not would make the index a source of behaviour rather than
// of speed.
func (i *Index) Search(ctx context.Context, q coresearch.Query) ([]coresearch.Hit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return nil, errClosed()
	}

	conjunction := bleve.NewConjunctionQuery()
	if q.Collection != "" {
		conjunction.AddQuery(termQuery("collection", q.Collection))
	}
	for field, value := range q.Filters {
		if value == "" {
			continue
		}
		conjunction.AddQuery(termQuery(field, value))
	}
	for _, token := range coresearch.Tokenize(q.Text) {
		conjunction.AddQuery(anyFieldQuery(token))
	}
	if len(conjunction.Conjuncts) == 0 {
		conjunction.AddQuery(bleve.NewMatchAllQuery())
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	req := bleve.NewSearchRequestOptions(conjunction, limit, 0, false)

	res, err := i.idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]coresearch.Hit, 0, len(res.Hits))
	for _, h := range res.Hits {
		out = append(out, coresearch.Hit{ID: idOf(h.ID), Score: h.Score})
	}
	return out, nil
}

// Rebuild discards the index and reindexes everything the iterator yields.
//
// This is the recovery procedure, and the reason the index may be deleted at
// any time without losing data.
func (i *Index) Rebuild(ctx context.Context, docs func(yield func(coresearch.Document) bool)) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.idx != nil {
		if err := i.idx.Close(); err != nil {
			return err
		}
		i.idx = nil
	}
	if err := os.RemoveAll(i.dir); err != nil {
		return errUnavailable(i.dir, err)
	}
	fresh, err := bleve.New(i.dir, buildMapping())
	if err != nil {
		return errUnavailable(i.dir, err)
	}
	i.idx = fresh

	batch := fresh.NewBatch()
	var failure error
	docs(func(d coresearch.Document) bool {
		if err := ctx.Err(); err != nil {
			failure = err
			return false
		}
		if err := batch.Index(docKey(d.Collection, d.ID), indexed{
			Collection: d.Collection,
			Title:      d.Text["title"],
			Descriptin: d.Text["description"],
			Tags:       d.Text["tags"],
			Content:    d.Text["content"],
			Agent:      d.Filters["agent"],
			Category:   d.Filters["category"],
			Status:     d.Filters["status"],
		}); err != nil {
			failure = err
			return false
		}
		return true
	})
	if failure != nil {
		return failure
	}
	return fresh.Batch(batch)
}

// indexed is the flat shape stored in the index.
//
// It is flat because the query side is: a term query names one field, and a map
// would make every field name a runtime string with no compiler behind it. The
// misspelling of Description is deliberate only in that Go needs the field
// exported and Bleve reads it by name — see the mapping below, which pins the
// wire name explicitly.
type indexed struct {
	Collection string `json:"collection"`
	Title      string `json:"title"`
	Descriptin string `json:"description"`
	Tags       string `json:"tags"`
	Content    string `json:"content"`
	Agent      string `json:"agent"`
	Category   string `json:"category"`
	Status     string `json:"status"`
}

// buildMapping declares which fields are analysed prose and which are exact
// terms.
//
// The distinction matters: a status of "ttl_expired" run through an analyser
// becomes two tokens and stops matching a term query for the whole string.
func buildMapping() mapping.IndexMapping {
	text := bleve.NewTextFieldMapping()
	text.Store = false

	exact := bleve.NewKeywordFieldMapping()
	exact.Store = false

	doc := bleve.NewDocumentStaticMapping()
	doc.AddFieldMappingsAt("title", text)
	doc.AddFieldMappingsAt("description", text)
	doc.AddFieldMappingsAt("tags", text)
	doc.AddFieldMappingsAt("content", text)
	doc.AddFieldMappingsAt("collection", exact)
	doc.AddFieldMappingsAt("agent", exact)
	doc.AddFieldMappingsAt("category", exact)
	doc.AddFieldMappingsAt("status", exact)

	m := bleve.NewIndexMapping()
	m.DefaultMapping = doc
	return m
}

// anyFieldQuery requires a token to appear in at least one of the text fields,
// weighted as core/search declares. The weights live there because the fallback
// scan applies the same ones.
func anyFieldQuery(token string) query.Query {
	any := bleve.NewDisjunctionQuery()
	for _, field := range []string{"title", "description", "tags", "content"} {
		m := bleve.NewMatchQuery(token)
		m.SetField(field)
		if w, ok := coresearch.Weights[field]; ok {
			m.SetBoost(w)
		}
		any.AddQuery(m)
	}
	return any
}

func termQuery(field, value string) query.Query {
	t := bleve.NewTermQuery(value)
	t.SetField(field)
	return t
}

// docKey namespaces an id by its collection, so one index serves a whole
// workspace and two collections cannot collide on an identifier.
func docKey(collection, id string) string { return collection + "/" + id }

func idOf(key string) string {
	if _, id, ok := strings.Cut(key, "/"); ok {
		return id
	}
	return key
}

func errUnavailable(dir string, cause error) error {
	return apperr.New("SEARCH_INDEX_UNAVAILABLE").
		Causer("bleveindex.Open").
		Msgf("the search index at %q could not be opened", dir).
		Issue("path", dir).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errClosed() error {
	return apperr.New("SEARCH_INDEX_CLOSED").
		Causer("bleveindex.Index").
		Msgf("the search index is closed").
		Status(apperr.StatusInternalServerError)
}
