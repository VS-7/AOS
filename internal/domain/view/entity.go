package view

import "time"

// Kind is what a view renders as. It is a hint the frontend chrome reads —
// which layout shell to wrap the tree in — not a constraint the tree itself
// enforces.
type Kind string

const (
	KindTable  Kind = "table"
	KindBoard  Kind = "board"
	KindDetail Kind = "detail"
)

// SortSpec orders a Source's rows by one field.
type SortSpec struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

// Source is where a view's data comes from: a collection, filtered, sorted,
// and capped, resolved by the collection domain this task does not yet import
// (see port.go).
type Source struct {
	Collection string         `json:"collection"`
	Filter     map[string]any `json:"filter,omitempty"`
	Sort       []SortSpec     `json:"sort,omitempty"`
	Limit      int            `json:"limit,omitempty"`
}

// Action is a button in a view. It does not mutate anything itself — it names
// a Descriptor in the command registry, with Input as the (possibly partial)
// arguments to invoke it with.
//
// This is deliberate: a view is a description of a screen, not a second way
// to write data. Routing every mutation through the same command registry
// means an action gets the same input validation and the same authorisation
// as a CLI invocation or an MCP tool call — there is no shortcut that skips
// either because it was clicked instead of typed.
type Action struct {
	Label   string         `json:"label"`
	Command string         `json:"command"`
	Input   map[string]any `json:"input,omitempty"`
	Confirm bool           `json:"confirm,omitempty"`
}

// Node is one element of a view's tree: a component from the catalog, its
// props, bindings from Source data into those props, and its children.
type Node struct {
	Component string            `json:"component"`
	Props     map[string]any    `json:"props,omitempty"`
	Bind      map[string]string `json:"bind,omitempty"`
	Children  []Node            `json:"children,omitempty"`
	Actions   []Action          `json:"actions,omitempty"`
}

// View is a screen an agent composed: a tree of catalog components, bound to
// a data source, stored as data so the frontend renders it without a build or
// a deploy.
type View struct {
	// ID and Skill come from the path: a view lives at
	// .aos/views/{skill}/{id}.view.json when scoped to a skill.
	ID   string `json:"id" collection:"path"`
	Name string `json:"name"`

	Title       string `json:"title"`
	Description string `json:"description,omitempty"`

	// Scope says who a view is for: e.g. "user" or "skill". It is not a Kind —
	// Kind is how the view renders, Scope is who owns it.
	Scope string `json:"scope"`

	// Skill is set when a view belongs to a skill rather than to a user
	// directly, which is what lets a skill ship its own screens.
	Skill string `json:"skill,omitempty" collection:"path=skill"`

	Source Source `json:"source"`
	Tree   Node   `json:"tree"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
