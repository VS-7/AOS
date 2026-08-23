# Activity

The workspace's event log and notification inbox.

What has happened in this workspace, in order.

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
- Not as your own memory — this log is shared and expires

## Commands

### `activity_delete`

Remove one entry from the log.

Remove a single entry.

This rewrites the month it lived in, which is the one operation in this package
that edits history rather than appending to it. Prefer marking it read.

- remove an entry published in error

### `activity_get`

Read one entry in full.

Read one activity, including the structured payload a routine filter would match on.

- read an entry a listing turned up

### `activity_list`

Read the workspace inbox.

What has happened, newest first.

Filter by namespace to see one kind of thing — tasks, memories, routines — and
by event to see one kind of change. The unread count is always for everything
that matched, not for the page you asked for, so it does not shrink as you page
through.

- what you have not seen
- every task that changed status

### `activity_purge`

Drop entries past the retention window.

Remove what is older than the retention window, 90 days by default.

Partitions entirely past the window are removed whole. A month only partly
expired is rewritten in place, and the output names which ones — rewriting an
audit log is worth knowing about even when it is correct.

- apply the configured retention
- keep only the last week

### `activity_read`

Mark one entry as read.

Mark one entry read for you.

Read state is per actor and never shared: this changes your inbox and nobody
else's.

- dismiss one notification

### `activity_read-all`

Mark the whole inbox as read.

Move your read watermark to now.

Everything published up to this moment counts as read for you. Anything
published after it does not.

- clear the inbox

