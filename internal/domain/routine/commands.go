package routine

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "routines",
	Tool:    "Routines",
	Summary: "Durable entry points for autonomous, scheduled or reactive work.",
	Doc: `Work that starts without anybody asking.

A routine belongs to an agent and fires from one of three triggers: a cron
schedule, an authenticated webhook, or an activity inside the workspace. The
third is how reactive automation is built — "when a bug enters in_review, run
this".

Every firing records a run, successful or not. A routine with no run history is
only a promise that something happened.

## Commands
- **list** — the routines, with the scheduler's real resolution
- **get** — one routine, with when it fires next and anything wrong with it
- **create** — declare one, minting a webhook token if it needs one
- **update** — change it
- **rotate** — mint a new webhook token, invalidating the old one
- **fire** — run it now
- **runs** — its audit history
- **delete** — remove it with its runs

## When to use
- **Recurring work:** a morning triage, a nightly sweep
- **Reactive work:** something that should happen whenever X happens
- **Work triggered from outside:** a deploy hook, a form submission

## When NOT to use
- Not for something you are doing now — just do it
- Not for a one-off at a specific time — that is a task with a due date`,
	Hint: `The scheduler evaluates cron triggers once per tick, 15 minutes by default. A
cron of "* * * * *" fires once per tick, not once a minute — get reports the
effective interval next to what you declared.

A webhook token is shown once, at creation. Only a hash is stored. Rotating is
the only way to get another.

Scope is what the routine may do while it runs. Without it a routine cannot
create tasks or reach outside the machine — the tool registry is filtered before
the agent sees it, so it does not choose a tool and then get refused.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "routines",
		Name:    "list",
		Summary: "List the routines.",
		Doc:     "The routines of an agent, with the scheduler's tick so the real resolution of each schedule is visible.",
		Examples: []command.Example{
			{Description: "your routines", Input: ListInput{}},
			{Description: "what is switched off", Input: ListInput{Status: Disabled}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List routines", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *View]{
		Group:   "routines",
		Name:    "get",
		Summary: "Read one routine.",
		Doc: `One routine: its triggers, its scope, when it fires next, and the
warnings about it.

The effective interval is reported next to the cron you declared. A schedule
finer than the tick fires once per tick, and this is where you find that out
rather than by watching it not happen.`,
		Examples: []command.Example{
			{Description: "read a routine before changing it", Input: GetInput{ID: "r-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one routine", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, CreateOutput]{
		Group:   "routines",
		Name:    "create",
		Summary: "Declare a routine.",
		Doc: `Create a durable entry point for autonomous work.

The body is the prompt: what the agent is to do when it fires. Write it as an
instruction to somebody who will read it with nobody else present — a routine
cannot ask a question and wait for an answer.

A webhook trigger mints a token, returned once. Store it now.`,
		Examples: []command.Example{
			{
				Description: "a weekday morning triage",
				Input: CreateInput{
					Name:     "Triage new bugs each morning",
					Triggers: []TriggerInput{{Type: Scheduled, Cron: "0 9 * * 1-5"}},
					Scope:    Scope{AllowCreateTasks: true},
					Content:  "List every task of type bug in the backlog, read each one, and set a priority with a comment saying why.",
				},
			},
			{
				Description: "reacting to a bug entering review",
				Input: CreateInput{
					Name: "Check the evidence on a reviewed bug",
					Triggers: []TriggerInput{{
						Type: Activity, Namespace: "task", Event: "status_changed",
						Filters: []Filter{
							{Field: "to", Operator: OpEq, Value: "in_review"},
							{Field: "type", Operator: OpEq, Value: "bug"},
						},
					}},
					Content: "Read the task's plan and confirm that every finished step recorded evidence. Comment on the ones that did not.",
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create a routine"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, CreateOutput]{
		Group:   "routines",
		Name:    "update",
		Summary: "Change a routine.",
		Doc: `Change a routine's name, status, triggers, scope or prompt.

Triggers are replaced whole rather than merged, because a partial update of a
discriminated union is how you end up with a scheduled trigger holding a stale
webhook hash. A webhook among the new triggers mints a new token.`,
		Examples: []command.Example{
			{Description: "switch one off without deleting it", Input: UpdateInput{
				ID: "r-1", Status: ptr(Disabled),
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Change a routine"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[RotateInput, RotateOutput]{
		Group:   "routines",
		Name:    "rotate",
		Summary: "Mint a new webhook token.",
		Doc: `Replace this routine's webhook token.

The previous one stops working immediately. The new one is shown once, here, and
only its hash is stored.`,
		Examples: []command.Example{
			{Description: "replace a token that leaked", Input: RotateInput{ID: "r-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Rotate a webhook token"},
		Handler:     svc.Rotate,
	})

	command.MustRegister(reg, command.Command[FireInput, *Run]{
		Group:   "routines",
		Name:    "fire",
		Summary: "Run a routine now.",
		Doc: `Fire a routine by hand, recording a run as any trigger would.

A disabled routine is not fired: the attempt records a skipped run and reports
why. Pass force to run it once anyway, without enabling it.`,
		Examples: []command.Example{
			{Description: "test a routine you just wrote", Input: FireInput{ID: "r-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Fire a routine"},
		Handler:     svc.Fire,
	})

	command.MustRegister(reg, command.Command[RunsInput, RunsOutput]{
		Group:   "routines",
		Name:    "runs",
		Summary: "Read a routine's audit history.",
		Doc: `Every firing of this routine, newest first: what triggered it, how it
ended, what it cost, and the conversation it ran in.

Failures are here too. A routine that fails silently is indistinguishable from
one that never ran, which is why every firing writes a record.`,
		Examples: []command.Example{
			{Description: "check whether it has been running", Input: RunsInput{ID: "r-1", Limit: 10}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read routine runs", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Runs,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "routines",
		Name:    "delete",
		Summary: "Remove a routine and its runs.",
		Doc: `Delete the routine directory, with its whole run history.

Disabling is usually what you want: it stops firing and keeps the record of what
it did while it ran.`,
		Examples: []command.Example{
			{Description: "remove a routine that is no longer wanted", Input: DeleteInput{ID: "r-9"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a routine", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func ptr[T any](v T) *T { return &v }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)     = (*Service)(nil).List
	_ func(context.Context, CreateInput) (CreateOutput, error) = (*Service)(nil).Create
	_ func(context.Context, FireInput) (*Run, error)           = (*Service)(nil).Fire
)
