package collections_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

// TestCloneJSONCopiesNestedMapsAndSlices is the direct test of the shared
// primitive: a one-level clone (a fresh outer map, values copied by
// reference) was tried first, in three different places in this codebase,
// and none of them were enough — a value decoded from JSON or YAML nests
// arbitrarily, and a caller mutating anything below the top level must not
// reach the original.
func TestCloneJSONCopiesNestedMapsAndSlices(t *testing.T) {
	original := map[string]any{
		"name": "Ada",
		"address": map[string]any{
			"city": "London",
		},
		"tags": []any{
			"vip",
			map[string]any{"kind": "founder"},
		},
	}

	cloned, ok := collections.CloneJSON(original).(map[string]any)
	if !ok {
		t.Fatalf("CloneJSON did not return a map[string]any: %T", cloned)
	}

	cloned["address"].(map[string]any)["city"] = "corrupted"
	cloned["tags"].([]any)[1].(map[string]any)["kind"] = "corrupted"
	cloned["tags"].([]any)[0] = "corrupted"

	if got := original["address"].(map[string]any)["city"]; got != "London" {
		t.Fatalf("mutating the clone's nested map reached the original: %v", got)
	}
	if got := original["tags"].([]any)[1].(map[string]any)["kind"]; got != "founder" {
		t.Fatalf("mutating the clone's nested slice element reached the original: %v", got)
	}
	if got := original["tags"].([]any)[0]; got != "vip" {
		t.Fatalf("mutating the clone's slice reached the original: %v", got)
	}
}

// TestCloneJSONHandlesNilAndScalars: a nil map produces an independent empty
// map rather than nil, so a caller that ranges or writes to the clone never
// has to special-case it; a scalar or nil value round-trips unchanged.
func TestCloneJSONHandlesNilAndScalars(t *testing.T) {
	var nilMap map[string]any
	cloned, ok := collections.CloneJSON(nilMap).(map[string]any)
	if !ok || cloned == nil {
		t.Fatalf("CloneJSON(nil map) = %#v, want a non-nil empty map", cloned)
	}
	if len(cloned) != 0 {
		t.Fatalf("CloneJSON(nil map) = %v, want empty", cloned)
	}

	for _, v := range []any{"a string", 42.0, true, nil} {
		if got := collections.CloneJSON(v); got != v {
			t.Fatalf("CloneJSON(%v) = %v, want it unchanged", v, got)
		}
	}
}
