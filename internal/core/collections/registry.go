package collections

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/OWNER/aos/internal/core/build"
)

var errNotAStruct = errors.New("a collection record must be a struct")

// Root is the per-workspace state directory, ".aos" — the equivalent of the
// original's ".fractal". Every native pattern is relative to the workspace root
// and starts here.
var Root = build.StateDir

// Descriptor is the type-erased declaration of a native collection: its name,
// where its records live and how they are laid out. Entities bind to it in
// their own phase; the paths are knowledge of the engine and are declared here
// once, so that two features cannot disagree about where a record lives.
type Descriptor struct {
	Name     string
	Patterns []*Pattern
	Format   Format

	// CascadeDelete removes the record's whole directory rather than the file
	// alone. It is true for every collection whose record is the index file of
	// a directory (TASK.md, SKILL.md, AGENT.md, ROUTINE.md, …).
	CascadeDelete bool
}

func d(name string, format Format, cascade bool, patterns ...string) Descriptor {
	compiled := make([]*Pattern, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, MustCompile(p))
	}
	return Descriptor{Name: name, Patterns: compiled, Format: format, CascadeDelete: cascade}
}

// The native collections, with the patterns of the original adapted from
// ".fractal/" to ".aos/". Nothing else about them changed: a user who knows
// where a record lives in one product finds it in the other.
var natives = []Descriptor{
	d("agents", FormatMarkdown, true,
		Root+"/agents/{id}/AGENT.md",
		Root+"/skills/{skill}/agents/{id}/AGENT.md",
	),
	d("skills", FormatMarkdown, true,
		Root+"/skills/{id}/SKILL.md",
	),
	// The second pattern is what lets a skill ship its own agents' memories.
	// Without it, installing a skill does not install a team.
	d("memories", FormatMarkdown, false,
		Root+"/agents/{agent}/memories/{id}.memory.md",
		Root+"/skills/*/agents/{agent}/memories/{id}.memory.md",
	),
	d("templates", FormatMarkdown, false,
		Root+"/templates/{id}.template.md",
		Root+"/skills/{skill}/templates/{id}.template.md",
	),
	d("instructions", FormatMarkdown, false,
		Root+"/instructions/{id}.instruction.md",
		Root+"/instructions/{type}/{id}.instruction.md",
		Root+"/skills/{skill}/instructions/{id}.instruction.md",
		Root+"/skills/{skill}/instructions/{type}/{id}.instruction.md",
	),
	d("tasks", FormatMarkdown, true,
		Root+"/tasks/{id}/TASK.md",
	),
	// The original stores both of these as JSON. They are Markdown here, and
	// the reason is the one ADR-0004 gives for the whole persistence model: a
	// comment body is prose, and prose in a JSON string is a single line with
	// escaped newlines — unreadable in a diff and impossible to edit by hand.
	// A todo follows its sibling so the two subcollections stay consistent.
	d("todos", FormatMarkdown, false,
		Root+"/tasks/{taskId}/todos/{id}.todo.md",
	),
	d("comments", FormatMarkdown, false,
		Root+"/tasks/{taskId}/comments/{id}.comment.md",
	),
	d("chats", FormatJSON, false,
		Root+"/chats/{id}.chat.json",
	),
	d("routines", FormatMarkdown, true,
		Root+"/agents/{agent}/routines/{id}/ROUTINE.md",
		Root+"/skills/*/agents/{agent}/routines/{id}/ROUTINE.md",
	),
	// Runs are the chat transcripts of an autonomous execution. The original
	// folds them into the chats collection; they are separate here because
	// they have their own lifecycle and their own retention.
	d("runs", FormatJSON, false,
		Root+"/tasks/{task}/runs/{id}.run.json",
		Root+"/agents/{agent}/routines/{routine}/runs/{id}.run.json",
		Root+"/routines/{routine}/runs/{id}.run.json",
		Root+"/skills/*/agents/{agent}/routines/{routine}/runs/{id}.run.json",
	),
	d("projects", FormatMarkdown, true,
		Root+"/projects/{id}/PROJECT.md",
	),
	d("goals", FormatMarkdown, true,
		Root+"/goals/{id}/GOAL.md",
		Root+"/skills/{skill}/goals/{id}/GOAL.md",
	),
}

// Natives returns every native collection descriptor, in a stable order.
func Natives() []Descriptor {
	out := make([]Descriptor, len(natives))
	copy(out, natives)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Registry holds what collections exist.
//
// The natives are fixed at construction. The dynamic ones come and go while
// the daemon runs, which is the entire point of a collection an agent can
// create: the original's watcher loads a schema the moment it appears, and a
// collection you have to restart to use is a collection the agent cannot use
// in the turn that created it.
//
// It is an instance rather than mutable package state because it is read by
// whatever is running and written by the watcher — an unguarded package map
// races on the first turn that creates a collection while a list is in flight.
type Registry struct {
	mu      sync.RWMutex
	natives map[string]Descriptor
	dynamic map[string]Descriptor
}

// NewRegistry returns a registry holding the native collections and nothing
// else.
func NewRegistry() *Registry {
	r := &Registry{
		natives: make(map[string]Descriptor, len(natives)),
		dynamic: map[string]Descriptor{},
	}
	for _, d := range natives {
		if _, dup := r.natives[d.Name]; dup {
			panic(fmt.Sprintf("collections: %q registered twice", d.Name))
		}
		r.natives[d.Name] = d
	}
	return r
}

// Register adds or replaces a dynamic collection.
//
// Replacing rather than refusing a duplicate is deliberate: uninstalling and
// reinstalling a skill re-registers the collections it ships, and a registry
// that refused the second registration would leave a workspace unable to
// reinstall what it had just removed.
func (r *Registry) Register(d Descriptor) error {
	if d.Name == "" {
		return errCollectionNameEmpty()
	}
	if len(d.Patterns) == 0 {
		return errCollectionNoPatterns(d.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, native := r.natives[d.Name]; native {
		return errCollectionNameReserved(d.Name)
	}
	r.dynamic[d.Name] = d
	return nil
}

// Unregister removes a dynamic collection. A native name is refused: the
// engine's own collections are not the caller's to remove, and letting one go
// would break every domain built on it.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, native := r.natives[name]; native {
		return errCollectionNativeNotRemovable(name)
	}
	delete(r.dynamic, name)
	return nil
}

// Lookup returns a descriptor by name, native or dynamic.
func (r *Registry) Lookup(name string) (Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if d, ok := r.natives[name]; ok {
		return d, true
	}
	d, ok := r.dynamic[name]
	return d, ok
}

// IsNative reports whether a name belongs to the engine rather than to a
// collection somebody declared.
func (r *Registry) IsNative(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.natives[name]
	return ok
}

// Names returns every collection name, natives first, each group sorted, so a
// listing is stable between calls and between machines.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.natives)+len(r.dynamic))
	for name := range r.natives {
		out = append(out, name)
	}
	sort.Strings(out)
	dyn := make([]string, 0, len(r.dynamic))
	for name := range r.dynamic {
		dyn = append(dyn, name)
	}
	sort.Strings(dyn)
	return append(out, dyn...)
}

// defaultRegistry is what the package-level Lookup, Natives and ModelOf read.
// It exists so the fourteen domains written before this type do not change a
// line: they ask the package, the package asks the default instance.
var defaultRegistry = NewRegistry()

// Default returns the process-wide registry. A caller that needs to register a
// dynamic collection takes this, or is given its own instance — the wiring in
// internal/app passes one explicitly.
func Default() *Registry { return defaultRegistry }

// Lookup returns the descriptor of a collection, native or dynamic.
func Lookup(name string) (Descriptor, bool) { return defaultRegistry.Lookup(name) }

// ModelOf binds an entity type to a descriptor, producing the typed model the
// repository works with. It is the single place where "where does a memory
// live" meets "what is a memory".
func ModelOf[T any](name string) (Model[T], error) {
	desc, ok := defaultRegistry.Lookup(name)
	if !ok {
		return Model[T]{}, errUnknownCollection(name)
	}
	m := Model[T]{Name: desc.Name, Patterns: desc.Patterns, Format: desc.Format}
	if desc.CascadeDelete {
		m.CascadeDir = parentDir
	}
	return m, nil
}

// parentDir is the cascade rule of every directory-backed collection: deleting
// TASK.md removes the task directory with its todos, comments and runs.
func parentDir(recordPath string) string { return dirOf(recordPath) }

// WorkspaceDirs lists the directories a fresh workspace is scaffolded with,
// derived from the patterns rather than written out a second time.
func WorkspaceDirs() []string {
	seen := map[string]bool{}
	var out []string
	for _, desc := range natives {
		for _, p := range desc.Patterns {
			prefix := p.Prefix()
			if prefix == "" || seen[prefix] {
				continue
			}
			seen[prefix] = true
			out = append(out, prefix)
		}
	}
	sort.Strings(out)
	return out
}
