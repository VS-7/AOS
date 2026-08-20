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
	Field string `json:"field" jsonschema:"Name of a declared field of the source collection to order by."`
	Desc  bool   `json:"desc,omitempty" jsonschema:"Reverse the order. False sorts ascending."`
}

// Source is where a view's data comes from: a collection, filtered, sorted,
// and capped, resolved by the collection domain this task does not yet import
// (see port.go).
type Source struct {
	Collection string         `json:"collection" jsonschema:"Id of the collection this view's data comes from."`
	Filter     map[string]any `json:"filter,omitempty" jsonschema:"Field values a record must match, by field name."`
	Sort       []SortSpec     `json:"sort,omitempty" jsonschema:"Fields to order the rows by, applied in order."`
	Limit      int            `json:"limit,omitempty" jsonschema:"Maximum number of records to render. 0 means no limit."`
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
	Label   string         `json:"label" jsonschema:"What the button reads. Also how ExecuteAction finds this action within the view."`
	Command string         `json:"command" jsonschema:"Name of a command in the registry this action invokes. Must exist when the view is written."`
	Input   map[string]any `json:"input,omitempty" jsonschema:"Arguments to invoke the command with. Merged with, and overridden by, whatever the caller of ExecuteAction supplies."`
	Confirm bool           `json:"confirm,omitempty" jsonschema:"Whether the frontend must ask the user to confirm before invoking the command."`
}

// Node is one element of a view's tree: a component from the catalog, its
// props, bindings from Source data into those props, and its children.
type Node struct {
	Component string            `json:"component" jsonschema:"Name of a component in the catalog Components() serves. Refused if unknown."`
	Props     map[string]any    `json:"props,omitempty" jsonschema:"Literal values for the component's props, checked against its declared schema."`
	Bind      map[string]string `json:"bind,omitempty" jsonschema:"Prop name to source-collection field name, resolved to the record's value when the view renders."`
	Children  []Node            `json:"children,omitempty" jsonschema:"Nested nodes. Refused unless the component declares it accepts children."`
	Actions   []Action          `json:"actions,omitempty" jsonschema:"Buttons this node offers."`
}

// View is a screen an agent composed: a tree of catalog components, bound to
// a data source, stored as data so the frontend renders it without a build or
// a deploy.
type View struct {
	// ID and Skill come from the path: a view lives at
	// .aos/views/{skill}/{id}.view.json when scoped to a skill.
	ID   string `json:"id" collection:"path" jsonschema:"Identifier of this view. Also its file name, so lowercase, digits, hyphen and underscore only."`
	Name string `json:"name" jsonschema:"Human name of the view. Example: \"Deals by stage\"."`

	Title       string `json:"title" jsonschema:"Heading the frontend shows above the rendered tree."`
	Description string `json:"description,omitempty" jsonschema:"What this view is for."`

	// Scope says who a view is for: e.g. "user" or "skill". It is not a Kind —
	// Kind is how the view renders, Scope is who owns it.
	Scope string `json:"scope" jsonschema:"user or skill. A skill-scoped view is removed when the skill is uninstalled."`

	// Skill is set when a view belongs to a skill rather than to a user
	// directly, which is what lets a skill ship its own screens.
	Skill string `json:"skill,omitempty" collection:"path=skill" jsonschema:"The skill this view ships with, when Scope is skill."`

	Source Source `json:"source" jsonschema:"Where this view's data comes from."`
	Tree   Node   `json:"tree" jsonschema:"The composed tree of catalog components this view renders."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When the view was created."`
	UpdatedAt time.Time `json:"updatedAt" jsonschema:"When the view was last changed."`
}
