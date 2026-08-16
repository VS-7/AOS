package job

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "jobs",
	Tool:    "Jobs",
	Summary: "The queue of deferred work, and whether it is healthy.",
	Doc: `What the daemon is running, has run, and could not run.

Turns, tasks and routines all become jobs. This group is how you find out
whether the work you expected to happen actually did, and why it did not.

## Commands
- **list** — the jobs, filtered by queue, status, workspace or kind
- **get** — one job with its payload, its result and its error
- **stats** — the shape of the queue: how many in each state, what is stuck
- **recover** — hand back jobs whose worker stopped reporting
- **purge** — remove finished jobs older than the window

## When to use
- **When something did not happen:** stats first, then list the dead ones
- **After a crash:** recover, to return the work its worker was holding

## When NOT to use
- Not to schedule work — work is enqueued by whatever owns it
- Not as a log of what an agent did; that is the activity log`,
	Hint: `There is no enqueue here, deliberately. A queue anything can write to is a queue
whose contents nobody can attribute.

A job marked claimed whose lease has lapsed is not a busy job: it is one whose
worker died. Stats separates the two, and recover is what fixes it.

A dead job is kept rather than deleted. "It ran and failed three times" and "it
never arrived" are different problems.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "jobs",
		Name:    "list",
		Summary: "List the queued work.",
		Doc:     "The jobs the daemon holds, newest first, filtered by queue, status, workspace or handler kind.",
		Examples: []command.Example{
			{Description: "what failed for good", Input: ListInput{Status: Dead}},
			{Description: "what is running now", Input: ListInput{Status: Claimed}},
			{Description: "one queue only", Input: ListInput{Queue: QueueRoutine}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List jobs", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Job]{
		Group:   "jobs",
		Name:    "get",
		Summary: "Read one job.",
		Doc:     "One job in full: what it was asked to do, how many times it has been tried, what it produced, and what went wrong.",
		Examples: []command.Example{
			{Description: "read a job a listing turned up", Input: GetInput{ID: "j-42"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one job", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[StatsInput, StatsOutput]{
		Group:   "jobs",
		Name:    "stats",
		Summary: "The shape of the queue right now.",
		Doc: `How many jobs in each state and on each queue, with the two lists that
matter: what died, and what is held by a worker that stopped reporting.

The second is the shape of a real incident. A claimed job whose lease has lapsed
is not busy — its worker is gone, and recover is what returns it.`,
		Examples: []command.Example{
			{Description: "is anything stuck", Input: StatsInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Queue statistics", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Stats,
	})

	command.MustRegister(reg, command.Command[RecoverInput, RecoverOutput]{
		Group:   "jobs",
		Name:    "recover",
		Summary: "Hand back work whose worker stopped reporting.",
		Doc: `Return every job whose lease has lapsed to the queue.

A worker renews its lease while it runs. One that is killed stops, the lease
expires, and this is what makes the work it was holding runnable again rather
than a permanent hole in the queue.`,
		Examples: []command.Example{
			{Description: "after a crash", Input: RecoverInput{}},
		},
		Annotations: command.Annotations{Title: "Recover stale jobs", IdempotentHint: true},
		Handler:     svc.Recover,
	})

	command.MustRegister(reg, command.Command[PurgeInput, PurgeOutput]{
		Group:   "jobs",
		Name:    "purge",
		Summary: "Remove finished jobs older than the window.",
		Doc: `Drop succeeded and dead jobs past the retention window, seven days by
default.

Only terminal jobs go. Anything pending, claimed or awaiting a retry stays
whichever window you ask for.`,
		Examples: []command.Example{
			{Description: "apply the default retention", Input: PurgeInput{}},
		},
		Annotations: command.Annotations{Title: "Purge finished jobs", DestructiveHint: true},
		Handler:     svc.Purge,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)   = (*Service)(nil).List
	_ func(context.Context, StatsInput) (StatsOutput, error) = (*Service)(nil).Stats
)
