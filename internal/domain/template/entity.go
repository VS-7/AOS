// Package template holds reusable generators for work with a repeatable
// shape — task briefs, plans, reports, emails — rendered from Liquid over
// caller-supplied variables. See docs/04 - Domínio/Template (Go).md and
// ADR-0014 for the security boundary this package sits behind: it is the one
// place in the system where user-authored Liquid actually executes, and
// internal/runtime/prompt does not import it — persisted data is never
// rendered on the prompt path.
package template

import "time"

// Variable is one entry of a Template's declared contract: what it expects a
// caller to pass to Render. It is what templates_get returns, so an agent can
// inspect a template before rendering it rather than discovering a missing
// variable as a render failure.
type Variable struct {
	Name        string `yaml:"name"        json:"name"        jsonschema:"Name of the variable, as it appears in the template body: {{ name }}."`
	Type        string `yaml:"type"        json:"type"        jsonschema:"Descriptive type: string, number, boolean, list or object. Not enforced at render time — documentation for the caller, not validation."`
	Description string `yaml:"description" json:"description" jsonschema:"What this variable is for."`
	Required    bool   `yaml:"required"    json:"required"    jsonschema:"Whether Render refuses without this variable."`
	Default     any    `yaml:"default,omitempty" json:"default,omitempty" jsonschema:"Value substituted when the caller omits this variable and it is not Required."`
}

// Template is a reusable Liquid generator, collection-backed like every other
// native record: .aos/templates/{id}.template.md, or
// .aos/skills/{skill}/templates/{id}.template.md when Skill is set.
type Template struct {
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this template, used to address it in templates_get and templates_render."`

	Name        string `yaml:"name"                  json:"name"                  jsonschema:"Human name of the template."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this template produces and when to use it — read before render decides whether this is the right one."`
	Skill       string `yaml:"skill,omitempty"       json:"skill,omitempty"       jsonschema:"The skill this template ships with, when it is skill-scoped."`

	// Variables is the declared contract: what Render validates the caller's
	// variables against before the Liquid engine ever runs. See
	// validateVariables in render.go.
	Variables []Variable `yaml:"variables,omitempty" json:"variables,omitempty" jsonschema:"The variables this template's body expects. templates_render validates against this before rendering."`

	// Output names where a render should land, when this template generates a
	// file rather than a caller-consumed string. It is itself Liquid,
	// rendered against the same variables Content is — see
	// Service.writeOutput — and templates_render only writes there when the
	// caller opts in with Write; by default a render only returns text.
	Output string `yaml:"output,omitempty" json:"output,omitempty" jsonschema:"Relative output path for a render of this template, itself Liquid. Written to only when templates_render is called with write:true — see its own doc."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When this template was created."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`

	// Content is the Liquid template body — the one place in this system
	// where user-authored Liquid actually executes (ADR-0014).
	Content string `yaml:"-" json:"content" collection:"content" jsonschema:"The Liquid template body."`
}

// Clone returns a Template that shares no mutable state with t: Variables is
// copied rather than aliased, the same rule every native record in this
// codebase applies at its own package boundary (see toolset.Toolset.Clone).
func (t Template) Clone() Template {
	c := t
	if t.Variables != nil {
		c.Variables = append([]Variable(nil), t.Variables...)
	}
	return c
}
