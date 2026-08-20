package view

import (
	"fmt"

	"github.com/OWNER/aos/internal/domain/collection"
)

// Validate descends the whole tree of v before it is ever written. This is
// the divergence the domain exists for: the original validates while
// rendering, so a mistake in a prop or a bind surfaces as a blank screen only
// once somebody opens it; here it is refused at write time, and the refusal
// names the component, the prop, and the path to the node, so the agent that
// just wrote the view can fix it in the same turn.
func Validate(v View, c collection.Collection, cmds Commands) error {
	return validateNode(v.Tree, c, cmds, "tree")
}

// validateNode checks one node and then its children, in an order the tests
// pin down: the children-acceptance check runs before the required-prop
// check, not after. A leaf component's own required props (Badge needs a
// "text") would otherwise mask the fact that it was never allowed to have
// children in the first place — the wrong one of two true refusals to report
// first.
func validateNode(n Node, c collection.Collection, cmds Commands, path string) error {
	spec, ok := LookupComponent(n.Component)
	if !ok {
		return errComponentUnknown(path, n.Component)
	}

	if len(n.Children) > 0 && !spec.AcceptsChildren {
		return errChildrenNotAccepted(path, n.Component)
	}

	properties, _ := spec.Props["properties"].(map[string]any)
	required, _ := spec.Props["required"].([]any)

	if err := checkRequiredProps(n, properties, required, path); err != nil {
		return err
	}
	if err := checkPropTypes(n, properties, path); err != nil {
		return err
	}
	if err := checkBinds(n, c, path); err != nil {
		return err
	}
	for _, a := range n.Actions {
		if !cmds.Has(a.Command) {
			return errActionCommandUnknown(path, a.Command)
		}
	}

	for i, child := range n.Children {
		if err := validateNode(child, c, cmds, fmt.Sprintf("%s.children[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

// checkRequiredProps refuses only a prop that is genuinely mandatory: one
// the catalog lists as required and whose schema does not admit null. The
// catalog is generated from zod, where an optional or nullable prop still
// appears in JSON Schema's "required" array — that array names every key the
// generator emitted, not every key a caller must supply — with null added to
// its type; the two are told apart by whether null is an admissible type, not
// by presence in "required" alone. A prop supplied via Bind counts as given,
// since its value is resolved from the record when the view renders, not at
// write time.
func checkRequiredProps(n Node, properties map[string]any, required []any, path string) error {
	for _, r := range required {
		name, _ := r.(string)
		if name == "" {
			continue
		}
		if _, given := n.Props[name]; given {
			continue
		}
		if _, bound := n.Bind[name]; bound {
			continue
		}
		propSchema, _ := properties[name].(map[string]any)
		if allowsNull(propSchema) {
			continue
		}
		return errPropRequired(path, n.Component, name)
	}
	return nil
}

// checkPropTypes checks every prop the node actually gives against the type
// the catalog declares for it. A prop the component does not declare at all
// is not this check's concern — additionalProperties is a schema detail the
// frontend enforces at render time, not one of the five refusals this domain
// promises.
func checkPropTypes(n Node, properties map[string]any, path string) error {
	for name, val := range n.Props {
		propSchema, ok := properties[name].(map[string]any)
		if !ok {
			continue
		}
		types := schemaTypes(propSchema)
		if len(types) == 0 || matchesAny(val, types) {
			continue
		}
		return errPropWrongType(path, n.Component, name, primaryType(types))
	}
	return nil
}

// checkBinds refuses a bind that points at a field the source collection
// never declared: rendered against nothing, it would show nothing, silently.
func checkBinds(n Node, c collection.Collection, path string) error {
	for _, field := range n.Bind {
		if !hasField(c, field) {
			return errBindUnknownField(path, field, c.ID)
		}
	}
	return nil
}

func hasField(c collection.Collection, name string) bool {
	for _, f := range c.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// schemaTypes reads the admissible JSON Schema types out of one prop's
// schema, following either a "type" keyword (a string or an array of them) or
// an "anyOf" of alternatives — the shape zod-to-json-schema emits for an
// optional or nullable field: {anyOf: [{type: "string", enum: [...]}, {type:
// "null"}]}.
func schemaTypes(propSchema map[string]any) []string {
	if propSchema == nil {
		return nil
	}
	if t, ok := propSchema["type"]; ok {
		return typeNames(t)
	}
	if anyOf, ok := propSchema["anyOf"].([]any); ok {
		var out []string
		for _, item := range anyOf {
			if m, ok := item.(map[string]any); ok {
				out = append(out, schemaTypes(m)...)
			}
		}
		return out
	}
	return nil
}

func typeNames(t any) []string {
	switch v := t.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// allowsNull reports whether a prop's schema admits null, which is how a zod
// .optional()/.nullable() field is told apart from a genuinely required one
// once both have already been emitted into JSON Schema's "required" array.
func allowsNull(propSchema map[string]any) bool {
	for _, t := range schemaTypes(propSchema) {
		if t == "null" {
			return true
		}
	}
	return false
}

// primaryType names the type an error message should blame: the first
// non-null entry, since "null" in the list means the prop may be omitted, not
// that null is the shape a caller who does supply it should send.
func primaryType(types []string) string {
	for _, t := range types {
		if t != "null" {
			return t
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return "unknown"
}

func matchesAny(val any, types []string) bool {
	for _, t := range types {
		if matchesType(val, t) {
			return true
		}
	}
	return false
}

// matchesType checks a Go value built either by decoding JSON (where a number
// is always float64) or by a caller constructing a Node directly in Go (where
// map[string]any{"n": 7} holds an int) — both are values this domain's own
// tests and its future command layer produce, and neither should be refused
// for a JSON Schema type its own encoding satisfies.
func matchesType(val any, t string) bool {
	switch t {
	case "string":
		_, ok := val.(string)
		return ok
	case "number", "integer":
		switch val.(type) {
		case float64, float32, int, int32, int64, uint, uint32, uint64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "array":
		_, ok := val.([]any)
		return ok
	case "object":
		_, ok := val.(map[string]any)
		return ok
	case "null":
		return val == nil
	default:
		// A schema keyword this validator does not model (e.g. a component
		// that declares a prop with no "type") is not refused for it: the
		// catalog, not this switch, is the source of truth for what a
		// component accepts.
		return true
	}
}
