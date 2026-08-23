# Goals

Strategic outcomes that daily work aligns to.

Before planning or executing significant work, check active goals to align
your efforts and avoid strategically useless results — technically correct
work that serves nothing.

## Commands
- **list** — goals matching a filter
- **get** — one goal in full
- **create** — a new goal
- **update** — change a goal's fields, including its status
- **delete** — remove a goal; every task that referenced it is disassociated,
  not removed

## When to use
- **Before starting significant work:** call goals_list with status active
  first, so the work chosen actually serves one of them

## When NOT to use
- Not for tracking a single unit of work — that is [[Task (Go)]]; a goal is
  the destination several tasks might serve

## Commands

### `goals_create`

Create a new goal.

Create a strategic outcome. Its id is derived from the title.

- a new goal

### `goals_delete`

Remove a goal.

Remove a goal. Every task that referenced it is disassociated, not deleted.

- remove an abandoned goal

### `goals_get`

Read one goal in full.

Read a goal: its status, its measure, and what it is for.

- read a goal before aligning work to it

### `goals_list`

List goals matching a filter.

Every goal matching the given status, project or text filter.

- everything active

### `goals_update`

Change a goal's fields.

Change the describable parts of a goal, including moving it to achieved or abandoned.

- mark a goal achieved

