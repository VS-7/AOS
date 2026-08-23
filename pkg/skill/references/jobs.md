# Jobs

The queue of deferred work, and whether it is healthy.

What the daemon is running, has run, and could not run.

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
- Not as a log of what an agent did; that is the activity log

## Commands

### `jobs_get`

Read one job.

One job in full: what it was asked to do, how many times it has been tried, what it produced, and what went wrong.

- read a job a listing turned up

### `jobs_list`

List the queued work.

The jobs the daemon holds, newest first, filtered by queue, status, workspace or handler kind.

- what failed for good
- what is running now
- one queue only

### `jobs_purge`

Remove finished jobs older than the window.

Drop succeeded and dead jobs past the retention window, seven days by
default.

Only terminal jobs go. Anything pending, claimed or awaiting a retry stays
whichever window you ask for.

- apply the default retention

### `jobs_recover`

Hand back work whose worker stopped reporting.

Return every job whose lease has lapsed to the queue.

A worker renews its lease while it runs. One that is killed stops, the lease
expires, and this is what makes the work it was holding runnable again rather
than a permanent hole in the queue.

- after a crash

### `jobs_stats`

The shape of the queue right now.

How many jobs in each state and on each queue, with the two lists that
matter: what died, and what is held by a worker that stopped reporting.

The second is the shape of a real incident. A claimed job whose lease has lapsed
is not busy — its worker is gone, and recover is what returns it.

- is anything stuck

