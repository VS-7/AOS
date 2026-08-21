# Integration notes — template domain

Everything below is what the integrator applies centrally. This branch does
not touch `internal/app/wire.go` or `frontend/src/lib/command-map.ts` on
purpose — see the mission brief for why.

## 1. `internal/app/wire.go`

Import block (alongside the other `internal/domain/*` imports):

```go
"github.com/OWNER/aos/internal/adapters/liquidengine"
"github.com/OWNER/aos/internal/domain/template"
```

`repoSet` struct — add a field:

```go
templates *fscollections.Repo[template.Template]
```

`newRepoSet` — model resolution, alongside the other `collections.ModelOf` calls:

```go
templateModel, err := collections.ModelOf[template.Template]("templates")
if err != nil {
	return repoSet{}, err
}
```

and, in the returned `repoSet{...}` literal:

```go
templates: fscollections.New(root, templateModel,
	fscollections.WithLock[template.Template](lock),
	fscollections.WithIndex[template.Template](index),
	fscollections.WithPublisher[template.Template](pub),
),
```

(`WithPublisher` follows the same convention as `collections`/`views`/`toolsets`/`skills` — a
template write should publish `collection.changed`-shaped realtime events the
same way those four do, since it is registered after `events` exists.)

Service construction, alongside `toolsetSvc`/`skillInstaller`:

```go
templateSvc := template.NewService(template.Deps{
	Repo:   repos.templates,
	Engine: liquidengine.New(),
	Clock:  clock,
})
```

Registration, alongside the other `*.Register(reg, ...)` calls:

```go
template.Register(reg, templateSvc)
```

`App` struct — add a field near `Toolsets`/`Skills`:

```go
Templates *template.Service
```

and in the `return &App{...}` literal:

```go
Templates: templateSvc,
```

## 2. `frontend/src/lib/command-map.ts`

Remove `"template"` from `DORMANT_DOMAINS`.

Add `COMMAND_MAP` entries. The command group is `templates`, six actions —
`list`, `get`, `create`, `update`, `delete`, `render` — matching the
original's own group name and action set exactly (verified against
`_extracted/v401/server/src/features/template/commands/index.ts`). Following
the shape of the existing `view.*`/`toolset.*` entries:

```ts
"template.list": "templates_list",
"template.getById": { key: "templates_get", renameIn: { template: "id" } },
"template.create": "templates_create",
"template.update": "templates_update",
"template.delete": { key: "templates_delete", renameIn: { template: "id" } },
"template.render": { key: "templates_render", renameIn: { template: "id" } },
```

(The `renameIn` pattern mirrors `view.getById`/`view.delete`/`view.render` in
the existing map — the frontend's route param is named after the resource,
`template`, and the Go command's field is `id`.)

## 3. Frontend UI gap — honest note

`frontend/src/features/template/` has only 1 file today (a stub/barrel), unlike
`artifact` (5), `goal` (12), `project` (14) or `marketplace` (16), which
already carry a ported panel from the original. This branch adds no new
frontend UI: once wired via the two steps above, `templates_*` is reachable
over MCP, HTTP and CLI, but there is no rich settings panel for a person to
manage templates from the desktop app yet — only whatever a generic
collection/record view can compose over the `templates` native collection
(the same fallback `view.Scaffold` already gives every other native
collection).

## 4. Known, disclosed gap: `render` does not write to disk

The original's `render` command is described as "the primary scaffolding
command" — it renders **and writes the result to disk** at a (Liquid-
interpolated) `output` path. The Go design doc's own `Design em Go` sketch for
`Render` only returns a string (`RenderResult{Output: out}`), with no
filesystem port in the `Service` interface it sketches, and the `Template`
entity's `Output` field is documented there only as "declares where a render
should land" — descriptive, not a instruction to `Render` itself.

This build follows the design doc's explicit sketch: `Render` returns the
rendered string and touches no filesystem. If disk-writing is wanted to match
the original's actual behavior, it needs:

- A `Workspaces`-shaped port (the same one `file.Service` takes in wire.go,
  see `workspaceRoot` there) so `Render` knows what directory `Output` is
  relative to.
- A decision on **when** it writes — always, or only when the caller opts in
  (an explicit `Write bool` on `RenderInput`, the safer default given this is
  the one place in the system Liquid actually executes over caller data).
- `Output` itself is plausibly Liquid too (the original's own example is
  `.fractal/artifacts/plans/{{name}}.plan.md`) — it would need to run through
  the same bounded `render()` path before being used as a path, with the same
  path-traversal care `artifact`'s HTTP serving already applies elsewhere in
  this phase.

Flagged here rather than silently built, per this project's own convention of
disclosing a gap instead of padding scope beyond what was specified
(see the Fase 8 roadmap note's own "Pendente" sections).
