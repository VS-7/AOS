# Integrating the artifact domain

**Done**, all of it. `wire.go` builds and registers `artifactSvc`, on
`App.Artifacts`; `frontend/src/lib/command-map.ts` has
`artifact.list`/`.getById`/`.create`/`.update`/`.setPassword`/`.delete`
(`artifact` is not in `DORMANT_DOMAINS`); and HTTP serving at `/v/artifacts/{id}/*`
— the one gap this note used to track — is built, tested, and wired. See
"HTTP serving — done" below for what shipped and what's still genuinely open
beyond it.

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

## HTTP serving — done

`internal/transport/artifactapi` serves `/v/artifacts/{id}/*` (no
`{workspace}` segment — this daemon is single-workspace, see that package's
own doc), mounted via `httpapi.Config.Artifacts` at `/v`, outside the
authenticated `/api` group: it authorises itself per artifact, per
visibility, through `artifact.Service.Authorize`, and never accepts the
session cookie the guarded group's own middleware falls back to — only a
deliberately presented `Authorization`/`X-Auth-Token` header, or, for a
`by_password` artifact, the password itself via the query string.
Containment reuses `artifactfiles.Files.Resolve` (new, alongside `Ensure`) —
the same `pathx.ResolveInside` call, not a second implementation. CSP is
fixed and restrictive (`default-src 'self'`, no `unsafe-inline`); the
per-artifact relaxation the design doc's sketch anticipated has no field on
the entity yet, so every artifact gets the strict policy — a smaller,
disclosed gap, not the whole transport layer being missing.

Tested at three levels: `internal/transport/artifactapi`'s own suite against
fakes (traversal, directory-listing refusal, CSP headers, cookie vs. header
auth, password-via-query), and `TestArtifactsAreServedThroughTheRunningDaemon`/
`TestPrivateArtifactsRequireAuthenticationOnTheRunningDaemon` (`internal/app`)
against the real composition root — a real HTTP GET, over a real socket,
reaching a real artifact.Service and artifactfiles.Files.

## What's still open (be honest, not padded)

- Per-artifact CSP relaxation (opt-in, per the design doc's sketch) has no
  entity field — out of scope for this pass; every artifact is strict today.
- No frontend page exists yet to open an artifact from — `frontend/src/features/artifact/`
  has state-layer files (store/hooks/triggers) but no page component. The
  backend is directly reachable by URL; there is no "open" button in the app.
- Frontend `command-map.ts` mapping in this note is unchanged from the
  original sketch — still not verified against the ported UI's actual call
  sites, since no UI calls it yet.
