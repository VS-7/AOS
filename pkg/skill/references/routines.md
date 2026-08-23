# Routines

Durable entry points for autonomous, scheduled or reactive work.

Work that starts without anybody asking.

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
- Not for a one-off at a specific time — that is a task with a due date

## Commands

### `routines_create`

Declare a routine.

Create a durable entry point for autonomous work.

The body is the prompt: what the agent is to do when it fires. Write it as an
instruction to somebody who will read it with nobody else present — a routine
cannot ask a question and wait for an answer.

A webhook trigger mints a token, returned once. Store it now.

- a weekday morning triage
- reacting to a bug entering review

### `routines_delete`

Remove a routine and its runs.

Delete the routine directory, with its whole run history.

Disabling is usually what you want: it stops firing and keeps the record of what
it did while it ran.

- remove a routine that is no longer wanted

### `routines_fire`

Run a routine now.

Fire a routine by hand, recording a run as any trigger would.

A disabled routine is not fired: the attempt records a skipped run and reports
why. Pass force to run it once anyway, without enabling it.

- test a routine you just wrote

### `routines_get`

Read one routine.

One routine: its triggers, its scope, when it fires next, and the
warnings about it.

The effective interval is reported next to the cron you declared. A schedule
finer than the tick fires once per tick, and this is where you find that out
rather than by watching it not happen.

- read a routine before changing it

### `routines_list`

List the routines.

The routines of an agent, with the scheduler's tick so the real resolution of each schedule is visible.

- your routines
- what is switched off

### `routines_rotate`

Mint a new webhook token.

Replace this routine's webhook token.

The previous one stops working immediately. The new one is shown once, here, and
only its hash is stored.

- replace a token that leaked

### `routines_runs`

Read a routine's audit history.

Every firing of this routine, newest first: what triggered it, how it
ended, what it cost, and the conversation it ran in.

Failures are here too. A routine that fails silently is indistinguishable from
one that never ran, which is why every firing writes a record.

- check whether it has been running

### `routines_update`

Change a routine.

Change a routine's name, status, triggers, scope or prompt.

Triggers are replaced whole rather than merged, because a partial update of a
discriminated union is how you end up with a scheduled trigger holding a stale
webhook hash. A webhook among the new triggers mints a new token.

- switch one off without deleting it

