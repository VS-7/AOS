// Package patch applies partial updates addressed by dotted path.
//
// Two aggregates already need it — the installation configuration and the
// workspace record — and both need the same three properties: the caller
// writes the path it sees in the JSON, an unknown path is rejected rather than
// silently added, and a value of the wrong type is reported against the field
// that could not take it. Two implementations of that would eventually disagree
// about one of the three.
package patch

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// UnknownPathError reports a path that the target type does not have.
type UnknownPathError struct{ Path string }

func (e *UnknownPathError) Error() string { return "unknown field " + strconv.Quote(e.Path) }

// ValueError reports a value the target field could not take.
type ValueError struct {
	Path  string
	Cause error
}

func (e *ValueError) Error() string {
	return "field " + strconv.Quote(e.Path) + " cannot take that value: " + e.Cause.Error()
}

func (e *ValueError) Unwrap() error { return e.Cause }

// Apply returns a copy of v with every dotted path in set applied.
//
// Paths are applied in sorted order, so that a patch touching two fields
// produces the same result whatever order the caller's map iterates in. Each
// one is validated and decoded on its own, which is what lets the error name
// the offending field rather than the whole payload.
func Apply[T any](v T, set map[string]any) (T, error) {
	var zero T
	allowed := Paths[T]()
	index := make(map[string]bool, len(allowed))
	for _, p := range allowed {
		index[p] = true
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return zero, err
	}
	tree := map[string]any{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return zero, err
	}

	out := v
	for _, path := range sortedKeys(set) {
		if !index[path] {
			return zero, &UnknownPathError{Path: path}
		}
		setPath(tree, strings.Split(path, "."), set[path])

		merged, err := json.Marshal(tree)
		if err != nil {
			return zero, &ValueError{Path: path, Cause: err}
		}
		var next T
		if err := json.Unmarshal(merged, &next); err != nil {
			return zero, &ValueError{Path: path, Cause: err}
		}
		out = next
	}
	return out, nil
}

// Paths returns every dotted path that Apply accepts for T, sorted.
//
// It is what a command publishes so that a caller — and a model reading the
// tool description — can see the field names without guessing them from a
// sample record. Composite values (slices, maps) are leaves: a caller replaces
// the whole list rather than addressing an element, because an index is not a
// stable name for a thing.
func Paths[T any]() []string {
	var zero T
	out := collect(reflect.TypeOf(zero), "", map[reflect.Type]bool{})
	sort.Strings(out)
	return out
}

func collect(t reflect.Type, prefix string, seen map[reflect.Type]bool) []string {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct || seen[t] {
		return nil
	}
	// A type that contains itself would otherwise walk forever. The guard is
	// scoped to one branch of the walk, not to the whole call, so two sibling
	// fields of the same type both get their paths.
	seen[t] = true
	defer delete(seen, t)

	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, ok := jsonName(f)
		if !ok {
			continue
		}
		if f.Anonymous && name == "" {
			out = append(out, collect(f.Type, prefix, seen)...)
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		out = append(out, path)
		out = append(out, collect(f.Type, path, seen)...)
	}
	return out
}

// jsonName reports the wire name of a field and whether it is written at all.
//
// The rule for anonymous fields is encoding/json's, not the obvious one: an
// embedded struct type promotes its exported fields even when the type itself
// is unexported, so PkgPath alone would wrongly hide them.
func jsonName(f reflect.StructField) (string, bool) {
	if f.PkgPath != "" && (!f.Anonymous || !structKind(f.Type)) {
		return "", false // unexported
	}
	tag, tagged := f.Tag.Lookup("json")
	if !tagged {
		if f.Anonymous {
			return "", true // embedded struct, fields inline
		}
		return f.Name, true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false
	}
	if name == "" {
		if f.Anonymous {
			return "", true
		}
		return f.Name, true
	}
	return name, true
}

// structKind reports whether t is a struct, or a pointer to one.
func structKind(t reflect.Type) bool {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t != nil && t.Kind() == reflect.Struct
}

// setPath writes value at parts, creating the intermediate objects a field with
// omitempty left out of the tree. The path is known to exist on the type by the
// time this runs, so a missing node means "unset", not "wrong".
func setPath(tree map[string]any, parts []string, value any) {
	head := parts[0]
	if len(parts) == 1 {
		tree[head] = value
		return
	}
	child, ok := tree[head].(map[string]any)
	if !ok {
		child = map[string]any{}
		tree[head] = child
	}
	setPath(child, parts[1:], value)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
