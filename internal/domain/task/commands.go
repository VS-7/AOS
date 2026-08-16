package task

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "tasks",
	Tool:    "Tasks",
	Summary: "Execution contracts with a lifecycle, an owner and a review.",
	Doc: `Work that somebody is accountable for finishing.

A task is not a reminder. It has eight lifecycle states, an owner, a plan of
steps, a discussion, and a review it has to pass. Only an agent assignee is
dispatched autonomously; a task owned by a person is tracked, not executed.

## Commands
- **list** — the tasks, filtered by status, type, owner, project or goal
- **get** — one task with its resolved owner, plan progress and blockers
- **create** — record work in one of the four entry states
- **update** — change what the task says
- **set-status** — move it, with the guards that go with the move
- **branch** — cut the isolated Git checkout it executes in
- **delete** — remove it with its plan, discussion and runs

## Lifecycle
suggestion → backlog → planning → todo → in_progress → in_review → finished,
with stopped reachable from the middle three and returning to them.

## When to use
- **Work that outlives a conversation:** anything you would otherwise re-explain
- **Work you will hand to an agent:** assign it and it gets dispatched

## When NOT to use
- Not for a step inside work that already exists — that is a todo
- Not for something you will do in this reply`,
	Hint: `Status moves with set-status, never with update. The move is validated and the
guards run; a field write would route around both.

A task cannot reach in_review while any step of its plan is open. That is
enforced here, not merely advised: finish each step with the evidence for it, or
skip the ones that stopped applying.

While executing a task, report progress in its comments, not in chat. Nobody is
watching the chat when you run autonomously.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "tasks",
		Name:    "list",
		Summary: "List the tasks.",
		Doc: `The tasks of this workspace, newest first.

Each carries its resolved owner — which decides whether it can be dispatched —
its plan progress, and the dependencies still standing in its way.`,
		Examples: []command.Example{
			{Description: "what is being worked on now", Input: ListInput{Status: InProgress}},
			{Description: "what is waiting for review", Input: ListInput{Status: InReview}},
			{Description: "one agent's queue", Input: ListInput{Assigned: "atlas", Status: Todo}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List tasks", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *View]{
		Group:   "tasks",
		Name:    "get",
		Summary: "Read one task.",
		Doc: `One task in full: its description, its owner resolved, how much of its
plan is done, and what it is blocked by.

Read this before starting work. The blockers are the part people skip and then
discover halfway through.`,
		Examples: []command.Example{
			{Description: "read a task before starting it", Input: GetInput{ID: "t-42"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one task", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *View]{
		Group:   "tasks",
		Name:    "create",
		Summary: "Record a unit of work.",
		Doc: `Create a task in one of the four entry states: suggestion, backlog,
planning or todo.

It cannot be created directly in in_progress or in_review — a task that starts
there has skipped every guard on the way, including the one that says work in
review has a finished plan behind it.

Assign it to an agent and it becomes eligible for autonomous dispatch. Assign it
to a person and the system tracks it without executing it.`,
		Examples: []command.Example{
			{Description: "a bug to fix later", Input: CreateInput{
				Name: "Denial patterns never match a command line",
				Type: "bug", Priority: High,
				Summary: "The sandbox matches denial patterns with a path glob, so `*` stops at a separator and a command with a path in it slips through.",
			}},
			{Description: "work handed to an agent, in isolation", Input: CreateInput{
				Name: "Port the routine scheduler", Type: "feature",
				Assigned: "atlas", Status: Todo, Worktree: true,
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create a task"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *View]{
		Group:   "tasks",
		Name:    "update",
		Summary: "Change what a task says.",
		Doc: `Change a task's name, type, owner, priority, dependencies or body.

Status is not writable here and the attempt is refused. Dependencies are checked
for cycles: a pair of tasks that wait on each other could never start, and the
system would have no way to say so later.`,
		Examples: []command.Example{
			{Description: "hand the work to an agent", Input: UpdateInput{
				ID: "t-42", Assigned: ptr("atlas"),
			}},
			{Description: "record that it waits on another task", Input: UpdateInput{
				ID: "t-42", DependsOn: ptr([]string{"t-17"}),
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Change a task"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[SetStatusInput, SetStatusOutput]{
		Group:   "tasks",
		Name:    "set-status",
		Aliases: []string{"transition"},
		Summary: "Move a task through its lifecycle.",
		Doc: `Move a task, with the guards that belong to the move.

Moving to in_review requires every step of the plan to be finished or skipped.
Moving to in_progress requires the dependencies to be finished. Moving to
stopped writes a checkpoint — the conversation, the job, the open steps and the
progress — so a resumed run starts where this one ended rather than at the
beginning.`,
		Examples: []command.Example{
			{Description: "start work", Input: SetStatusInput{ID: "t-42", Status: InProgress}},
			{Description: "hand it to review", Input: SetStatusInput{ID: "t-42", Status: InReview}},
			{Description: "stop, recording why", Input: SetStatusInput{
				ID: "t-42", Status: Stopped, Reason: "waiting on the API key for the provider contract test",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Move a task"},
		Handler:     svc.SetStatus,
	})

	command.MustRegister(reg, command.Command[BranchInput, *Worktree]{
		Group:   "tasks",
		Name:    "branch",
		Summary: "Cut the isolated checkout a task executes in.",
		Doc: `Create a Git worktree on the task's own branch.

The sandbox root becomes that checkout, so an agent working on the task cannot
touch the main working tree. The workspace's onCreateScript runs inside it
afterwards, under the assigned agent's sandbox policy rather than with free
rein — a setup script is third-party code in most workspaces.

Old checkouts are pruned to the workspace limit, oldest finished task first. One
belonging to work still in progress is never taken.`,
		Examples: []command.Example{
			{Description: "isolate a task before starting it", Input: BranchInput{ID: "t-42"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Cut a task worktree"},
		Handler:     svc.Branch,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "tasks",
		Name:    "delete",
		Summary: "Remove a task and everything under it.",
		Doc: `Delete the task directory: the task, its plan, its discussion and its
runs.

This is not reversible and it removes the record of how the work was thought
about. A task that turned out to be unnecessary is usually better moved to
finished with a comment saying why.`,
		Examples: []command.Example{
			{Description: "remove a task created by mistake", Input: DeleteInput{ID: "t-99"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a task", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func ptr[T any](v T) *T { return &v }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)           = (*Service)(nil).List
	_ func(context.Context, SetStatusInput) (SetStatusOutput, error) = (*Service)(nil).SetStatus
	_ func(context.Context, BranchInput) (*Worktree, error)          = (*Service)(nil).Branch
)
