package agent

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group. It follows the
// original's shape — what it does, when to use it, when not to — because that
// structure is what makes a tool description usable rather than decorative.
var GroupDoc = command.GroupDoc{
	Name:    "agents",
	Tool:    "Agent",
	Summary: "Create, read and change the agents of the workspace.",
	Doc: `Manage the agents of this workspace.

An agent is a persistent identity: a slug, a role, a model, and Markdown
instructions that shape how it works. It is stored as ` + "`.aos/agents/{id}/AGENT.md`" + `,
which means an agent is versionable in Git and editable by hand.

## When to use
- To see who is available before delegating work
- To read an agent's instructions before changing them
- To create a specialist for a recurring kind of task

## When NOT to use
- Not to store knowledge — that is what memories are for
- Not to track work — that is what tasks are for`,
	Hint: "Deleting an agent removes its memories and routines with it. Read before you delete.",
}

// Register declares the group on the registry.
//
// The domain knows the Command Layer; the Command Layer does not know the
// domain. This function is the only place the two meet, and adding a capability
// never means touching internal/core/command.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "agents",
		Name:    "list",
		Aliases: []string{"ls"},
		Summary: "List the agents of the workspace.",
		Doc: `List every agent in the workspace, including the ones a skill shipped.

Returns the identity of each agent, not its instructions: use ` + "`agents get`" + ` for
the full record. Ordered by slug, so the list is stable between calls.`,
		Examples: []command.Example{
			{Description: "every agent", Input: ListInput{}},
			{Description: "only the reviewers", Input: ListInput{Role: "reviewer"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List agents", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Agent]{
		Group:   "agents",
		Name:    "get",
		Summary: "Read one agent, instructions included.",
		Doc: `Read one agent by slug.

Returns the whole record: identity, model binding, channels and the Markdown
body that is the agent's system instruction.`,
		Examples: []command.Example{
			{Description: "read the orchestrator", Input: GetInput{ID: "atlas"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read an agent", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[MeInput, *Agent]{
		Group:   "agents",
		Name:    "me",
		Summary: "Resolve your own identity in this workspace.",
		Doc: `Return the full record of whoever is calling.

Inside an agent execution that is the agent itself; from a human terminal it is
the workspace orchestrator. There is no argument: the identity comes from the
execution context, which is the point — an agent that could name itself could
name another.

Use it at the start of a session to confirm which instructions are actually
loaded. Most surprising behaviour turns out to be a different agent than the
one you thought was running.`,
		Examples: []command.Example{
			{Description: "who am I right now", Input: MeInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Resolve your own identity", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Me,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Agent]{
		Group:   "agents",
		Name:    "create",
		Summary: "Create an agent.",
		Doc: `Create an agent in this workspace.

The slug is the identity: it names the directory, and it is what every other
record refers to. It is lowercased on write, so "Atlas" and "atlas" are the same
agent rather than two.`,
		Examples: []command.Example{
			{
				Description: "a reviewer bound to a specific model",
				Input: CreateInput{
					ID: "reviewer", Name: "Reviewer", Role: "Quality Assurance Specialist",
					Provider: "openai", Model: "gpt-5.5",
					Content: "# Instructions\nReview diffs for correctness before style.",
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create an agent"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Agent]{
		Group:   "agents",
		Name:    "update",
		Summary: "Change an agent.",
		Doc: `Change one or more fields of an agent.

Only the fields you send are changed; the rest are left alone. Sending
` + "`content`" + ` replaces the instructions entirely — read them first.`,
		Registry:    true,
		Annotations: command.Annotations{Title: "Update an agent", IdempotentHint: true},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "agents",
		Name:    "delete",
		Summary: "Delete an agent and everything under it.",
		Doc: `Delete an agent.

This removes the agent's whole directory: its memories, its routines and its
event log go with it. There is no undo other than Git.`,
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete an agent", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error) = (*Service)(nil).List
	_ func(context.Context, GetInput) (*Agent, error)      = (*Service)(nil).Get
)
