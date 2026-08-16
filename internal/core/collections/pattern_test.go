package collections_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
)

func TestMatchExtractsPlaceholders(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    collections.Key
	}{
		{
			".aos/agents/{agent}/memories/{id}.memory.md",
			".aos/agents/luara/memories/a1b2c3.memory.md",
			collections.Key{"agent": "luara", "id": "a1b2c3"},
		},
		{
			".aos/agents/{id}/AGENT.md",
			".aos/agents/orchestrator/AGENT.md",
			collections.Key{"id": "orchestrator"},
		},
		{
			".aos/tasks/{taskId}/todos/{id}.todo.md",
			".aos/tasks/t-1/todos/td-9.todo.md",
			collections.Key{"taskId": "t-1", "id": "td-9"},
		},
		{
			".aos/instructions/{type}/{id}.instruction.md",
			".aos/instructions/review/style.instruction.md",
			collections.Key{"type": "review", "id": "style"},
		},
		{
			// A dotted identifier is captured whole: the literal suffix anchors
			// the match at the end, so greediness cannot eat ".memory".
			".aos/agents/{agent}/memories/{id}.memory.md",
			".aos/agents/luara/memories/2026.08.15.memory.md",
			collections.Key{"agent": "luara", "id": "2026.08.15"},
		},
	}
	for _, c := range cases {
		p, err := collections.Compile(c.pattern)
		if err != nil {
			t.Fatalf("%s: %v", c.pattern, err)
		}
		got, ok := p.Match(c.path)
		if !ok {
			t.Errorf("%s did not match %s", c.pattern, c.path)
			continue
		}
		if got.String() != c.want.String() {
			t.Errorf("%s → %s, want %s", c.path, got, c.want)
		}
	}
}

// TestSkillScopedPatternMatchesAndCapturesNothingForTheStar is the property
// that lets a skill ship its own agents: the wildcard swallows the skill id and
// the record still belongs to the same collection.
func TestSkillScopedPatternMatchesAndCapturesNothingForTheStar(t *testing.T) {
	p, err := collections.Compile(".aos/skills/*/agents/{agent}/memories/{id}.memory.md")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p.Match(".aos/skills/github-flow/agents/luara/memories/a.memory.md")
	if !ok {
		t.Fatal("skill-scoped path did not match")
	}
	if got["agent"] != "luara" || got["id"] != "a" {
		t.Fatalf("key = %s", got)
	}
	if _, extra := got["skill"]; extra {
		t.Error("the star must capture nothing")
	}
	if p.Writable() {
		t.Error("a wildcard pattern cannot build a path and must not claim it can")
	}
}

func TestMatchRejectsPathsOfAnotherCollection(t *testing.T) {
	p := collections.MustCompile(".aos/agents/{agent}/memories/{id}.memory.md")
	for _, path := range []string{
		".aos/agents/luara/AGENT.md",
		".aos/agents/luara/memories/a/b.memory.md", // a placeholder is one element
		".aos/tasks/t-1/TASK.md",
		"memories/a.memory.md",
		".aos/agents/luara/memories/a.memory.md.bak",
	} {
		if _, ok := p.Match(path); ok {
			t.Errorf("%q must not match", path)
		}
	}
}

// TestBuildIsTheInverseOfMatch is the round-trip the whole design rests on.
func TestBuildIsTheInverseOfMatch(t *testing.T) {
	for _, desc := range collections.Natives() {
		for _, p := range desc.Patterns {
			if !p.Writable() {
				continue
			}
			key := collections.Key{}
			for _, f := range p.Fields() {
				key[f] = "sample-" + f
			}
			path, err := p.Build(key)
			if err != nil {
				t.Errorf("%s: build: %v", p.Raw(), err)
				continue
			}
			back, ok := p.Match(path)
			if !ok {
				t.Errorf("%s: built %q does not match its own pattern", p.Raw(), path)
				continue
			}
			if back.String() != key.String() {
				t.Errorf("%s: round trip lost data: %s → %q → %s", p.Raw(), key, path, back)
			}
		}
	}
}

// TestBuildFailsOnMissingPlaceholder closes an entire class of bug: writing a
// record to a partially resolved path.
func TestBuildFailsOnMissingPlaceholder(t *testing.T) {
	p := collections.MustCompile(".aos/agents/{agent}/memories/{id}.memory.md")
	_, err := p.Build(collections.Key{"agent": "luara"})
	if err == nil {
		t.Fatal("build with a missing placeholder must fail")
	}
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	e, _ := apperr.As(err)
	if e == nil || len(e.Actions) == 0 {
		t.Error("the error must say which placeholders are required")
	}
	if !strings.Contains(e.Message, "id") {
		t.Errorf("the message must name the missing placeholder: %q", e.Message)
	}
}

func TestBuildRejectsAPathSeparatorInAValue(t *testing.T) {
	p := collections.MustCompile(".aos/agents/{agent}/memories/{id}.memory.md")
	_, err := p.Build(collections.Key{"agent": "luara", "id": "../../etc/passwd"})
	if err == nil {
		t.Fatal("a key with a separator must be refused")
	}
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildRefusesAWildcardPattern(t *testing.T) {
	p := collections.MustCompile(".aos/skills/*/agents/{agent}/memories/{id}.memory.md")
	if _, err := p.Build(collections.Key{"agent": "a", "id": "b"}); err == nil {
		t.Fatal("a wildcard pattern has nothing to put back where the star was")
	}
}

func TestCompileRejectsMalformedPatterns(t *testing.T) {
	for _, raw := range []string{
		".aos/agents/{agent/AGENT.md", // unterminated
		".aos/agents/{}/AGENT.md",     // empty
		".aos/agents/{a/b}/AGENT.md",  // separator inside a placeholder
		".aos/{id}/{id}.md",           // duplicated
	} {
		if _, err := collections.Compile(raw); err == nil {
			t.Errorf("%q should not compile", raw)
		}
	}
}

func TestGlobAndPrefixKeepTheWalkNarrow(t *testing.T) {
	p := collections.MustCompile(".aos/agents/{agent}/memories/{id}.memory.md")
	if got, want := p.Glob(), ".aos/agents/*/memories/*.memory.md"; got != want {
		t.Errorf("glob = %q, want %q", got, want)
	}
	if got, want := p.Prefix(), ".aos/agents"; got != want {
		t.Errorf("prefix = %q, want %q", got, want)
	}
	// A pattern with no placeholder still yields its directory.
	if got := collections.MustCompile(".aos/activity.json").Prefix(); got != ".aos" {
		t.Errorf("prefix = %q", got)
	}
}

func TestDoubleStarMatchesAcrossDirectories(t *testing.T) {
	p := collections.MustCompile(".aos/collections/**/{id}.json")
	if _, ok := p.Match(".aos/collections/a/b/c/rec.json"); !ok {
		t.Error("** must cross directory boundaries")
	}
}
