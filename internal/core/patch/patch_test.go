package patch_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/OWNER/aos/internal/core/patch"
)

type inner struct {
	Prefix string `json:"prefix,omitempty"`
	Force  bool   `json:"force"`
}

type sample struct {
	Name   string            `json:"name"`
	Colour string            `json:"colour,omitempty"`
	Count  int               `json:"count"`
	Git    inner             `json:"git"`
	Tags   []string          `json:"tags,omitempty"`
	Extra  map[string]string `json:"extra,omitempty"`
	Hidden string            `json:"-"`
	unseen string            //nolint:unused // proves unexported fields are skipped
}

func TestApplySetsANestedField(t *testing.T) {
	got, err := patch.Apply(sample{Name: "a"}, map[string]any{"git.prefix": "feat"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Git.Prefix != "feat" {
		t.Fatalf("git.prefix = %q", got.Git.Prefix)
	}
	if got.Name != "a" {
		t.Errorf("an untouched field changed: %q", got.Name)
	}
}

// TestApplyReachesAFieldThatWasOmitted is the defect this package exists to
// avoid: a field with omitempty is absent from the marshalled tree, so a
// patcher that requires the key to be present would refuse to ever set it.
func TestApplyReachesAFieldThatWasOmitted(t *testing.T) {
	got, err := patch.Apply(sample{Name: "a"}, map[string]any{"colour": "#fff"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Colour != "#fff" {
		t.Fatalf("colour = %q, want it set", got.Colour)
	}

	// The same, one level down: git.prefix is omitempty and git itself is
	// present but empty.
	got, err = patch.Apply(sample{}, map[string]any{"git.prefix": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Git.Prefix != "x" {
		t.Fatalf("git.prefix = %q", got.Git.Prefix)
	}
}

func TestApplyDoesNotMutateItsInput(t *testing.T) {
	before := sample{Name: "a", Tags: []string{"one"}, Git: inner{Prefix: "p"}}
	if _, err := patch.Apply(before, map[string]any{
		"name": "b", "tags": []string{"two"}, "git.prefix": "q",
	}); err != nil {
		t.Fatal(err)
	}
	if before.Name != "a" || before.Tags[0] != "one" || before.Git.Prefix != "p" {
		t.Fatalf("Apply mutated its input: %+v", before)
	}
}

func TestUnknownPathIsRejectedAndNamed(t *testing.T) {
	for _, path := range []string{"nope", "git.nope", "name.deeper", "Hidden", "unseen"} {
		_, err := patch.Apply(sample{}, map[string]any{path: "x"})
		var unknown *patch.UnknownPathError
		if !errors.As(err, &unknown) {
			t.Errorf("%s: error = %v, want an UnknownPathError", path, err)
			continue
		}
		if unknown.Path != path {
			t.Errorf("error names %q, want %q", unknown.Path, path)
		}
	}
}

// TestWrongTypeNamesTheField: a patch that sets two fields and gets one of them
// wrong has to say which one, or the caller is left bisecting its own payload.
func TestWrongTypeNamesTheField(t *testing.T) {
	_, err := patch.Apply(sample{}, map[string]any{
		"name":  "fine",
		"count": "not a number",
	})
	var bad *patch.ValueError
	if !errors.As(err, &bad) {
		t.Fatalf("error = %v, want a ValueError", err)
	}
	if bad.Path != "count" {
		t.Fatalf("error names %q, want \"count\"", bad.Path)
	}
}

func TestApplyIsOrderIndependent(t *testing.T) {
	set := map[string]any{"name": "n", "git.force": true, "count": 3, "colour": "#000"}
	first, err := patch.Apply(sample{}, set)
	if err != nil {
		t.Fatal(err)
	}
	// Map iteration order differs between runs; a hundred applications of the
	// same patch must agree.
	for i := 0; i < 100; i++ {
		again, err := patch.Apply(sample{}, set)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(again, first) {
			t.Fatalf("run %d differs: %+v vs %+v", i, again, first)
		}
	}
}

func TestPathsListsTheSettableFields(t *testing.T) {
	got := patch.Paths[sample]()
	want := []string{"colour", "count", "extra", "git", "git.force", "git.prefix", "name", "tags"}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
}

// TestCompositesAreLeaves records the rule rather than discovering it later: a
// slice or map is replaced whole, because an index is not a stable name.
func TestCompositesAreLeaves(t *testing.T) {
	got, err := patch.Apply(sample{Tags: []string{"a", "b"}}, map[string]any{"tags": []string{"c"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "c" {
		t.Fatalf("tags = %v", got.Tags)
	}
	if _, err := patch.Apply(sample{}, map[string]any{"tags.0": "x"}); err == nil {
		t.Error("addressing a slice element must be rejected")
	}
}

type embedded struct {
	Shared string `json:"shared"`
}

type outer struct {
	embedded
	Own string `json:"own"`
}

// TestEmbeddedFieldsInline: the command inputs embed a shared struct, and its
// fields are addressed without a prefix, exactly as they appear on the wire.
func TestEmbeddedFieldsInline(t *testing.T) {
	got, err := patch.Apply(outer{}, map[string]any{"shared": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Shared != "v" {
		t.Fatalf("shared = %q", got.Shared)
	}
	for _, p := range patch.Paths[outer]() {
		if p == "embedded.shared" {
			t.Error("an embedded field must not be addressed through its type name")
		}
	}
}

type node struct {
	Name  string `json:"name"`
	Child *node  `json:"child,omitempty"`
}

// TestRecursiveTypeTerminates: a self-referential type would walk forever
// without the guard, and the type graph of a real domain has one sooner or
// later.
func TestRecursiveTypeTerminates(t *testing.T) {
	paths := patch.Paths[node]()
	if len(paths) == 0 {
		t.Fatal("no paths at all")
	}
	for _, p := range paths {
		if p == "child.child" {
			t.Fatalf("the walk recursed into itself: %v", paths)
		}
	}
}
