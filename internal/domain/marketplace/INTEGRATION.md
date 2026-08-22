# Integrating `marketplace` into `wire.go`

**Done.** `internal/app/wire.go` reads `cfg.Marketplace.Registries` (the
`config.Marketplace` section this note asked for exists —
`internal/domain/config/entity.go:102`, including `RequireSignature`),
builds one `marketplace.Registry` per entry via `marketplaceRegistries`,
constructs `marketplaceSvc` with `skillInstaller` as its `Installer`,
registers it, and exposes it on `App.Marketplace`.
`frontend/src/lib/command-map.ts` has `marketplace.list` →
`marketplace_discovery`, `marketplace.getByName` → `marketplace_get`, and
`marketplace.install` → `marketplace_install` (`marketplace` is not in
`DORMANT_DOMAINS`) — the actual command names differ slightly from this
note's original guess (`marketplace.discovery`/`marketplace.get`), the
integrator's call, not a bug.

`internal/domain/skill/install.go`'s `Fetch`/`InstallPackage` split this
note asked for is exactly what shipped — `marketplace.Service` drives its
own `Registry.Fetch` (Git or HTTP) and then goes through
`skillInstaller.InstallPackage` for the verify/consent/apply path every
install obeys.

Both `Registry` implementations are real and tested, not stubs:
`internal/adapters/marketplacegit` (clone + local index read) and
`internal/adapters/marketplacehttp` (remote index over HTTP), each proven
against the shared `Registry` contract suite
(`TestRegistryObeysTheContract` in both packages) plus their own behaviour
tests (search filters, fetch of an unlisted source, an unreachable registry
degrading with a clear error instead of hanging the CLI).
