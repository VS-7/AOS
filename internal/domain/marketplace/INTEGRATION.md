# Integrating `marketplace` into `wire.go`

Not done here on purpose — every Phase 8 domain agent this round was asked
to leave `internal/app/wire.go` and `frontend/src/lib/command-map.ts` to a
single integration pass, to avoid an N-way merge conflict on two files
every domain touches. This is that pass's checklist for `marketplace`.

## `internal/domain/skill` changed

`internal/domain/skill/install.go`'s `Installer.Install` used to do
fetch-then-verify-then-consent-then-apply as one unexported sequence. It
now does `Fetch` and then `InstallPackage` (newly exported), which is the
verify/consent/apply half. Nothing about `Install`'s own signature, order,
or behaviour changed — `go test ./internal/domain/skill/...` is green
unmodified. This was necessary because `Installer` is built with exactly
one `Fetcher` (`Deps.Fetcher`), wired once at boot to the local-directory
fetcher (`skillfetch.New()`); `marketplace.Service` needs to drive its own
`Registry.Fetch` (Git or HTTP, neither known to `skill`) and still go
through the one verify/consent/apply path every install obeys, rather than
`Installer` growing a second `Fetcher` slot to pick between at construction
time.

## `wire.go` additions

Import block:

```go
"github.com/OWNER/aos/internal/adapters/marketplacegit"
"github.com/OWNER/aos/internal/adapters/marketplacehttp"
"github.com/OWNER/aos/internal/domain/marketplace"
```

Registries come from config — `~/.aos/config.json`'s
`"marketplace": {"registries": [...], "requireSignature": false}`
(see the design doc, `docs/04 - Domínio/Marketplace (Go).md`). This
package does not read config itself, matching every other domain here —
build the map once in `New`, near where `configSvc` is constructed:

```go
// marketplaceRegistries builds one Registry per configured entry. Read
// from cfg.Marketplace.Registries once config.Service exposes that field —
// it does not yet; see "config.Service gap" below.
func marketplaceRegistries(entries []config.MarketplaceRegistry) (map[string]marketplace.Registry, []string) {
	regs := make(map[string]marketplace.Registry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		switch e.Type {
		case "git":
			regs[e.ID] = marketplacegit.New(e.URL)
		case "http":
			regs[e.ID] = marketplacehttp.New(e.URL)
		}
		order = append(order, e.ID)
	}
	return regs, order
}
```

Service construction, alongside `skillInstaller` (marketplace needs it as
its `Installer` dependency — `*skill.Installer` already satisfies
`marketplace.Installer` after the `InstallPackage` export above, no
adapter needed):

```go
marketplaceRegs, marketplaceOrder := marketplaceRegistries(cfg.Marketplace.Registries) // see config gap below
marketplaceSvc := marketplace.NewService(marketplace.Deps{
	Registries: marketplaceRegs,
	Order:      marketplaceOrder,
	Installer:  skillInstaller,
})
```

Register call, next to the other ecosystem domains:

```go
marketplace.Register(reg, marketplaceSvc)
```

`App` struct field:

```go
Marketplace *marketplace.Service
```

and in the return literal:

```go
Marketplace: marketplaceSvc,
```

### `config.Service` gap

`internal/domain/config` has no `Marketplace` section yet — this agent was
not authorized to touch other domains' packages beyond the small, explicit
`skill.InstallPackage` export above, so `config.Service`/`corecfg` needs a
`Marketplace.Registries []MarketplaceRegistry{ID, Type, URL}` field (and
`RequireSignature bool`, unused by this package today but named in the
design doc) added before the snippet above compiles as written. Until
then, `New` can wire `marketplace.Service` with an empty
`Deps.Registries` — `Discovery`/`Install` both refuse cleanly with
`MARKETPLACE_NO_REGISTRIES_CONFIGURED`, so the domain is safe to register
even before config catches up.

## `command-map.ts` additions

Remove `"marketplace"` from `DORMANT_DOMAINS` in
`frontend/src/lib/command-map.ts`. Add, following the `toolset.*`/`view.*`
entries already there as the template:

```ts
"marketplace.discovery": "marketplace_discovery",
"marketplace.get": { key: "marketplace_get", renameIn: { source: "source" } },
"marketplace.install": "marketplace_install",
```

`frontend/src/features/marketplace/` already has 16 files ported from the
original product — not read in this pass (time went to the backend and its
tests); the integrator should confirm the ported UI's expected response
shape against `Listing`'s json tags (`registry`, `source`, `name`,
`description`, `version`, `tags`, `stars`, `updatedAt`, `permissions`)
before wiring the frontend calls live.
