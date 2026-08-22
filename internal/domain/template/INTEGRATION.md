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

## 4. Closed gap: `render` now writes to disk, opt-in

This was previously a disclosed gap: `Render` only ever returned the rendered
string, with no filesystem port wired to it, unlike the original's `render`
("the primary scaffolding command"), which writes the result to disk at a
Liquid-interpolated `output` path.

It is now closed, resolved exactly the way this note's own three open
questions anticipated:

- `template.Deps` gained `Workspaces` (`Root(ctx) (string, error)`, the same
  shape `file.Service` already takes) and `Files` (`Resolve`/`WriteFile`/
  `MkdirAll`, narrowed to exactly `file.FS`'s own three methods) —
  `wire.go` wires both to the same `workspaceRoot{workspaceSvc}` and
  `osfile.New()` the file explorer already uses, not a second
  implementation.
- Writing is opt-in: `RenderInput.Write` (default `false`) — only the caller
  asking for it touches disk, given this is the one place in the system
  Liquid actually executes over caller data. The `render` command's
  `ReadOnlyHint` annotation was removed accordingly, since the approval
  channel derives its risk level from it (ADR-0007) and the command can now
  genuinely write.
- `Output` is itself rendered through the same bounded `render()` Content
  uses — a pathological Output path gets the same timeout and size cap as a
  pathological body — and the resolved path is confined to the workspace via
  `Files.Resolve`, which returns whatever the real adapter's containment
  check reports (an outside-the-workspace path surfaces as
  `TEMPLATE_OUTPUT_WRITE_FAILED`, not a silent escape).

See `internal/domain/template/service.go`'s `Render`/`writeOutput` and
`internal/domain/template/service_test.go`'s "render, write to disk" section.
