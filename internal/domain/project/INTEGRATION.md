# Integrating `project`

Not wired into `internal/app/wire.go` or `frontend/src/lib/command-map.ts` by
this branch, on purpose — see the parallel-work note in the task that
produced it. Everything below is what the integrator needs to do that by
hand, once, centrally.

## `wire.go`

Import:

```go
"github.com/OWNER/aos/internal/domain/project"
```

`repoSet` / `newRepoSet` — add a repo the same way `toolsets`/`views` are
added:

```go
// in repoSet
projects *fscollections.Repo[project.Project]

// in newRepoSet
projectModel, err := collections.ModelOf[project.Project]("projects")
if err != nil {
	return repoSet{}, err
}
...
projects: fscollections.New(root, projectModel,
	fscollections.WithLock[project.Project](lock),
	fscollections.WithIndex[project.Project](index),
	fscollections.WithPublisher[project.Project](pub),
),
```

Service construction, after `taskSvc` exists (needed for the `Unlinkers`
wiring below):

```go
projectSvc := project.NewService(project.Deps{
	Repo:      repos.projects,
	Unlinkers: []project.Unlinker{taskProjectUnlinker{tasks: taskSvc}},
	Clock:     clock,
})
```

Register, alongside the other domains:

```go
project.Register(reg, projectSvc)
```

`App` struct field:

```go
Projects *project.Service
```

...and set it in the `return &App{...}` block: `Projects: projectSvc,`.

## The `Unlinker` adapter for `task`

`project.Service.Delete` asks every `Unlinker` to drop its own references to
the deleted project id before removing the project's own record — see
`service.go`'s doc comment on `Delete`. `task.Task` already carries a
`Project string` field (`internal/domain/task/entity.go:91`) and
`task.Service` already has everything needed to clear it:
`List(ctx, ListInput{Project: id})` to find every task referencing the
project, then `Update(ctx, UpdateInput{ID: t.ID, Project: stringPtr("")})`
for each. No change to `internal/domain/task` itself. A small adapter, e.g. in
`internal/app/ecosystem.go` next to the other adapters already there:

```go
type taskProjectUnlinker struct{ tasks *task.Service }

func (u taskProjectUnlinker) UnlinkProject(ctx context.Context, projectID string) error {
	found, err := u.tasks.List(ctx, task.ListInput{Project: projectID})
	if err != nil {
		return err
	}
	empty := ""
	for _, t := range found.Tasks { // check the actual field name on ListOutput
		if _, err := u.tasks.Update(ctx, task.UpdateInput{ID: t.ID, Project: &empty}); err != nil {
			return err
		}
	}
	return nil
}
```

(Verify `task.ListOutput`'s exact field name — not read by this branch to
avoid touching `internal/domain/task` files unnecessarily.)

## The `Unlinker` adapter for `goal`

`goal` is being built in parallel by a sibling task this round and did not
exist yet when this branch started. Once it lands: if `goal.Goal` carries a
project reference (the design doc for `project.md` implies goals associate
with a project the same way tasks do), add a second adapter following the
identical shape —

```go
type goalProjectUnlinker struct{ goals *goal.Service }

func (u goalProjectUnlinker) UnlinkProject(ctx context.Context, projectID string) error { ... }
```

— and append it to the `Unlinkers` slice in `project.NewService`'s
construction above:

```go
Unlinkers: []project.Unlinker{
	taskProjectUnlinker{tasks: taskSvc},
	goalProjectUnlinker{goals: goalSvc},
},
```

Until that lands, deleting a project only unlinks tasks — an honest, smaller
gap rather than a silent one; the `Unlinker` seam exists precisely so this is
a one-line addition later, not a rewrite.

## `frontend/src/lib/command-map.ts`

Remove `"project"` from `DORMANT_DOMAINS`. Add entries mirroring the
`view.*`/`toolset.*` shapes already there:

```ts
"project.list": "projects_list",
"project.getById": { key: "projects_get", renameIn: { project: "id" } },
"project.create": "projects_create",
"project.update": { key: "projects_update", renameIn: { project: "id" } },
"project.delete": { key: "projects_delete", renameIn: { project: "id" } },
```

`frontend/src/features/project/` already has 14 files ported from the
original — this branch did not audit their exact call sites against the
shape above (out of scope, kept to backend + tests per the task briefing);
the integrator should diff the ported UI's expected command names against
this list before wiring, since `id` vs `project` as the path param name is
the one detail most likely to need adjustment.

## Field divergence from the design doc's sketch

`docs/04 - Domínio/Project (Go).md`'s own `entity.go` sketch has `Status`,
`Color`, `Paths` but omits `Icon` and `Source` — both of which are real,
well-specified fields in the original's schema (`icon`, `source`, with
`source` validated as an absolute, existing directory — see
`_extracted/v401/.../features/project/schemas/project.schema.ts` and
`errors/project.errors.ts`'s `FRACTAL_PROJECT_SOURCE_INVALID`). This branch
kept the doc's fields and added `Icon`/`Source` rather than dropping either
set, on the same reasoning `Artifact (Go)` used to add `PasswordHash`: the
design doc is the primary spec, but a real field the original schema
specifies and the doc's sketch happened to omit is a doc gap, not a feature
to drop.
