// Package collections is the engine that maps a file on disk to a typed Go
// record and back.
//
// The filesystem is the database of the domain (ADR-0004): a .memory.md with
// YAML front matter inside the user's repository is the record. That is what
// makes the state of an agent versionable in Git, editable by hand and portable
// by copying a directory.
package collections

import (
	"context"
	"sort"
	"time"
)

// Format selects how a record is laid out in its file.
type Format int

const (
	FormatMarkdown Format = iota // YAML front matter + Markdown body
	FormatJSON                   // the whole file is the record (chats, todos)
)

func (f Format) String() string {
	if f == FormatJSON {
		return "json"
	}
	return "markdown"
}

// Model declares how one entity maps to files on disk.
type Model[T any] struct {
	Name     string     // "memories"
	Patterns []*Pattern // compiled from ".aos/agents/{agent}/memories/{id}.memory.md"
	Format   Format

	// CascadeDir returns the directory to remove entirely on delete, relative
	// to the workspace root. Nil means only the record file is removed.
	//
	// The original does this in an onDeleted hook that swallows every error;
	// here it is a declaration, and a failure to remove is reported.
	CascadeDir func(path string) string

	OnCreated func(ctx context.Context, v *T) error
	OnUpdated func(ctx context.Context, old, new *T) error
	OnDeleted func(ctx context.Context, v *T) error
}

// WritePattern returns the pattern used to build a path for a new record: the
// first one without a wildcard. Wildcard patterns (the skill-scoped variants)
// are read-only by construction.
func (m Model[T]) WritePattern() (*Pattern, error) {
	for _, p := range m.Patterns {
		if p.Writable() {
			return p, nil
		}
	}
	return nil, errNoWritablePattern(m.Name)
}

// WritePatternFor picks the most specific writable pattern a key can fill.
//
// A collection with one shape does not need it. One with several does, for
// two different reasons. A run belongs either to a task or to a routine, and
// the two live in different directories with disjoint placeholders — any
// pattern that fills at all is the right one, because only one ever can. A
// skill-scoped collection, view, agent, template or goal is the harder case:
// its workspace pattern ({id}) and its skill-scoped pattern ({skill, id})
// are not disjoint — a key carrying both id and skill satisfies the
// workspace pattern too, trivially, since fills only checks that a pattern's
// own placeholders are present, never that the key carries nothing more.
//
// The first version of this method returned the first writable pattern that
// filled, in declaration order. That was correct for the disjoint case
// (registry.go happens to declare the more specific "runs" patterns before
// the less specific one, so first-match landed right by coincidence) and
// silently wrong for the overlapping case: a skill-scoped agent, collection,
// view, template or goal was always written to its workspace path — the
// first, least specific pattern that could possibly fill — never its own
// skill directory, and read back with Skill empty, because the workspace
// pattern has no {skill} placeholder to populate it from. Nothing caught
// this until a skill could actually be installed and its own agent turned
// out to live at .aos/agents/{id}/AGENT.md instead of inside the skill's
// directory. Picking the pattern whose own field set the key fills most
// completely — the most specific match — is what makes both cases correct
// at once: a key with {id} alone still lands on the workspace pattern
// (nothing more specific fills), and a key with {id, skill} lands on the
// skill-scoped one, because it fills more of what a candidate pattern asks
// for. Falling back to the first writable pattern when nothing fills at all
// keeps that corner — a record with no path fields set — behaving as before.
func (m Model[T]) WritePatternFor(k Key) (*Pattern, error) {
	var best *Pattern
	bestFields := -1
	for _, p := range m.Patterns {
		if !p.Writable() || !fills(p, k) {
			continue
		}
		// Strictly greater, so a tie keeps the earlier-declared pattern: the
		// natives declare the more general shape first everywhere two
		// patterns could ever fill the same number of fields, and this is
		// what keeps that order meaningful rather than incidental.
		if n := len(p.Fields()); n > bestFields {
			best, bestFields = p, n
		}
	}
	if best != nil {
		return best, nil
	}
	return m.WritePattern()
}

// fills reports whether a key has a non-empty value for every placeholder.
func fills(p *Pattern, k Key) bool {
	for _, field := range p.Fields() {
		if k[field] == "" {
			return false
		}
	}
	return true
}

// MatchPath finds the pattern that owns a path and the key it yields.
func (m Model[T]) MatchPath(rel string) (*Pattern, Key, bool) {
	for _, p := range m.Patterns {
		if k, ok := p.Match(rel); ok {
			return p, k, true
		}
	}
	return nil, nil, false
}

// Key holds the placeholder values that identify a record. For memories that is
// {agent, id}; for tasks just {id}.
type Key map[string]string

// Record is the T of a collection whose fields were declared at runtime rather
// than as a Go struct.
//
// Everything the engine does around a record — matching a path to a pattern,
// choosing a writable pattern for a key, the atomic write, the per-file lock,
// the CAS check on Version, the Changed event — looks at the pattern and the
// key, never inside T. That is what lets a collection an agent invented at
// 14:00 be a Model[Record] instead of a second engine.
type Record struct {
	// Key is the placeholder values that identify this record, exactly as for
	// a native: it lives in the path and is never written into the body.
	Key Key

	// Fields is the declared data. internal/domain/collection validates it
	// against the collection's schema before it ever reaches here — the engine
	// stores what it is given.
	Fields map[string]any

	// Content is the Markdown body, for a collection with FormatMarkdown. A
	// FormatJSON collection leaves it empty.
	Content string
}

// Clone returns an independent copy, so a caller cannot mutate a key the engine
// is still holding.
func (k Key) Clone() Key {
	out := make(Key, len(k))
	for a, b := range k {
		out[a] = b
	}
	return out
}

// String renders the key deterministically, for use as a map key and in error
// messages: "agent=luara,id=a1b2".
func (k Key) String() string {
	names := make([]string, 0, len(k))
	for n := range k {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]byte, 0, len(names)*16)
	for i, n := range names {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, n...)
		out = append(out, '=')
		out = append(out, k[n]...)
	}
	return string(out)
}

// Covers reports whether k satisfies every constraint in filter. An empty
// filter matches everything, which is how List without a key lists all records.
func (k Key) Covers(filter Key) bool {
	for name, want := range filter {
		if k[name] != want {
			return false
		}
	}
	return true
}

// Version is the optimistic-concurrency token: modtime plus size, captured on
// read and checked on write. The zero value means "no expectation".
type Version struct {
	ModTime time.Time
	Size    int64
}

// IsZero reports whether the caller expressed no expectation.
func (v Version) IsZero() bool { return v.ModTime.IsZero() && v.Size == 0 }

// Equal compares two versions. Modification times are compared at their stored
// resolution, which differs by filesystem; the size guards the common case of a
// same-second rewrite of different content.
func (v Version) Equal(o Version) bool {
	return v.Size == o.Size && v.ModTime.Equal(o.ModTime)
}

// Query selects records for List.
type Query struct {
	// Key constrains placeholders: {"agent": "luara"} lists one agent's records.
	Key Key

	// Filters match front-matter fields for equality, by their JSON name.
	Filters map[string]any

	// OrderBy names a front-matter field; empty means order by path, which is
	// stable and cheap. Ordering is always total: ties break by path.
	OrderBy string
	Desc    bool

	Limit  int
	Offset int

	// IncludeContent loads the Markdown body of every record. It is off by
	// default: List answers "what is here", Get answers "what does it say".
	IncludeContent bool
}

// Changed is published on the event bus after every successful write.
//
// The json tags are not decoration: this value is the payload of the
// `collection.changed` frame the daemon puts on the WebSocket
// (internal/app's collectionPublisher), so its field names are a wire
// contract the interface reads. Without them it marshalled as Go names —
// `Collection`, `Op`, `Path` — and every consumer on the other side, which
// reads `collection`/`op`/`path`, saw undefined: the file panel filtered an
// always-empty change list, and nothing could tell which collection had
// moved.
type Changed struct {
	Collection string `json:"collection"`
	Key        Key    `json:"key,omitempty"`
	Op         string `json:"op"` // "create" | "update" | "delete"
	Path       string `json:"path,omitempty"`
}

// Publisher is the slice of the event bus this engine needs.
type Publisher interface {
	Publish(ctx context.Context, ev Changed)
}

// NopPublisher drops events. It is the default, so the engine works before the
// bus exists.
type NopPublisher struct{}

func (NopPublisher) Publish(context.Context, Changed) {}
