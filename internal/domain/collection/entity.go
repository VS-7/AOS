// Package collection is structured domain data defined at runtime by the
// agent. It is what lets "build me a CRM" become real tables without a
// programmer.
//
// The schema is data and is never evaluated. That is not a performance
// decision: it is why loading a schema an agent wrote is safe.
package collection

import "time"

// Scope says whether a collection belongs to the workspace or came with a
// skill. A skill-scoped collection is removed when the skill is uninstalled.
type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeSkill     Scope = "skill"
)

// Format is how each record of this collection is laid out on disk.
type Format string

const (
	FormatMarkdown Format = "md"
	FormatJSON     Format = "json"
)

// FieldType is the constrained subset of JSON Schema this engine understands:
// enough to describe records, not enough to be a programming language.
type FieldType string

const (
	TypeString  FieldType = "string"
	TypeNumber  FieldType = "number"
	TypeBoolean FieldType = "boolean"
	TypeDate    FieldType = "date"
	TypeEnum    FieldType = "enum"
	TypeRef     FieldType = "ref"
	TypeList    FieldType = "list"
)

// Field is one declared column.
type Field struct {
	Name        string    `json:"name" jsonschema:"Name of the field. How a record refers to it."`
	Type        FieldType `json:"type" jsonschema:"One of: string, number, boolean, date, enum, ref, list."`
	Description string    `json:"description,omitempty" jsonschema:"What this field holds, for whoever fills it in."`
	Required    bool      `json:"required,omitempty" jsonschema:"Whether a record must give this field a value."`
	Enum        []string  `json:"enum,omitempty" jsonschema:"The allowed values, for a field of type enum."`
	Ref         string    `json:"ref,omitempty" jsonschema:"The id of another collection, for a field of type ref."` // another collection's id
	Default     any       `json:"default,omitempty" jsonschema:"Value used when a create hook of action defaultTo applies."`
	Unique      bool      `json:"unique,omitempty" jsonschema:"Whether two records of this collection may share this value."`
}

// Collection is the declaration. The records it describes live under it and
// are collections.Record values, not Go structs.
type Collection struct {
	ID string `json:"id" collection:"path" jsonschema:"Identifier of this collection. Also its directory name, so lowercase, digits, hyphen and underscore only."`

	Name        string `json:"name" jsonschema:"Human name of the collection. Example: \"Contacts\"."`
	Description string `json:"description,omitempty" jsonschema:"What this collection is for."`
	Scope       Scope  `json:"scope" jsonschema:"workspace or skill. A skill-scoped collection is removed when the skill is uninstalled."`
	Skill       string `json:"skill,omitempty" collection:"path=skill" jsonschema:"The skill this collection ships with, when Scope is skill."`
	Format      Format `json:"format" jsonschema:"md or json — how each record of this collection is laid out on disk."`

	Fields []Field `json:"fields" jsonschema:"The declared columns of a record."`
	Hooks  []Hook  `json:"hooks,omitempty" jsonschema:"Declarative normalisations applied to a record on create or update."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When the collection was declared."`
	UpdatedAt time.Time `json:"updatedAt" jsonschema:"When the declaration last changed."`
}
