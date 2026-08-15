// Package command is the single definition of every capability of the system.
//
// One declaration produces five surfaces: a CLI command, an MCP tool, a tool in
// the agent's internal registry, an HTTP endpoint and a page of documentation.
// It is the piece with the most leverage in the system: without it, ~140
// capabilities times five surfaces are 700 points of manual synchronisation,
// and they diverge in weeks.
package command

import "context"

// Example is one worked call, shown in --help, in the tool description and in
// the generated SKILL.md.
type Example struct {
	Description string `json:"description"`
	Input       any    `json:"input"`
}

// Command is the single definition of one capability. Everything the five
// surfaces need is derived from this struct plus reflection over In.
type Command[In any, Out any] struct {
	Group   string // "memories"
	Name    string // "store"
	Summary string // one line — cobra's Short
	Doc     string // full Markdown — becomes the MCP tool description

	// Aliases are alternative names on the CLI ("ls" for "list").
	Aliases  []string
	Examples []Example

	// Local marks a command that never talks to a remote API: no --base-url,
	// no --token. Today only the gateway group.
	Local bool

	// Registry keeps the command inside the agent's internal tool registry.
	// False is the privilege boundary: gateway, auth and tunnels stay out of
	// the agent's reach. The agent operates the domain, not the identity of the
	// installation nor its exposure to the network.
	Registry bool

	// Annotations map to MCP tool annotations and drive the approval risk
	// level of ADR-0007.
	Annotations Annotations

	Handler func(ctx context.Context, in In) (Out, error)
}

// Annotations describe what a tool does to the world. The MCP client shows
// them, and the approval channel derives its risk level from them.
type Annotations struct {
	Title           string
	ReadOnlyHint    bool
	DestructiveHint bool
	IdempotentHint  bool
	OpenWorldHint   bool
}

// Surface names who is calling. It exists for one reason: `_reasoning` is
// mandatory for a model and meaningless for a human typing a command, exactly
// as in the original, where the CLI schema is args+options and the tool schema
// is that plus `_reasoning`.
type Surface int

const (
	SurfaceCLI Surface = iota
	SurfaceHTTP
	SurfaceMCP
	SurfaceAgent
)

func (s Surface) String() string {
	switch s {
	case SurfaceCLI:
		return "cli"
	case SurfaceHTTP:
		return "http"
	case SurfaceMCP:
		return "mcp"
	default:
		return "agent"
	}
}

// RequiresReasoning reports whether a call from this surface must carry a
// non-empty `_reasoning`.
func (s Surface) RequiresReasoning() bool { return s == SurfaceMCP || s == SurfaceAgent }

// GroupDoc describes a group of commands: the text that becomes the composite
// tool's description and the group's page in the generated documentation.
type GroupDoc struct {
	Name    string
	Summary string
	Doc     string

	// Tool is the composite tool's name, in the original's casing: "Memory"
	// for the "memories" group. Empty means derive it from the group name.
	Tool string

	// Hint is an extra paragraph placed between the action list and the usage
	// block of the composite description, mirroring the original's groupHint.
	Hint string
}
