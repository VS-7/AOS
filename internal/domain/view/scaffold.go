package view

import (
	"sort"

	"github.com/OWNER/aos/internal/domain/collection"
)

// scaffoldFor maps a declared field type to the component that shows it. It
// is deliberately conservative: a scaffold that renders is worth more than
// one that is clever, and the agent edits what it gets.
var scaffoldFor = map[collection.FieldType]string{
	collection.TypeString:  "Text",
	collection.TypeNumber:  "Stat",
	collection.TypeBoolean: "Badge",
	collection.TypeDate:    "Text",
	collection.TypeEnum:    "Badge",
	collection.TypeRef:     "Link",
	collection.TypeList:    "Text",
}

// ScaffoldComponents returns every component name Scaffold can emit: the two
// containers it composes with (Stack, Table) plus every value scaffoldFor
// maps to. A test checks each one still exists in the generated catalog —
// the drift a hand-written map and an embedded JSON file can develop without
// the compiler ever seeing it, since neither references the other's symbols.
func ScaffoldComponents() []string {
	seen := map[string]bool{"Stack": true, "Table": true}
	for _, name := range scaffoldFor {
		seen[name] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// fieldNode builds the node that shows one field, binding whichever prop
// carries the record's value to the field's own name and filling any other
// prop the component requires literally — Stat and Link both need a label
// the record does not have, so the field's name stands in for it.
func fieldNode(component string, f collection.Field) Node {
	switch component {
	case "Stat":
		return Node{Component: component, Props: map[string]any{"label": f.Name}, Bind: map[string]string{"value": f.Name}}
	case "Link":
		return Node{Component: component, Props: map[string]any{"label": f.Name}, Bind: map[string]string{"href": f.Name}}
	default: // Text, Badge: a single required prop, bound straight to the field.
		return Node{Component: component, Bind: map[string]string{"text": f.Name}}
	}
}

// scaffoldTree composes the view's tree for one collection and Kind.
//
// KindTable renders through the Table component itself, which already knows
// how to lay out many rows; KindBoard and KindDetail have no such component
// in the catalog, so they compose one field node per declared field inside a
// Stack — again, conservative over clever.
func scaffoldTree(c collection.Collection, kind Kind) Node {
	if kind == KindTable {
		columns := make([]any, 0, len(c.Fields))
		for _, f := range c.Fields {
			columns = append(columns, f.Name)
		}
		return Node{
			Component: "Table",
			Props: map[string]any{
				"columns": columns,
				"rows":    []any{},
			},
		}
	}

	children := make([]Node, 0, len(c.Fields))
	for _, f := range c.Fields {
		component := scaffoldFor[f.Type]
		if component == "" {
			component = "Text"
		}
		children = append(children, fieldNode(component, f))
	}
	return Node{Component: "Stack", Children: children}
}
