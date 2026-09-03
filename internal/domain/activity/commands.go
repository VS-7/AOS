package activity

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "activity",
	Tool:    "Activity",
	Summary: "The workspace's event log and notification inbox.",
	Doc: `What has happened in this workspace, in order.

Every meaningful change publishes here: a task moves, a memory is formed, a
routine runs. The log is the inbox a person reads and the bus a routine reacts
to — one record, so the two can never disagree about what happened.

## Commands
- **list** — read the inbox, newest first, filtered by namespace or event
- **get** — read one entry in full
- **read** — mark one entry read
- **read-all** — move your read watermark to now
- **purge** — drop what is past the retention window

## When to use
- **Reorienting after time away:** list with unread, to see what changed
- **Tracing a change:** filter by namespace and event to find when it happened

## When NOT to use
- Not as a place to write progress on a task — that is a task comment
- Not as your own memory — this log is shared and expires`,
	Hint: `Read state is per actor. Marking something read marks it read for you, not for
everyone, and your inbox is independent of the person's who is watching.

Entries expire after 90 days by default. Anything you want to survive that
belongs in a memory or a task comment, not here.`,
}

// Register declares the group on the registry.
//
// Publish is deliberately absent: an activity records something the system did,
// and a surface that could write one directly would let an agent fabricate the
// record of a change that never happened. Every entry in this log comes from the
// mutation that caused it.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "activity",
		Name:    "list",
		Summary: "Read the workspace inbox.",
		Doc: `What has happened, newest first.

Filter by namespace to see one kind of thing — tasks, memories, routines — and
by event to see one kind of change. The unread count is always for everything
that matched, not for the page you asked for, so it does not shrink as you page
through.`,
		Examples: []command.Example{
			{Description: "what you have not seen", Input: ListInput{Unread: true}},
			{Description: "every task that changed status", Input: ListInput{Namespace: "task", Event: "status_changed"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read the activity log", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Activity]{
		Group:   "activity",
		Name:    "get",
		Summary: "Read one entry in full.",
		Doc:     "Read one activity, including the structured payload a routine filter would match on.",
		Examples: []command.Example{
			{Description: "read an entry a listing turned up", Input: GetInput{ID: "550e8400-e29b-41d4-a716-446655440000"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one activity", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[MarkInput, MarkOutput]{
		Group:   "activity",
		Name:    "read",
		Summary: "Mark one entry as read.",
		Doc: `Mark one entry read for you.

Read state is per actor and never shared: this changes your inbox and nobody
else's.`,
		Examples: []command.Example{
			{Description: "dismiss one notification", Input: MarkInput{ID: "550e8400-e29b-41d4-a716-446655440000"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Mark an activity read", IdempotentHint: true},
		Handler:     svc.MarkAsRead,
	})

	command.MustRegister(reg, command.Command[MarkAllInput, MarkOutput]{
		Group:   "activity",
		Name:    "read-all",
		Summary: "Mark the whole inbox as read.",
		Doc: `Move your read watermark to now.

Everything published up to this moment counts as read for you. Anything
published after it does not.`,
		Examples: []command.Example{
			{Description: "clear the inbox", Input: MarkAllInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Mark everything read", IdempotentHint: true},
		Handler:     svc.MarkAllAsRead,
	})

	command.MustRegister(reg, command.Command[PurgeInput, PurgeOutput]{
		Group:   "activity",
		Name:    "purge",
		Summary: "Drop entries past the retention window.",
		Doc: `Remove what is older than the retention window, 90 days by default.

Partitions entirely past the window are removed whole. A month only partly
expired is rewritten in place, and the output names which ones — rewriting an
audit log is worth knowing about even when it is correct.`,
		Examples: []command.Example{
			{Description: "apply the configured retention", Input: PurgeInput{}},
			{Description: "keep only the last week", Input: PurgeInput{OlderThanDays: 7}},
		},
		Annotations: command.Annotations{Title: "Purge the activity log", DestructiveHint: true},
		Handler:     svc.Purge,
	})

	command.MustRegister(reg, command.Command[EventsInput, EventsOutput]{
		Group:   "activity",
		Name:    "events",
		Summary: "What a routine can react to.",
		Doc: `The catalogue of event kinds this workspace publishes.

A routine's activity trigger names a namespace and an event and fires when one
arrives. This answers which pairs are real, and which payload keys each one
carries — the fields a trigger filter can match on.

The catalogue is a promise, not a history: it lists what *can* happen, on a
workspace where none of it has happened yet. Use **list** to read what actually
did.`,
		Examples: []command.Example{
			{Description: "everything a routine can watch for", Input: EventsInput{}},
			{Description: "just the task events", Input: EventsInput{Namespace: "task"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List the event kinds", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Events,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "activity",
		Name:    "delete",
		Summary: "Remove one entry from the log.",
		Doc: `Remove a single entry.

This rewrites the month it lived in, which is the one operation in this package
that edits history rather than appending to it. Prefer marking it read.`,
		Examples: []command.Example{
			{Description: "remove an entry published in error", Input: DeleteInput{ID: "550e8400-e29b-41d4-a716-446655440000"}},
		},
		Annotations: command.Annotations{Title: "Delete an activity", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)   = (*Service)(nil).List
	_ func(context.Context, PurgeInput) (PurgeOutput, error) = (*Service)(nil).Purge
)
