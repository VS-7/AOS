# Integrating `project`

**Done.** `internal/app/wire.go` builds `projectSvc` with both `Unlinker`s —
`taskProjectUnlinker` and `goalProjectUnlinker` (`goal` landed after this
note was written; the second adapter this note asked for is wired in
alongside the first, `internal/app/wire.go:561-564`) — registers it, and
exposes it on `App.Projects`. `frontend/src/lib/command-map.ts` has
`project.list`/`.getById`/`.create`/`.update`/`.delete`, and `project` is not
in `DORMANT_DOMAINS`.

Everything below is the original integration note, kept for the adapter
shape and the field-divergence record — both still accurate.

## The `Unlinker` adapters

`project.Service.Delete` asks every `Unlinker` to drop its own references to
the deleted project id before removing the project's own record. Both live in
`internal/app/ecosystem.go`:

```go
type taskProjectUnlinker struct{ tasks *task.Service }

func (u taskProjectUnlinker) UnlinkProject(ctx context.Context, projectID string) error {
	found, err := u.tasks.List(ctx, task.ListInput{Project: projectID})
	if err != nil {
		return err
	}
	empty := ""
	for _, t := range found.Tasks {
		if _, err := u.tasks.Update(ctx, task.UpdateInput{ID: t.ID, Project: &empty}); err != nil {
			return err
		}
	}
	return nil
}

type goalProjectUnlinker struct{ goals *goal.Service }

func (u goalProjectUnlinker) UnlinkProject(ctx context.Context, projectID string) error { /* mirrors the above against goal.Service */ }
```

## Field divergence from the design doc's sketch (still true)

`docs/04 - Domínio/Project (Go).md`'s own `entity.go` sketch has `Status`,
`Color`, `Paths` but omits `Icon` and `Source` — both real fields in the
original's schema (`icon`, `source`, validated as an absolute, existing
directory). This branch kept the doc's fields and added `Icon`/`Source`
rather than dropping either set, the same reasoning `Artifact (Go)` used for
`PasswordHash`: the design doc is the primary spec, but a real field the
original schema specifies and the doc's sketch happened to omit is a doc
gap, not a feature to drop.
