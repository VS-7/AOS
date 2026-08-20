package collections_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

// TestWritePatternForPicksTheMostSpecificPatternAcrossEveryNative is a
// property check over every native that declares more than one writable
// pattern: a key that only fills the least specific pattern's fields must
// select it, and a key that fills the most specific pattern's fields — even
// though it also, trivially, fills every less specific one, since fills only
// checks that a pattern's own placeholders are present, never that the key
// carries nothing more — must select the most specific one instead.
//
// This is the regression the round exists for. WritePatternFor used to
// return the first writable pattern that filled, in declaration order. Every
// native below declares its workspace pattern before its skill-scoped one,
// so a skill-scoped record's key — which carries both id and skill — always
// landed on the workspace pattern, the first one that could possibly fill,
// and never on its own skill directory.
func TestWritePatternForPicksTheMostSpecificPatternAcrossEveryNative(t *testing.T) {
	for _, desc := range collections.Natives() {
		var writable []*collections.Pattern
		for _, p := range desc.Patterns {
			if p.Writable() {
				writable = append(writable, p)
			}
		}
		if len(writable) < 2 {
			continue
		}
		t.Run(desc.Name, func(t *testing.T) {
			model := collections.Model[collections.Record]{Name: desc.Name, Patterns: desc.Patterns}

			sort.Slice(writable, func(i, j int) bool {
				return len(writable[i].Fields()) < len(writable[j].Fields())
			})
			least, most := writable[0], writable[len(writable)-1]

			leastKey := keyFor(least)
			got, err := model.WritePatternFor(leastKey)
			if err != nil {
				t.Fatalf("least-specific key %v: %v", leastKey, err)
			}
			if got.Raw() != least.Raw() {
				t.Fatalf("least-specific key %v selected %q, want %q", leastKey, got.Raw(), least.Raw())
			}

			mostKey := keyFor(most)
			got, err = model.WritePatternFor(mostKey)
			if err != nil {
				t.Fatalf("most-specific key %v: %v", mostKey, err)
			}
			if got.Raw() != most.Raw() {
				t.Fatalf("most-specific key %v selected %q, want %q — the least specific pattern shadowed it",
					mostKey, got.Raw(), most.Raw())
			}

			mostPath := mustBuild(t, most, mostKey)
			leastPath := mustBuild(t, least, leastKey)
			if mostPath == leastPath {
				t.Fatalf("the most-specific and least-specific keys built the same path: %q", mostPath)
			}
		})
	}
}

// TestWritePatternForRunsPicksTheRightShape is the case the original doc
// comment on WritePatternFor was written for: a run belongs either to a
// task, to an agent's routine, or — a third shape the "most specific wins"
// property test above only checks the extremes of — to a routine addressed
// on its own. The agent+routine shape must not be shadowed by the bare
// routine pattern, even though a key carrying agent, routine and id also,
// trivially, fills the bare routine pattern's two fields.
func TestWritePatternForRunsPicksTheRightShape(t *testing.T) {
	desc, ok := collections.Lookup("runs")
	if !ok {
		t.Fatal("no native \"runs\"")
	}
	model := collections.Model[collections.Record]{Name: desc.Name, Patterns: desc.Patterns}

	cases := []struct {
		name string
		key  collections.Key
		want string
	}{
		{"a task's run", collections.Key{"task": "t1", "id": "r1"}, collections.Root + "/tasks/{task}/runs/{id}.run.json"},
		{"an agent's routine's run", collections.Key{"agent": "a1", "routine": "ro1", "id": "r1"},
			collections.Root + "/agents/{agent}/routines/{routine}/runs/{id}.run.json"},
		{"a routine's run, addressed without its agent", collections.Key{"routine": "ro1", "id": "r1"},
			collections.Root + "/routines/{routine}/runs/{id}.run.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := model.WritePatternFor(c.key)
			if err != nil {
				t.Fatal(err)
			}
			if got.Raw() != c.want {
				t.Fatalf("key %v selected %q, want %q", c.key, got.Raw(), c.want)
			}
		})
	}
}

// TestWritePatternForInstructionsPicksTheRightShape covers the native with
// the most writable patterns — four — and the only one where "most fields
// wins" has to break a near-tie: a skill-scoped, untyped key and a
// workspace-scoped, typed key each fill two fields of a different pattern,
// and neither may be confused for the fully-qualified skill-scoped-and-typed
// shape or for each other.
func TestWritePatternForInstructionsPicksTheRightShape(t *testing.T) {
	desc, ok := collections.Lookup("instructions")
	if !ok {
		t.Fatal("no native \"instructions\"")
	}
	model := collections.Model[collections.Record]{Name: desc.Name, Patterns: desc.Patterns}

	cases := []struct {
		name string
		key  collections.Key
		want string
	}{
		{"bare", collections.Key{"id": "x"}, collections.Root + "/instructions/{id}.instruction.md"},
		{"typed", collections.Key{"id": "x", "type": "review"}, collections.Root + "/instructions/{type}/{id}.instruction.md"},
		{"skill-scoped", collections.Key{"id": "x", "skill": "crm"}, collections.Root + "/skills/{skill}/instructions/{id}.instruction.md"},
		{"skill-scoped and typed", collections.Key{"id": "x", "skill": "crm", "type": "review"},
			collections.Root + "/skills/{skill}/instructions/{type}/{id}.instruction.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := model.WritePatternFor(c.key)
			if err != nil {
				t.Fatal(err)
			}
			if got.Raw() != c.want {
				t.Fatalf("key %v selected %q, want %q", c.key, got.Raw(), c.want)
			}
		})
	}
}

// TestASkillScopedRecordWritesInsideTheSkillDirectory is the concrete
// promise this whole phase is named for: installing a skill installs a
// team, which means the team's files actually live inside the skill's own
// directory, not next to the workspace's own agents, collections, views,
// templates, toolsets or goals with the skill left unrecorded.
func TestASkillScopedRecordWritesInsideTheSkillDirectory(t *testing.T) {
	for _, native := range []string{"agents", "templates", "goals", "collections", "views", "toolsets"} {
		t.Run(native, func(t *testing.T) {
			desc, ok := collections.Lookup(native)
			if !ok {
				t.Fatalf("no native %q", native)
			}
			model := collections.Model[collections.Record]{Name: desc.Name, Patterns: desc.Patterns}
			key := collections.Key{"id": "widget", "skill": "crm"}

			p, err := model.WritePatternFor(key)
			if err != nil {
				t.Fatal(err)
			}
			path, err := p.Build(key)
			if err != nil {
				t.Fatal(err)
			}
			want := collections.Root + "/skills/crm/"
			if !strings.HasPrefix(path, want) {
				t.Fatalf("%s: a skill-scoped key built %q, which does not start with %q — it landed outside the skill directory",
					native, path, want)
			}
		})
	}
}

// TestWritePatternForFallsBackWhenNothingFills preserves the one behaviour
// the original code had for a key with no path fields at all: the first
// writable pattern, same as WritePattern itself. Nothing above exercises
// this branch, since every native with more than one writable pattern also
// has at least one whose fields an empty key cannot fill — and every native
// with exactly one, single-shape collections do not go through
// WritePatternFor at all in production, but the method still has to behave
// for one when it is called directly.
func TestWritePatternForFallsBackWhenNothingFills(t *testing.T) {
	desc, ok := collections.Lookup("tasks")
	if !ok {
		t.Fatal("no native \"tasks\"")
	}
	model := collections.Model[collections.Record]{Name: desc.Name, Patterns: desc.Patterns}

	got, err := model.WritePatternFor(collections.Key{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := model.WritePattern()
	if err != nil {
		t.Fatal(err)
	}
	if got.Raw() != want.Raw() {
		t.Fatalf("empty key selected %q, want the fallback %q", got.Raw(), want.Raw())
	}
}

// keyFor builds a synthetic key that fills exactly p's own fields, each with
// a distinct, recognisable value.
func keyFor(p *collections.Pattern) collections.Key {
	k := collections.Key{}
	for _, f := range p.Fields() {
		k[f] = "v-" + f
	}
	return k
}

func mustBuild(t *testing.T, p *collections.Pattern, k collections.Key) string {
	t.Helper()
	path, err := p.Build(k)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
