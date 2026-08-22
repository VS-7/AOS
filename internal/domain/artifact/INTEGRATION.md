# Integrating the artifact domain

**`wire.go` and `command-map.ts` are done** — `artifactSvc` is built,
registered, and on `App.Artifacts`; `frontend/src/lib/command-map.ts` has
`artifact.list`/`.getById`/`.create`/`.update`/`.setPassword`/`.delete`
(`artifact` is not in `DORMANT_DOMAINS`). The sections below documenting
that wiring are kept for reference but are no longer the open item.

**HTTP serving (`/v/*`) is the remaining gap** — see "What's missing" below,
still accurate as of this note.

## `internal/app/wire.go`

Import:

```go
"github.com/OWNER/aos/internal/adapters/artifactfiles"
"github.com/OWNER/aos/internal/domain/artifact"
```

In `repoSet` / `newRepoSet`, alongside `toolsets`/`skills`:

```go
type repoSet struct {
	// ...
	artifacts *fscollections.Repo[artifact.Artifact]
}

func newRepoSet(...) (repoSet, error) {
	// ...
	artifactModel, err := collections.ModelOf[artifact.Artifact]("artifacts")
	if err != nil {
		return repoSet{}, err
	}
	// ...
	return repoSet{
		// ...
		artifacts: fscollections.New(root, artifactModel,
			fscollections.WithLock[artifact.Artifact](lock),
			fscollections.WithIndex[artifact.Artifact](index),
			fscollections.WithPublisher[artifact.Artifact](pub),
		),
	}, nil
}
```

Service construction, near where `toolsetSvc`/`skillInstaller` are built:

```go
artifactSvc := artifact.NewService(artifact.Deps{
	Repo:   repos.artifacts,
	Files:  artifactfiles.New(root),
	Hasher: artifact.Argon2Hasher{},
	Clock:  clock,
	IDs:    idgen,
	Log:    logger,
})
```

Registration, alongside the other `.Register(reg, ...)` calls:

```go
artifact.Register(reg, artifactSvc)
```

`App` struct field, alongside `Toolsets`/`Skills`:

```go
Artifacts *artifact.Service
```

...and in the `return &App{...}` literal:

```go
Artifacts: artifactSvc,
```

## `frontend/src/lib/command-map.ts`

Remove `"artifact"` from `DORMANT_DOMAINS`.

Add `COMMAND_MAP` entries mirroring the `view.*`/`toolset.*` shape already
there:

```ts
"artifact.list": "artifacts_list",
"artifact.getById": { key: "artifacts_get", renameIn: { artifact: "id" } },
"artifact.create": "artifacts_create",
"artifact.update": { key: "artifacts_update", renameIn: { artifact: "id" } },
"artifact.setPassword": { key: "artifacts_set-password", renameIn: { artifact: "id" } },
"artifact.delete": { key: "artifacts_delete", renameIn: { artifact: "id" } },
```

Verify the exact left-hand method names the ported UI in
`frontend/src/features/artifact/` (5 files) actually calls before landing
this — they were skimmed but not cross-checked line by line against this
mapping, for time.

## HTTP serving — NOT built this round, see below

`/v/{workspace}/artifacts/{id}/*` (path-traversal guard, strict CSP, no
session cookie, extension-derived Content-Type, unknown → attachment) is
specified in `docs/05 - Transporte/Artifacts e Estáticos.md` and in this
domain's own design doc, but was **not implemented** — see "What's missing"
below. `Service.Authorize` (three visibilities) is fully built and tested and
is what that transport layer should call before serving a file; `Files` in
`internal/adapters/artifactfiles` resolves paths inside one artifact's own
directory via `pathx.ResolveInside`, which the HTTP handler should reuse
rather than re-implementing containment.

## What's missing (be honest, not padded)

- **HTTP serving is not built.** The domain package (CRUD, scaffolding,
  password hashing/persistence, three-visibility authorization) is complete
  and tested; the `/v/*` transport layer that actually serves an artifact's
  files to a browser is not. This was a stretch goal per the task brief and
  was cut for time after three stalled attempts at this task ate the budget
  meant for it. The path-traversal and CSP/Content-Type requirements from the
  design doc are consequently untested at the HTTP layer — `artifactfiles`'s
  own tests prove the *filesystem* containment (`pathx.ResolveInside`
  refuses `../../../etc/passwd`), not a served HTTP response.
- Frontend `command-map.ts` mapping above is a best-effort sketch, not
  verified against the ported UI's actual call sites.
