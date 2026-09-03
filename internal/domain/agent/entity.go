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

	// Reasoning is a divergence from the original, which reads the level only
	// from the installation's configuration. A reviewer and a triage agent
	// should not think equally hard, and the global value stays the default.
	Reasoning string `yaml:"reasoning,omitempty" json:"reasoning,omitempty" jsonschema:"How hard this agent should think: none, low, medium or high. Defaults to the installation's setting."`

	// Sandbox is what this agent may reach. Absent means read-only with no
	// execution at all, which is stricter than the original's default and is
	// the point of ADR-0006: the reach of an agent is a decision somebody
	// makes and writes down, in a file that shows up in a review.
	Sandbox *Sandbox `yaml:"sandbox,omitempty" json:"sandbox,omitempty" jsonschema:"Filesystem and execution policy for this agent."`

	CreatedAt time.Time `yaml:"createdAt,omitempty" json:"createdAt,omitzero" jsonschema:"When the agent was created."`
	UpdatedAt time.Time `yaml:"updatedAt,omitempty" json:"updatedAt,omitzero" jsonschema:"When the agent was last changed."`

	// Content is the Markdown body: the system instructions of the agent.
	Content string `yaml:"-" json:"content,omitempty" collection:"content" jsonschema:"Markdown system instructions for the agent runtime."`
}

// Sandbox is the policy block of the front matter.
type Sandbox struct {
	Permissions []string `yaml:"permissions,omitempty" json:"permissions,omitempty" jsonschema:"Any of: read, write, delete, execute. Defaults to read alone."`
	Exec        *Exec    `yaml:"exec,omitempty" json:"exec,omitempty" jsonschema:"Which programs this agent may run."`
}

// Exec is the execution policy: an allowlist, never a blocklist (ADR-0006).
type Exec struct {
	Policy     string   `yaml:"policy,omitempty" json:"policy,omitempty" jsonschema:"allowlist or deny-all. Defaults to deny-all."`
	Allow      []string `yaml:"allow,omitempty" json:"allow,omitempty" jsonschema:"Binary names or absolute paths this agent may run. Example: [\"git\", \"go\"]."`
	DenyArgs   []string `yaml:"denyArgs,omitempty" json:"denyArgs,omitempty" jsonschema:"Command lines to refuse even for an allowed binary. Example: [\"git push --force*\"]."`
	AllowShell bool     `yaml:"allowShell,omitempty" json:"allowShell,omitempty" jsonschema:"Whether this agent may reach a shell. A shell makes the allowlist a suggestion, so it takes its own opt-in."`
}

// DefaultSandbox is what an agent created through this domain may do.
//
// The zero value — read-only, no execution — is the right default for a
// *record*: an AGENT.md somebody hand-writes without a sandbox block has
// declared nothing, and nothing is what it should be able to do.
//
// It is the wrong default for a *creation*. The orchestrator's own
// instructions tell it to create focused specialists, and the settings screen
// offers a "New agent" form with no sandbox field at all; every agent either
// produced could read the workspace and nothing else, so its first Write or
// Bash came back AOS_SANDBOX_PERMISSION_DENIED and the specialist was useless
// from the moment it existed.
//
// This is not an escalation: whoever calls Create — the orchestrator, or a
// person at the settings screen — already holds these powers. Execution stays
// an allowlist (ADR-0006) and the shell stays off, because a shell makes an
// allowlist a suggestion. A caller that names a narrower sandbox gets exactly
// what it named, and agents_update replaces the block whole.
func DefaultSandbox() *Sandbox {
	return &Sandbox{
		Permissions: []string{"read", "write", "delete", "execute"},
		Exec: &Exec{
			Policy: "allowlist",
			Allow: []string{
				"git", "go", "node", "npm", "npx", "pnpm", "yarn", "bun",
				"python3", "pip3", "make", "task", "cargo", "rustc",
				"ls", "cat", "grep", "find", "rg", "sed", "awk", "head", "tail", "wc", "diff",
			},
			AllowShell: false,
		},
	}
}

// DisplayName falls back to the slug, as the original does on create.
func (a Agent) DisplayName() string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}
