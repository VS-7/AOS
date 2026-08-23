# Projects

Durable, top-level containers organizing related goals, tasks and files into coherent bodies of work.

A project structures a long-running effort that spans multiple tasks —
associate a task or goal with the right project to keep work traceable. The
hierarchy workspace → project → {goal, task} is not rigid: a task can exist
with no project and no goal.

## Commands
- **list** — every project, optionally filtered by status
- **get** — one project's full record
- **create** — a new project
- **update** — change a project's describable fields
- **delete** — remove a project's own record without touching the tasks or
  goals that were organized under it

## When to use
- **Structuring a long-running effort:** create a project before starting a
  batch of related tasks, so they stay traceable as a group
- **Filtering by relevance:** list with a status filter before assuming a
  project is still active

## When NOT to use
- Not for a single, standalone task — a project is for work that spans
  several

## Commands

### `projects_create`

Create a new project.

Start a new organizing boundary for related work. id defaults to a slug of name when omitted.

- a new project bound to a host directory

### `projects_delete`

Remove a project's own record.

Never cascades: tasks and goals under this project are unlinked, not removed.

- remove a finished project

### `projects_get`

Read one project's full record.

Read a project in full, including its paths and its bound host source, if any.

- read before updating

### `projects_list`

List projects, optionally filtered by status.

Every project in the workspace, or only those in one status.

- everything active

### `projects_update`

Change a project's describable fields.

A field left nil is unchanged; paths, given at all, replaces the field wholesale.

- pause a project

