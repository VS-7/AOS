# Views

Screens an agent composes over a collection, with no build and no deploy.

A view is a tree of catalog components, bound to a collection's data, stored
as data — the frontend renders it the moment it is written.

Every button a view offers names a real command in the registry, checked when
the view is written and again when the button is pressed: a view is a
description of a screen, not a second way to write data that skips the
validation and the authorisation every other caller gets.

## Commands
- **list** — every view declared in the workspace, native and dynamic
- **get** — one declaration, including a skill-scoped one (pass skill)
- **render** — a view with its source data attached, ready to paint
- **execute-action** — press a button: dispatch the command it names
- **delete** — remove a view

## When to use
- **After a collection exists and needs a screen:** scaffold one, or compose
  a tree by hand with create
- **A skill-scoped view:** list returns it with its Skill field; get, render
  and delete need that field back to resolve it

## When NOT to use
- Not to run logic — every action is a command call, never code

## Commands

### `views_components`

List the catalog of components a view can compose.

Every component the design system publishes: its name, its declared
props and whether it accepts children — read this before composing a tree by
hand, so create is not a guess.

- before composing a view

### `views_create`

Compose a new view.

Declare a screen: a tree of catalog components bound to a collection's
fields.

The whole tree is validated against the source collection and against the
command registry before anything is written — an unknown component, an unbound
field or an action naming a command that does not exist is refused here, not
discovered later as a blank screen.

- a scaffolded table, composed by hand

### `views_delete`

Remove a view.

Remove a view's declaration.

A skill-scoped view is normally removed by uninstalling the skill, not by
calling this directly while the skill is still installed.

- remove a view

### `views_execute-action`

Press a button in a view.

Dispatch the command one of a view's declared actions names, by its
Label.

This does not execute anything itself: it resolves the action the view
already declared and validated, merges the caller's input over the action's
own, and invokes the named command through the same registry a CLI or an MCP
call would go through — the same input validation, the same authorisation.
What this can do therefore depends entirely on what the button names: a
read, or a delete, or anything registered.

- click a row's delete button

### `views_get`

Read one view's declaration.

Read one view's tree, unresolved — no source data attached. Use render to
get the tree together with the rows it binds to.

A skill-scoped view needs its Skill back to resolve — views_list reports it
on every entry.

- a user's own view
- a view a skill brought

### `views_list`

List every declared view.

Every view declared in the workspace — native and dynamic, user-scoped and skill-scoped alike.

- everything declared

### `views_render`

Resolve a view against its source data.

The view's tree, with the rows its source names actually attached — what
the frontend paints. No build, no deploy: this is called fresh every time the
screen opens.

- render a table before showing it

### `views_scaffold`

Compose a view an agent did not have to design by hand.

Map a collection's declared fields to the component that shows each one,
producing a tree that already survives what create would validate — this
does not write anything, it only composes; call create with the result to
save it.

- a table over a collection
- a detail screen

