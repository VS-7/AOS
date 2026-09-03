package agent

import "github.com/OWNER/aos/internal/core/command"

// ListInput is the payload of `agents list`.
type ListInput struct {
	Skill string `json:"skill,omitempty" jsonschema:"Only agents shipped by this skill. Omit to list the workspace's own agents and every skill's."`
	Role  string `json:"role,omitempty" jsonschema:"Only agents with this functional role label."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of agents to return. 0 means no limit."`

	command.Reasoning
}

// ListOutput is what `agents list` returns.
type ListOutput struct {
	Agents []Agent `json:"agents" jsonschema:"The agents found, ordered by id."`
	Total  int     `json:"total" jsonschema:"How many agents matched."`
}

// MeInput is the payload of `agents me`: nothing.
//
// The identity is not a parameter. It comes from the execution context, which
// is the whole point — an agent that could name itself could name another.
type MeInput struct {
	command.Reasoning
}

// GetInput is the payload of `agents get`.
type GetInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Agent slug to retrieve. Example: \"neo\"." validate:"required"`

	command.Reasoning
}

// CreateInput is the payload of `agents create`, derived from the entity with
// the server-owned fields removed.
type CreateInput struct {
	// ID is optional because a caller who has a name has enough. It was
	// required, and the interface's own "New agent" form — ported from an
	// original whose server minted the id — never collected one, so creating
	// an agent from the application failed with "id is required", every time.
	// Every other domain that owns a slug already derives it (workspace,
	// task, goal, instruction, project); this one simply never did.
	ID           string    `json:"id,omitempty" cli:"arg" jsonschema:"Unique agent identifier (slug). Lowercase, no spaces. Defaults to a slug of name. Example: \"atlas\"." validate:"max=64"`
	Name         string    `json:"name,omitempty" jsonschema:"Human-readable display name. Defaults to the slug."`
	Description  string    `json:"description,omitempty" jsonschema:"Orchestrator-facing summary of when to delegate to this agent."`
	Role         string    `json:"role,omitempty" jsonschema:"Functional role label shown in delegation prompts."`
	Leader       string    `json:"leader,omitempty" jsonschema:"Slug of the leader agent in a hierarchical team."`
	Provider     string    `json:"provider,omitempty" jsonschema:"LLM provider slug for this agent. Example: \"openai\"."`
	Model        string    `json:"model,omitempty" jsonschema:"LLM model id for this agent. Example: \"gpt-4o\"."`
	Voice        string    `json:"voice,omitempty" jsonschema:"Voice name for speech output."`
	Channels     []Channel `json:"channels,omitempty" jsonschema:"Communication channel bindings for this agent."`
	Orchestrator bool      `json:"orchestrator,omitempty" jsonschema:"Marks the workspace orchestrator fallback for non-direct chats."`
	Reasoning_   string    `json:"reasoning,omitempty" jsonschema:"How hard this agent should think: none, low, medium or high. Defaults to the installation's setting."`
	Sandbox      *Sandbox  `json:"sandbox,omitempty" jsonschema:"Filesystem and execution policy. Omit for the working default: read, write, delete and execution by allowlist, without a shell."`
	Content      string    `json:"content,omitempty" jsonschema:"Markdown system instructions for the agent runtime."`

	command.Reasoning
}

// UpdateInput changes an existing agent. Every field is optional: an omitted
// field is left as it is, rather than blanked.
type UpdateInput struct {
	ID           string  `json:"id" cli:"arg" jsonschema:"Agent slug to update." validate:"required"`
	Name         *string `json:"name,omitempty" jsonschema:"New display name."`
	Description  *string `json:"description,omitempty" jsonschema:"New orchestrator-facing summary."`
	Role         *string `json:"role,omitempty" jsonschema:"New functional role label."`
	Leader       *string `json:"leader,omitempty" jsonschema:"New leader slug."`
	Provider     *string `json:"provider,omitempty" jsonschema:"New LLM provider slug."`
	Model        *string `json:"model,omitempty" jsonschema:"New LLM model id."`
	Voice        *string `json:"voice,omitempty" jsonschema:"New voice name."`
	Orchestrator *bool   `json:"orchestrator,omitempty" jsonschema:"Whether this agent is the workspace orchestrator."`
	Reasoning_   *string `json:"reasoning,omitempty" jsonschema:"New reasoning level: none, low, medium or high."`

	// Sandbox replaces the policy block whole rather than merging into it.
	// A permission somebody meant to remove has to actually be removed, and a
	// merge is how a removal silently does nothing.
	Sandbox *Sandbox `json:"sandbox,omitempty" jsonschema:"New filesystem and execution policy. Replaces the block entirely."`

	Content *string `json:"content,omitempty" jsonschema:"New Markdown system instructions. Replaces the body entirely."`

	command.Reasoning
}

// DeleteInput removes an agent and everything under its directory.
type DeleteInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Agent slug to delete. This removes the agent's memories and routines with it." validate:"required"`

	command.Reasoning
}

// DeleteOutput reports what was removed.
type DeleteOutput struct {
	ID      string `json:"id" jsonschema:"The agent that was removed."`
	Deleted bool   `json:"deleted" jsonschema:"True when the agent existed and was removed."`
}
