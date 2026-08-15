package collections

import (
	"errors"
	"fmt"
	"sort"

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
	d("todos", FormatJSON, false,
		Root+"/tasks/{taskId}/todos/{id}.json",
	),
	d("comments", FormatJSON, false,
		Root+"/tasks/{taskId}/comments/{id}.json",
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

var byName = func() map[string]Descriptor {
	m := make(map[string]Descriptor, len(natives))
	for _, desc := range natives {
		if _, dup := m[desc.Name]; dup {
			panic(fmt.Sprintf("collections: %q registered twice", desc.Name))
		}
		m[desc.Name] = desc
	}
	return m
}()

// Natives returns every native collection descriptor, in a stable order.
func Natives() []Descriptor {
	out := make([]Descriptor, len(natives))
	copy(out, natives)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup returns the descriptor of a native collection.
func Lookup(name string) (Descriptor, bool) {
	desc, ok := byName[name]
	return desc, ok
}

// ModelOf binds an entity type to a native descriptor, producing the typed
// model the repository works with. It is the single place where "where does a
// memory live" meets "what is a memory".
func ModelOf[T any](name string) (Model[T], error) {
	desc, ok := byName[name]
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
