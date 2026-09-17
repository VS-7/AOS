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
	seen := map[string]bool{"Stack": true, "Table": true, "Grid": true, "Card": true, "Stat": true}
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
// how to lay out many rows. KindBoard and KindDetail have no such component,
// so each record becomes a Card: the Card is the node that binds, and the
// frontend repeats the lowest node holding every binding once per record — so
// one Card per record, not one run of every record's values interleaved.
//
// The two differ in what a person reads them for. A board is scanned: a grid
// of compact cards, titled by the record's name, each field in the component
// its type suggests. A detail sheet is read: one card per row of the page,
// every field a labelled row, so a value is never shown without its name.
func scaffoldTree(c collection.Collection, kind Kind) Node {
	switch kind {
	case KindBoard:
		title := titleField(c)
		children := make([]Node, 0, len(c.Fields))
		for _, f := range c.Fields {
			if f.Name == title {
				continue
			}
			component := scaffoldFor[f.Type]
			if component == "" {
				component = "Text"
			}
			children = append(children, fieldNode(component, f))
		}
		card := Node{Component: "Card", Props: map[string]any{"density": "compact"}, Children: children}
		bindTitle(&card, title)
		return Node{Component: "Grid", Props: map[string]any{"columns": 3, "gap": "md"}, Children: []Node{card}}

	case KindDetail:
		rows := make([]Node, 0, len(c.Fields))
		for _, f := range c.Fields {
			rows = append(rows, Node{
				Component: "Stat",
				Props:     map[string]any{"label": f.Name, "variant": "row"},
				Bind:      map[string]string{"value": f.Name},
			})
		}
		card := Node{Component: "Card", Children: rows}
		bindTitle(&card, titleField(c))
		return Node{Component: "Stack", Props: map[string]any{"gap": "lg"}, Children: []Node{card}}
	}

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

// titleField is the field a record is called by: the first declared string,
// which for the collections agents write ("name", "title") is the one a person
// would read first. Empty when the collection declares no string field.
func titleField(c collection.Collection) string {
	for _, f := range c.Fields {
		if f.Type == collection.TypeString {
			return f.Name
		}
	}
	return ""
}

func bindTitle(card *Node, field string) {
	if field != "" {
		card.Bind = map[string]string{"title": field}
	}
}
