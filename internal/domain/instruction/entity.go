// Package instruction is durable, workspace-scoped behavioral policy: rules
// that shape every agent in the workspace, not just the one that wrote them.
//
// The distinction from internal/domain/memory is the whole point of this
// package existing separately, and the original's own master prompt draws it
// for the model reading it: memory is personal — the agent's own accumulated
// behavior — where an instruction is everyone's, workspace-wide policy that
// no agent's personal memory is allowed to override. That is also why an
// instruction is injected into a prompt as "trusted" where a memory is only
// "observed" — see docs/03 - Peças Críticas/Prompt Assembly.md — and why
// creating or changing one, unlike writing a memory, is a consultive action
// under ADR-0007: it is a shared-state change, not a private one.
package instruction

import "time"

// Instruction is one durable, workspace-wide behavioral rule.
type Instruction struct {
	// ID identifies this instruction. It lives in the path, exactly like
	// every other native collection record: .aos/instructions/{id}.instruction.md.
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this instruction, derived from Name if not given explicitly."`

	Name string `yaml:"name" json:"name" jsonschema:"Human name of the instruction. Example: \"Feature Protocol\"."`

	// Type is free-form categorization, inherited from the original: standards,
	// patterns, workflows, or anything else a workspace finds useful. It is
	// not a closed union — nothing here enforces its membership — because
	// the original never enforced one either, and a rule already inherited
	// without ceremony (ADR-0016, Faixa 3) is not the place to add one.
	Type string `yaml:"type" json:"type" jsonschema:"Categorization for organizational purposes. Example: standards, patterns, workflows."`

	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this instruction is for."`

	// Skill names the skill this instruction shipped with, when it did.
	// A skill-installed instruction is removed with the skill, the same rule
	// skill-scoped collections and views already follow.
	Skill string `yaml:"skill,omitempty" json:"skill,omitempty" jsonschema:"The skill this instruction ships with, when it is skill-scoped."`

	// Paths scopes the rule to matching files, the same role Paths plays for
	// Memory's Scopes: entries are doublestar glob patterns tested against a
	// file the agent is about to touch. Empty means workspace-wide — the
	// default and the common case, per the design doc's own decision.
	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty" jsonschema:"Glob patterns matching files this instruction applies to. Empty applies to the whole workspace. Example: [\"internal/domain/**/*.go\"]."`

	// Active gates whether Applicable ever returns this instruction. An
	// inactive instruction is kept, not deleted — the workspace equivalent of
	// toolset.StatusDisabled: the work of getting the wording right survives
	// even while the rule itself is switched off.
	Active bool `yaml:"active" json:"active" jsonschema:"Whether this instruction is currently in force. An inactive instruction is kept but never applied."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When this instruction was created."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`

	// Content is the instruction's own body: the text injected into a
	// prompt's trusted instruction block when Applicable selects it.
	Content string `yaml:"-" json:"content" collection:"content" jsonschema:"Markdown body of the instruction — what actually reaches the prompt."`
}

// Clone returns an Instruction that shares no mutable state with i: Paths is
// copied rather than aliased, the same rule every native record in this
// codebase follows at its own package boundary (see e.g. toolset.Toolset.Clone).
func (i Instruction) Clone() Instruction {
	c := i
	if i.Paths != nil {
		c.Paths = make([]string, len(i.Paths))
		copy(c.Paths, i.Paths)
	}
	return c
}
