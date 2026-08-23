# Collections

Structured tables an agent declares at runtime.

Data an agent shapes without a programmer: a schema, and the rows that
follow it.

A collection is a declaration — its fields, their types, what a hook
normalises on write — and its records are the rows that declaration governs.
"build me a CRM" becomes a collections_create for "contacts" plus the records
an agent or a person files into it, nothing generated, nothing deployed.

## Commands
- **list** — every collection declared in the workspace, native and dynamic
- **get** — one declaration, including a skill-scoped one (pass skill)
- **create** — declare a new collection
- **delete** — remove a declaration and unregister it
- **records-list** — the rows of one collection
- **records-get** — one row
- **records-create** — file a new row
- **records-update** — revalidate and rewrite a row
- **records-delete** — remove a row

## When to use
- **Structured data with no fixed shape in this codebase:** declare a
  collection before inventing a file format for it
- **A skill's own collection:** list returns it with its Skill field; get and
  delete need that field back to resolve it

## When NOT to use
- Not for anything already native (tasks, agents, memories) — those have
  their own groups and their own validation
- Not for one-off, unstructured content — that is a file

## Commands

### `collections_create`

Declare a new collection.

Declare a table: its fields and, optionally, the hooks that normalise a
record before it is validated.

The schema is checked for internal coherence — every ref points at something
real, every enum has values, no field is named twice — before anything is
written, and the collection is registered only once the write to disk has
actually succeeded.

- a contacts table

### `collections_delete`

Remove a collection's declaration.

Remove a declaration and unregister it. Its records go with it — nothing
can resolve a write against a declaration that no longer exists.

A skill-scoped collection is normally removed by uninstalling the skill, not
by calling this directly while the skill is still installed.

- remove a workspace collection

### `collections_get`

Read one collection's declaration.

Read one collection: its fields, their types, and the hooks that
normalise a record on write.

A skill-scoped collection needs its Skill back to resolve — collections_list
reports it on every entry, so a caller that listed first already has it.

- a workspace collection
- a collection a skill brought

### `collections_list`

List every declared collection.

Every collection declared in the workspace — native and dynamic, workspace-scoped and skill-scoped alike.

- everything declared

### `collections_records-create`

File a new row.

Add a record to a collection.

The declared hooks normalise the data first, then it is validated against the
schema and, for a field declared unique, against what is already stored.

- a new contact
- a note with a Markdown body

### `collections_records-delete`

Remove a row.

Remove one record from a collection.

- remove a contact

### `collections_records-get`

Read one row.

Read one record of a collection in full.

- a contact by id

### `collections_records-list`

List a collection's rows.

Every row of one collection matching the given filters and order.

- every row
- filtered and ordered

### `collections_records-update`

Revalidate and rewrite a row.

Replace a record's fields and revalidate them. A refusal leaves the
stored record untouched — nothing between reading it and validating the new
data can have written anything.

- move a contact to a new stage

