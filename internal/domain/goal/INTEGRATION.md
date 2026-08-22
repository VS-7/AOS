# Integration

**Done.** `internal/app/wire.go` builds `goalSvc` (`Repo: repos.goals, Tasks:
goalTasksAdapter{tasks: taskSvc}, Clock: clock`), registers it
(`goal.Register(reg, goalSvc)`), and exposes it on `App.Goals`. The
`goalTasksAdapter` this note specified is in `internal/app/ecosystem.go`,
unchanged from the sketch below. `frontend/src/lib/command-map.ts` has
`goal.list`/`.getById`/`.create`/`.update`/`.delete`, and `goal` is not in
`DORMANT_DOMAINS`. `Goals.Active(ctx)` feeds the workspace inventory in
`internal/app/runtime.go`'s `reader.Inventory` alongside every other
resource category (`TestTheAssembledPromptCarriesEveryInventoryCategory`).

Everything below is the original integration note, kept for the adapter
shape and the scope decisions it recorded — both still accurate.

## `goalTasksAdapter` — no change needed to `internal/domain/task`

`goal.Tasks` only needs `ClearGoal(ctx, goalID string) error`. `task.Service`
already has everything to build this without any change to that package:
`List` (which already filters by `Goal`, see `task/service.go`'s `in.Goal !=
"" && t.Goal != in.Goal`) and `Update` (which already takes a `*string` for
`Goal`, see `if in.Goal != nil { current.Goal = *in.Goal }`). The adapter
lives beside the other small adapters already in `internal/app`:

```go
// internal/app/ecosystem.go
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

## Scope notes / deviations from the design doc (still true)

- `Service.List`'s `Query` has `Status []Status`, `Project string`, `Text
  string` — no pagination (`limit`/`offset`); `List` returns everything
  matching.
- The Go entity omits the original TS schema's `slug` and `priority` fields,
  matching `docs/04 - Domínio/Goal (Go).md`'s own sketch (`DueAt` replaces
  `deadline`, `Measure` is added). Not an oversight — a deliberate field-set
  decision the Go doc already made.
- `Active` is a plain `Service` method, not its own command — it is wired
  into the prompt inventory (see above), not exposed as `goals_active`.
