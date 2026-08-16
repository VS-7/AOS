package todo

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
//
// The original nests these under tasks — "tasks todos list", "tasks_todos_list".
// The Command Layer here is two levels deep by construction, so they are their
// own group with the parent task as the first argument. The shape of the call
// is the same; only the name is flatter.
var GroupDoc = command.GroupDoc{
	Name:    "todos",
	Tool:    "Todos",
	Summary: "The execution plan inside a task.",
	Doc: `The steps of a task, in order.

A plan is not a nicety here: a task cannot move to in_review while any of its
steps is still open, so the plan is what the review guard counts. Write it
before deep execution, and keep it honest as you go.

## Commands
- **list** — read a task's plan with its progress
- **get** — read one step
- **create** — add a step
- **update** — change a step's title, order, evidence or notes
- **set-status** — move a step, with the evidence for it
- **delete** — remove a step

## When to use
- **Before executing:** if the plan is missing, write it first
- **As you work:** move each step and record what you verified

## When NOT to use
- Not for a one-line task with nothing to sequence
- Not as a progress report to a person — that is a task comment`,
	Hint: `Status is moved with set-status, never written with update. The move is
validated; a field write would route around every guard.

Record evidence when you finish a step that was verifiable. The system will let
you finish without it, and will tell you that nobody reading the plan later will
know what was checked.

A step that turned out to be unnecessary is skipped, not finished. Finishing it
claims work that did not happen.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "todos",
		Name:    "list",
		Summary: "Read a task's plan.",
		Doc: `The steps of one task, in plan order, with how much of it is done.

The progress count is what the task's review guard reads: completed counts steps
that are finished or deliberately skipped, and everything else is still open.`,
		Examples: []command.Example{
			{Description: "read the plan before starting", Input: ListInput{Task: "t-42"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a task plan", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Todo]{
		Group:   "todos",
		Name:    "get",
		Summary: "Read one step.",
		Doc:     "Read one step of a plan, including the evidence recorded for it.",
		Examples: []command.Example{
			{Description: "read a step by identifier", Input: GetInput{Task: "t-42", ID: "s-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one step", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Todo]{
		Group:   "todos",
		Name:    "create",
		Summary: "Add a step to the plan.",
		Doc: `Add one step.

Left without an order, the step goes at the end — so a plan written one call at
a time keeps the sequence it was written in.`,
		Examples: []command.Example{
			{Description: "the first step of a bug fix", Input: CreateInput{
				Task: "t-42", Title: "Reproduce the failure in a test",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Add a step"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Todo]{
		Group:   "todos",
		Name:    "update",
		Summary: "Change a step's description.",
		Doc: `Change what a step says.

Status is not writable here and the attempt is refused rather than ignored: a
field that silently did nothing would let you believe the step had moved.`,
		Examples: []command.Example{
			{Description: "record what was verified", Input: UpdateInput{
				Task: "t-42", ID: "s-1", Evidence: ptr("go test ./internal/domain/task passes, 24 cases"),
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Change a step"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[SetStatusInput, SetStatusOutput]{
		Group:   "todos",
		Name:    "set-status",
		Aliases: []string{"transition"},
		Summary: "Move a step through its lifecycle.",
		Doc: `Move one step, with the evidence for the move.

A step that turned out to be unnecessary is skipped, not finished — finishing it
claims work that did not happen. A step that failed after being finished is
reopened rather than deleted, so the record of the attempt survives.`,
		Examples: []command.Example{
			{Description: "finish a step with what proves it", Input: SetStatusInput{
				Task: "t-42", ID: "s-1", Status: Finished,
				Evidence: "the new test fails before the fix and passes after it",
			}},
			{Description: "a step that stopped applying", Input: SetStatusInput{
				Task: "t-42", ID: "s-3", Status: Skipped,
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Move a step"},
		Handler:     svc.SetStatus,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "todos",
		Name:    "delete",
		Summary: "Remove a step from the plan.",
		Doc: `Remove one step.

Prefer skipping. A deleted step leaves no record that it was ever planned, and
the plan is the audit trail of how the work was thought about.`,
		Examples: []command.Example{
			{Description: "remove a step added by mistake", Input: DeleteInput{Task: "t-42", ID: "s-4"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Remove a step", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func ptr[T any](v T) *T { return &v }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)           = (*Service)(nil).List
	_ func(context.Context, SetStatusInput) (SetStatusOutput, error) = (*Service)(nil).SetStatus
)
