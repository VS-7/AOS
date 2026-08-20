package toolset

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "toolsets",
	Tool:    "Toolsets",
	Summary: "External connections — MCP servers, REST APIs, CLIs — reached through one call.",
	Doc: `A toolset is configured once and called by id thereafter. Its own tools are
never registered into the agent's tool list — they are reached only through
toolsets_call, which is the one place this system executes something outside
its own process.

## Commands
- **list** — every configured toolset
- **get** — one toolset's full configuration
- **get-config** — the same read, for a caller only editing its configuration
- **call** — run one of its tools
- **update-config** — reconfigure it
- **delete** — remove it

## When to use
- **Before calling a tool you have not called yet:** get it first — the
  configuration and status tell you whether it is even reachable
- **A capability outside what this system's own domains cover:** call
  toolsets_call rather than assuming a native command exists

## When NOT to use
- Not to reach a native domain — toolsets are for integrations this system
  does not itself implement`,
	Hint: `A disabled toolset keeps its configuration and refuses call — disable rather
than delete a misbehaving one, so the work of getting it right is not lost.

call is opaque both ways: the input and the result are relayed to and from
whatever the connected target defines, not interpreted here.`,
}

// ListInput takes nothing — List has no filter — but every command input
// still carries a reason.
type ListInput struct {
	command.Reasoning
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	listHandler := func(ctx context.Context, _ ListInput) ([]Toolset, error) {
		return svc.List(ctx)
	}
	command.MustRegister(reg, command.Command[ListInput, []Toolset]{
		Group:   "toolsets",
		Name:    "list",
		Summary: "List every configured toolset.",
		Doc:     "Every toolset configured in this workspace, with its type and lifecycle status.",
		Examples: []command.Example{
			{Description: "everything configured", Input: ListInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List toolsets", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     listHandler,
	})

	command.MustRegister(reg, command.Command[GetInput, *Toolset]{
		Group:   "toolsets",
		Name:    "get",
		Summary: "Read one toolset's configuration.",
		Doc:     "Read a toolset in full: its connection settings and its lifecycle status.",
		Examples: []command.Example{
			{Description: "read a toolset before calling it", Input: GetInput{ID: "gh"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a toolset", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[GetInput, *Toolset]{
		Group:   "toolsets",
		Name:    "get-config",
		Summary: "Read one toolset's configuration, for editing.",
		Doc: `The same read as toolsets_get — a toolset's configuration is not split
into a separate document — named for a caller opening it specifically to
change its settings.`,
		Examples: []command.Example{
			{Description: "read before reconfiguring", Input: GetInput{ID: "gh"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a toolset's configuration", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CallInput, CallOutput]{
		Group:   "toolsets",
		Name:    "call",
		Summary: "Run one tool of a connected toolset.",
		Doc: `The one boundary where this system executes something outside its own
process: connects, calls one tool, closes, and audits the attempt regardless
of outcome.

Input and the result are opaque to this service — call toolsets_get first if
the tool's own argument shape is not already known.`,
		Examples: []command.Example{
			{Description: "call a tool with no arguments", Input: CallInput{ID: "gh", Tool: "whoami"}},
		},
		Registry: true,
		Annotations: command.Annotations{
			Title: "Call a tool", DestructiveHint: true, OpenWorldHint: true,
		},
		Handler: svc.Call,
	})

	command.MustRegister(reg, command.Command[UpdateConfigInput, *Toolset]{
		Group:   "toolsets",
		Name:    "update-config",
		Summary: "Reconfigure a toolset.",
		Doc: `Change the describable parts of a toolset's configuration. A field left
nil is unchanged; Args, Env and Headers, given at all, replace the field
wholesale — there is no per-key merge.`,
		Examples: []command.Example{
			{Description: "disable a misbehaving toolset", Input: UpdateConfigInput{ID: "gh", Status: statusPtr(StatusDisabled)}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Reconfigure a toolset"},
		Handler:     svc.UpdateConfig,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "toolsets",
		Name:    "delete",
		Summary: "Remove a toolset.",
		Doc:     "Remove a toolset's configuration. Idempotent: deleting what is already gone succeeds rather than erroring.",
		Examples: []command.Example{
			{Description: "remove a toolset", Input: DeleteInput{ID: "gh"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a toolset", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func statusPtr(s Status) *Status { return &s }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, GetInput) (*Toolset, error)               = (*Service)(nil).Get
	_ func(context.Context, CallInput) (CallOutput, error)            = (*Service)(nil).Call
	_ func(context.Context, UpdateConfigInput) (*Toolset, error)      = (*Service)(nil).UpdateConfig
	_ func(context.Context, DeleteInput) (DeleteOutput, error)        = (*Service)(nil).Delete
)
