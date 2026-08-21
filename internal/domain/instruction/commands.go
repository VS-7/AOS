package instruction

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "instructions",
	Tool:    "Instructions",
	Summary: "Durable, workspace-wide behavioral policy — rules every agent follows, not just the one that wrote them.",
	Doc: `An instruction is workspace-wide policy: it shapes every agent, and its own
memories and preferences never override it. Use instructions for a
correction the user wants to apply to the whole workspace; use memory
(memories_store) for a correction about your own personal behavior. See
memories_store's own doc for the other half of that split.

## Commands
- **list** — every instruction, optionally filtered by skill or a text query
- **get** — one instruction's full content
- **create** — declare a new instruction
- **update** — change an existing instruction's fields
- **delete** — remove an instruction

## When to use
- **The user establishes a policy that should apply to every agent:** create
  an instruction, not a memory
- **Before assuming a behavior is already covered:** list first — an
  instruction with overlapping Paths may already say what you were about to
  create again

## When NOT to use
- Not for something true only of your own behavior — that is memories_store
- Not to bypass a workspace-wide rule you disagree with: an instruction is
  policy, and changing it is a shared-state action other agents will also be
  bound by`,
	Hint: `create and update are shared-state changes — they route through the same
approval a destructive tool call would, when the caller is an agent rather
than a person, per ADR-0007. An inactive instruction (active: false) is kept,
not deleted, so disabling a rule that turned out wrong does not lose the work
of writing it.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "instructions",
		Name:    "list",
		Summary: "List instructions, optionally filtered.",
		Doc:     "Every instruction in the workspace, or the subset matching a skill or a text query.",
		Examples: []command.Example{
			{Description: "everything active in the workspace", Input: ListInput{}},
			{Description: "one skill's own instructions", Input: ListInput{Skill: "browser"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List instructions", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Instruction]{
		Group:   "instructions",
		Name:    "get",
		Summary: "Read one instruction in full.",
		Doc:     "Read an instruction's complete content, the part instructions_list omits by name only.",
		Examples: []command.Example{
			{Description: "read before applying it", Input: GetInput{ID: "feature-protocol"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read an instruction", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Instruction]{
		Group:   "instructions",
		Name:    "create",
		Summary: "Declare a new workspace-wide instruction.",
		Doc: `A shared-state change: the instruction created here shapes every agent in
the workspace, not just the one that called this. id is derived from name
when not given explicitly.`,
		Examples: []command.Example{
			{Description: "a workspace-wide standard", Input: CreateInput{
				Name: "Feature Protocol", Type: "standards",
				Content: "# Usage\n\nEvery new feature ships with a test.",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create an instruction"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Instruction]{
		Group:   "instructions",
		Name:    "update",
		Summary: "Change an existing instruction.",
		Doc:     "A field left nil is unchanged; Paths, given at all, replaces the field wholesale.",
		Examples: []command.Example{
			{Description: "narrow an instruction to one directory", Input: UpdateInput{
				ID: "feature-protocol", Paths: []string{"internal/domain/**/*.go"},
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update an instruction"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "instructions",
		Name:    "delete",
		Summary: "Remove an instruction.",
		Doc:     "Remove an instruction's declaration. Idempotent: deleting what is already gone succeeds rather than erroring.",
		Examples: []command.Example{
			{Description: "retire a rule that no longer applies", Input: DeleteInput{ID: "old-standards"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete an instruction", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)     = (*Service)(nil).List
	_ func(context.Context, GetInput) (*Instruction, error)    = (*Service)(nil).Get
	_ func(context.Context, CreateInput) (*Instruction, error) = (*Service)(nil).Create
	_ func(context.Context, UpdateInput) (*Instruction, error) = (*Service)(nil).Update
	_ func(context.Context, DeleteInput) (DeleteOutput, error) = (*Service)(nil).Delete
)
