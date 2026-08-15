// Package search declares the full-text port and the tokenisation both sides
// of it agree on.
//
// The types live in core rather than in the aggregate that uses them because
// two will: memories today, user-defined collection records later. The
// implementation lives in an adapter, and the index it maintains is derived
// data — the Markdown files are the truth, and deleting the index is the
// recovery procedure (ADR-0013).
package search

import (
	"context"
	"strings"
	"unicode"
)

// Consistency says how fresh the answer has to be.
//
// Indexing is asynchronous, which is right for the common case: a session that
// starts by recalling does not care that a memory written four seconds ago is
// not yet indexed. Storing does care — it recalls first to avoid writing a
// duplicate — so it asks for Strong and pays for the drain.
type Consistency int

const (
	Eventual Consistency = iota
	Strong
)

// Document is one indexed record.
type Document struct {
	// ID is unique within the collection.
	ID string

	// Collection scopes the document, so one index serves a whole workspace.
	Collection string

	// Text holds the searchable fields by name. The implementation decides how
	// to weight them; the weights are declared in Weights below so that the
	// index and the fallback scan rank alike.
	Text map[string]string

	// Filters are exact-match attributes: status, category, agent.
	Filters map[string]string
}

// Query selects documents.
type Query struct {
	Collection  string
	Text        string
	Filters     map[string]string
	Limit       int
	Consistency Consistency
}

// Hit is one result, most relevant first.
type Hit struct {
	ID    string
	Score float64
}

// Index is the port. Three methods: what a caller does to a search index.
type Index interface {
	Upsert(ctx context.Context, d Document) error
	Delete(ctx context.Context, collection, id string) error
	Search(ctx context.Context, q Query) ([]Hit, error)
}

// Weights are the field weights the original uses, kept because they encode a
// real judgement: the description is written to be searched, so it counts for
// nearly as much as the title, and the body counts for least because it is long
// enough that any word eventually appears in it.
var Weights = map[string]float64{
	"title":       5,
	"description": 4,
	"tags":        3,
	"content":     1,
}

// stopWords are the tokens that should not, on their own, make a query match.
// The list is the original's, unchanged.
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "for": true, "in": true, "is": true,
	"it": true, "of": true, "on": true, "or": true, "the": true, "to": true,
	"with": true, "by": true,
}

// Tokenize splits a query into the terms that must match: lowercase runs of
// letters and digits, with the stop words dropped.
//
// Both the index and the fallback scan call this, which is the point. A query
// that tokenises differently on the two paths would return different results
// depending on whether the index happened to be open.
func Tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" || stopWords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Matches reports whether every token of the query appears somewhere in the
// document's text.
//
// It is an AND over substrings, not a ranked match, because that is what the
// original settled on after its index layer proved unreliable for this: a
// recall that returns things containing only one of three words is worse than
// one that returns nothing, since the caller acts on what comes back.
func Matches(d Document, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	var b strings.Builder
	for _, v := range d.Text {
		b.WriteString(strings.ToLower(v))
		b.WriteByte(' ')
	}
	haystack := b.String()
	for _, tok := range tokens {
		if !strings.Contains(haystack, tok) {
			return false
		}
	}
	return true
}

// Score ranks a document against the query tokens using Weights, so the
// fallback scan orders results the way the index would.
func Score(d Document, tokens []string) float64 {
	var total float64
	for field, text := range d.Text {
		weight, ok := Weights[field]
		if !ok {
			weight = 1
		}
		lower := strings.ToLower(text)
		for _, tok := range tokens {
			total += weight * float64(strings.Count(lower, tok))
		}
	}
	return total
}
