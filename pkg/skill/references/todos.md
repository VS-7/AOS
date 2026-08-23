# Todos

The execution plan inside a task.

The steps of a task, in order.

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
- Not as a progress report to a person — that is a task comment

## Commands

### `todos_create`

Add a step to the plan.

Add one step.

Left without an order, the step goes at the end — so a plan written one call at
a time keeps the sequence it was written in.

- the first step of a bug fix

### `todos_delete`

Remove a step from the plan.

Remove one step.

Prefer skipping. A deleted step leaves no record that it was ever planned, and
the plan is the audit trail of how the work was thought about.

- remove a step added by mistake

### `todos_get`

Read one step.

Read one step of a plan, including the evidence recorded for it.

- read a step by identifier

### `todos_list`

Read a task's plan.

The steps of one task, in plan order, with how much of it is done.

The progress count is what the task's review guard reads: completed counts steps
that are finished or deliberately skipped, and everything else is still open.

- read the plan before starting

### `todos_set-status`

Move a step through its lifecycle.

Move one step, with the evidence for the move.

A step that turned out to be unnecessary is skipped, not finished — finishing it
claims work that did not happen. A step that failed after being finished is
reopened rather than deleted, so the record of the attempt survives.

- finish a step with what proves it
- a step that stopped applying

### `todos_update`

Change a step's description.

Change what a step says.

Status is not writable here and the attempt is refused rather than ignored: a
field that silently did nothing would let you believe the step had moved.

- record what was verified

