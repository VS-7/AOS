// Package agent is the identity of a worker in the workspace.
//
// An agent is a Markdown file: YAML front matter for the identity, the body for
// the system instructions. That is what makes an agent versionable in Git —
// `git log` on AGENT.md is the history of how the agent was shaped.
package agent

import "time"

// Channel binds an agent to a messaging provider.
type Channel struct {
	Provider string `yaml:"provider" json:"provider" jsonschema:"Channel provider slug registered for this agent. Example: \"telegram\"."`
	Data     any    `yaml:"data" json:"data" jsonschema:"Provider-specific configuration payload. Pass structured JSON. Example: { \"chatId\": \"123\" }."`
}

// Agent is the complete record, the master shape every input derives from.
//
// The field descriptions are ported from the original's schema, where every
// field carries a .describe(). Those texts are not decoration: they become the
// documentation of the MCP tool and they are what the model reads to fill the
// payload.
type Agent struct {
	// ID and Skill come from the path, not from the front matter: an agent
	// lives at .aos/agents/{id}/AGENT.md, or inside a skill that ships it.
	ID    string `yaml:"-" json:"id" collection:"path" jsonschema:"Unique agent identifier (slug) used for file naming and references. Example: \"atlas\"."`
	Skill string `yaml:"-" json:"skill,omitempty" collection:"path" jsonschema:"Optional skill scope limiting the agent to one installed skill. Example: \"memory\"."`

	Name         string    `yaml:"name" json:"name" jsonschema:"Human-readable display name for the agent. Example: \"Neo\"."`
	Image        string    `yaml:"image,omitempty" json:"image,omitempty" jsonschema:"Optional avatar URL or data URI shown wherever the agent is listed."`
	Description  string    `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"Orchestrator-facing summary of when to delegate to this agent. Example: \"Owns agent CRUD and runtime tooling.\"."`
	Role         string    `yaml:"role,omitempty" json:"role,omitempty" jsonschema:"Functional role label shown in delegation prompts. Example: \"Quality Assurance Specialist\"."`
	Leader       string    `yaml:"leader,omitempty" json:"leader,omitempty" jsonschema:"Slug of the leader agent in a hierarchical team. Example: \"atlas\"."`
	Provider     string    `yaml:"provider,omitempty" json:"provider,omitempty" jsonschema:"LLM provider slug for this agent. Example: \"openai\"."`
	Model        string    `yaml:"model,omitempty" json:"model,omitempty" jsonschema:"LLM model id for this agent. Example: \"gpt-4o\"."`
	Voice        string    `yaml:"voice,omitempty" json:"voice,omitempty" jsonschema:"Voice name for speech output. Example: \"Kore\"."`
	Channels     []Channel `yaml:"channels,omitempty" json:"channels,omitempty" jsonschema:"Communication channel bindings for this agent."`
	Orchestrator bool      `yaml:"orchestrator" json:"orchestrator" jsonschema:"Marks the workspace orchestrator fallback for non-direct chats."`

	CreatedAt time.Time `yaml:"createdAt,omitempty" json:"createdAt,omitzero" jsonschema:"When the agent was created."`
	UpdatedAt time.Time `yaml:"updatedAt,omitempty" json:"updatedAt,omitzero" jsonschema:"When the agent was last changed."`

	// Content is the Markdown body: the system instructions of the agent.
	Content string `yaml:"-" json:"content,omitempty" collection:"content" jsonschema:"Markdown system instructions for the agent runtime."`
}

// DisplayName falls back to the slug, as the original does on create.
func (a Agent) DisplayName() string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}
