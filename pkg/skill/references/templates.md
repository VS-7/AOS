# Templates

Reusable Liquid generators for work with a repeatable shape — task briefs, plans, reports.

A template combines a stable Liquid body with a declared set of variables.
Inspect a template's variables before rendering it, the same inspect-before-
execute pattern every composite tool in this system follows.

## Commands
- **list** — every template, optionally filtered by skill or a text query
- **get** — one template's full Liquid body and declared variables
- **create** — author a new template
- **update** — change an existing template's body, variables or metadata
- **delete** — remove a template
- **render** — run a template's body against variables, bounded by a
  timeout and an output size cap; write:true also writes the result to the
  template's own (Liquid-rendered) Output path

## When to use
- **Work has a recognizable, repeatable shape:** a task brief, a plan, a
  report — list or get before writing one from scratch
- **Before render:** get the template first to learn what variables it
  expects; a missing required one refuses with that same instruction

## When NOT to use
- Not for one-off text with no reuse value — a template is worth the
  round-trip only when the shape repeats

## Commands

### `templates_create`

Author a new template.

Create a template. Content is validated as Liquid before anything is written; a broken template is refused here, not discovered at render.

- a minimal template

### `templates_delete`

Remove a template.

Remove a template. Idempotent: deleting what is already gone succeeds rather than erroring.

- remove a template

### `templates_get`

Read one template's Liquid body and declared variables.

Read a template in full — call this before render to learn what variables it expects.

- inspect before rendering

### `templates_list`

List templates, optionally filtered by skill or text.

Every template matching the filter, without their Liquid content — get one by id to read its body.

- everything
- a skill's own templates

### `templates_render`

Render a template's body against variables.

Runs a template's Liquid body against the variables given, bounded by a
timeout and an output size cap. A required variable with no default and no
value given refuses the render, naming which one and pointing at
templates_get.

With write:true, also renders the template's own Output path — itself
Liquid — and writes the result there, confined to the active workspace.
Refused if the template declares no Output.

- render with variables
- render and write to the template's Output path

### `templates_update`

Change an existing template.

Change the describable parts of a template. A field left nil is
unchanged; Variables, given at all, replaces the field wholesale — there is
no per-entry merge. New Content, if given, is re-validated as Liquid before
anything is written.

- change the body

