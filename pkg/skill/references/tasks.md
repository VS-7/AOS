# Tasks

Execution contracts with a lifecycle, an owner and a review.

Work that somebody is accountable for finishing.

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
- Not for something you will do in this reply

## Commands

### `tasks_branch`

Cut the isolated checkout a task executes in.

Create a Git worktree on the task's own branch.

The sandbox root becomes that checkout, so an agent working on the task cannot
touch the main working tree. The workspace's onCreateScript runs inside it
afterwards, under the assigned agent's sandbox policy rather than with free
rein — a setup script is third-party code in most workspaces.

Old checkouts are pruned to the workspace limit, oldest finished task first. One
belonging to work still in progress is never taken.

- isolate a task before starting it

### `tasks_create`

Record a unit of work.

Create a task in one of the four entry states: suggestion, backlog,
planning or todo.

It cannot be created directly in in_progress or in_review — a task that starts
there has skipped every guard on the way, including the one that says work in
review has a finished plan behind it.

Assign it to an agent and it becomes eligible for autonomous dispatch. Assign it
to a person and the system tracks it without executing it.

- a bug to fix later
- work handed to an agent, in isolation

### `tasks_delete`

Remove a task and everything under it.

Delete the task directory: the task, its plan, its discussion and its
runs.

This is not reversible and it removes the record of how the work was thought
about. A task that turned out to be unnecessary is usually better moved to
finished with a comment saying why.

- remove a task created by mistake

### `tasks_get`

Read one task.

One task in full: its description, its owner resolved, how much of its
plan is done, and what it is blocked by.

Read this before starting work. The blockers are the part people skip and then
discover halfway through.

- read a task before starting it

### `tasks_list`

List the tasks.

The tasks of this workspace, newest first.

Each carries its resolved owner — which decides whether it can be dispatched —
its plan progress, and the dependencies still standing in its way.

- what is being worked on now
- what is waiting for review
- one agent's queue

### `tasks_set-status`

Move a task through its lifecycle.

Move a task, with the guards that belong to the move.

Moving to in_review requires every step of the plan to be finished or skipped.
Moving to in_progress requires the dependencies to be finished. Moving to
stopped writes a checkpoint — the conversation, the job, the open steps and the
progress — so a resumed run starts where this one ended rather than at the
beginning.

- start work
- hand it to review
- stop, recording why

### `tasks_update`

Change what a task says.

Change a task's name, type, owner, priority, dependencies or body.

Status is not writable here and the attempt is refused. Dependencies are checked
for cycles: a pair of tasks that wait on each other could never start, and the
system would have no way to say so later.

- hand the work to an agent
- record that it waits on another task

