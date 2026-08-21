# Integration

Not wired into `internal/app/wire.go` or `frontend/src/lib/command-map.ts` by
design — those are shared files another pass integrates centrally. This is
the exact glue needed.

## `internal/app/wire.go`

Import:

```go
"github.com/OWNER/aos/internal/domain/goal"
```

`repoSet` / `newRepoSet` (mirror the `toolset`/`view` entries):

```go
// in repoSet struct:
goals *fscollections.Repo[goal.Goal]

// in newRepoSet, alongside the other collections.ModelOf calls:
goalModel, err := collections.ModelOf[goal.Goal]("goals")
if err != nil {
	return repoSet{}, err
}
// ... in the returned repoSet literal:
goals: fscollections.New(root, goalModel,
	fscollections.WithLock[goal.Goal](lock),
	fscollections.WithIndex[goal.Goal](index),
	fscollections.WithPublisher[goal.Goal](pub),
),
```

Service construction, after `taskSvc` exists (needed for the `Tasks` port
below) and before `reg.Freeze()`:

```go
goalSvc := goal.NewService(goal.Deps{
	Repo:  repos.goals,
	Tasks: goalTasksAdapter{tasks: taskSvc},
	Clock: clock,
})
```

Register call, alongside the other `.Register(reg, ...)` lines:

```go
goal.Register(reg, goalSvc)
```

`App` struct field, grouped with `Views`/`Toolsets`/`Skills`:

```go
Goals *goal.Service
```

...and in the returned `&App{...}` literal:

```go
Goals: goalSvc,
```

### `goalTasksAdapter` — no change needed to `internal/domain/task`

`goal.Tasks` only needs `ClearGoal(ctx, goalID string) error`. `task.Service`
already has everything to build this without any change to that package:
`List` (which already filters by `Goal`, see `task/service.go`'s `in.Goal !=
"" && t.Goal != in.Goal`) and `Update` (which already takes a `*string` for
`Goal`, see `if in.Goal != nil { current.Goal = *in.Goal }`). The adapter
lives beside the other small adapters already in `internal/app` (e.g.
`workspaceRoot`, `planner`, `assignees` in wire.go/ecosystem.go):

```go
// in internal/app, e.g. ecosystem.go
type goalTasksAdapter struct{ tasks *task.Service }

func (a goalTasksAdapter) ClearGoal(ctx context.Context, goalID string) error {
	found, err := a.tasks.List(ctx, task.ListInput{Goal: goalID})
	if err != nil {
		return err
	}
	empty := ""
	for _, t := range found {
		if _, err := a.tasks.Update(ctx, task.UpdateInput{ID: t.ID, Goal: &empty}); err != nil {
			return err
		}
	}
	return nil
}
```

(Verify `task.ListInput` actually has a `Goal` field with that exact name —
`service.go`'s `in.Goal` reference strongly implies it does, but this was not
independently confirmed by reading `task/port.go`/`task/service.go`'s input
struct declarations, since this package does not import `task`.)

## `frontend/src/lib/command-map.ts`

Remove `"goal"` from `DORMANT_DOMAINS` (`frontend/src/lib/command-map.ts`,
the `Set([...])` literal).

Add entries mirroring `toolset.*`/`view.*`'s shape — the original TS
controller (`_extracted/v401/server/src/features/goal/controllers/goal.controller.ts`)
names its path param `goal`, so `getById`/`update`/`delete` all `renameIn`
`goal` → `id`:

```ts
"goal.list": "goals_list",
"goal.getById": { key: "goals_get", renameIn: { goal: "id" }, wrapOut: "goal" },
"goal.create": "goals_create",
"goal.update": { key: "goals_update", renameIn: { goal: "id" } },
"goal.delete": { key: "goals_delete", renameIn: { goal: "id" } },
```

`frontend/src/features/goal/` already has 12 ported files from the original
product — not inspected in this pass (time-boxed); worth a shape-match check
before considering the frontend wiring done.

## Scope notes / deviations from the design doc

- The design doc's `Query` type on `Service.List` wasn't spelled out beyond
  its name — I added `Status []Status`, `Project string`, `Text string`,
  matching the original TS `FractalGoalListInputSchema`'s `status`/`project`/
  `query` filters minus pagination (`limit`/`offset` — omitted, time-boxed;
  `List` returns everything matching, no paging yet).
- The Go design doc's struct omits the original's `slug` and `priority`
  fields (present in `schemas/goal.schema.ts` but not in `docs/04 -
  Domínio/Goal (Go).md`'s sketch) — followed the Go doc as the source of
  truth rather than reintroducing them from the TS schema, since the Go doc
  already represents a deliberate field-set decision (e.g. `DueAt` replacing
  `deadline`, `Measure` added). If priority/slug turn out to be wanted,
  that's a follow-up, not an oversight.
- `Active` command-registry doesn't have its own command — the doc's
  `Service.Active` is a plain Go method, wired into `internal/runtime/prompt`
  for the "check active goals" inventory line, not a `goals_active` MCP/CLI
  command. Not wired into `prompt` here (out of scope — this package only
  exposes the method); an integrator wiring `Prompt Assembly`'s inventory can
  call `Goals.Active(ctx)` the way `internal/app/wire.go`'s `reader{...}`
  adapter already reaches other services.
