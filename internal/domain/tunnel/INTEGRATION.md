# Integrating the tunnel domain

Not merged here on purpose — `internal/app/wire.go` was being edited by
another agent in parallel this round. Everything below is what `wire.go`
needs, verified to compile against this branch's `internal/domain/tunnel` and
`internal/adapters/cloudflaredproc`.

## `internal/domain/config` — nothing to add

`config.Security` (`Enabled`, `APIToken`) and `config.Tunnel` (`Hostname`,
`Token`) already exist with exactly the fields this domain reads — no schema
change was needed.

## `wire.go` changes

**Imports**, alongside the other domain/adapter imports:

```go
"github.com/OWNER/aos/internal/adapters/cloudflaredproc"
"github.com/OWNER/aos/internal/domain/tunnel"
```

**Config adapter.** `tunnel.Service` depends on the narrow `tunnel.Config`
port, not `config.Service` directly (same narrowing every domain here does).
Add this adapter type near the other small adapters in `wire.go` or
`ecosystem.go`:

```go
// tunnelConfig adapts config.Service to tunnel.Config: the two substructures
// Start's guard reads, kept separate so tunnel does not import config's full
// surface just to name a type.
type tunnelConfig struct{ svc config.Service }

func (c tunnelConfig) Raw(ctx context.Context) (tunnel.RawConfig, error) {
	cfg, err := c.svc.Raw(ctx)
	if err != nil {
		return tunnel.RawConfig{}, err
	}
	return tunnel.RawConfig{
		SecurityEnabled: cfg.Security.Enabled,
		APIToken:        cfg.Security.APIToken,
		Hostname:        cfg.Tunnel.Hostname,
		Token:           cfg.Tunnel.Token,
	}, nil
}
```

**Service construction**, after `configSvc` exists (it already does, early in
`New`):

```go
tunnelSvc := tunnel.NewService(tunnel.Deps{
	Config: tunnelConfig{svc: configSvc},
	Runner: cloudflaredproc.New(),
	Clock:  clock,
	Log:    logger,
})
```

**Command registration**, alongside the other `.Register(reg, ...)` calls:

```go
tunnel.Register(reg, tunnelSvc)
```

**`App` struct field**, alongside `Gateway`:

```go
// Tunnel exposes the local daemon on the public internet via Cloudflare
// Tunnel. No MCP surface — see tunnel.Register's own doc.
Tunnel tunnel.Service
```

...and in the `return &App{...}` literal:

```go
Tunnel: tunnelSvc,
```

No `repoSet`/`fscollections` entry — this domain has no collection-backed
entity, only process state kept in memory (see `entity.go`'s own doc comment).

No `Freeze()`-order concern beyond the usual: register before `reg.Freeze()`,
same as every other domain.

## `frontend/src/lib/command-map.ts`

None needed. `tunnel` is not in the current `DORMANT_DOMAINS` set (only
`artifact`, `goal`, `instruction`, `marketplace`, `model`, `project`,
`template`, `token`, `tunnel`, `user` are — wait, it *is* listed). Remove
`"tunnel"` from `DORMANT_DOMAINS`, but there is **no COMMAND_MAP entry to
add**: this group has `Registry: false` (CLI/HTTP only, no MCP/agent tool —
see `commands.go`'s own doc), and — per the design doc — "Continua sem tools
MCP... Configurar canal externo é ação de configuração humana." The frontend
settings panel, if/when built, should call `tunnel_start` / `tunnel_stop` /
`tunnel_status` over HTTP directly rather than through the agent-facing
COMMAND_MAP the way `view.*`/`toolset.*` do — those three commands are
`Local: true` (CLI/HTTP surfaces only), which HTTP still reaches even with
`Registry: false` (that field gates the *agent's own* tool list, not the
HTTP/CLI surfaces — see `commands.go`'s doc comment on `Register`).

`frontend/src/features/tunnel/` has only 1 file (a stub) — no rich settings
panel exists to wire up yet; out of scope for this pass.

## A note on "no MCP tools"

The registry does not have a literal per-surface MCP toggle — `Sorted()`
(what `internal/transport/mcpserver` publishes) returns every registered
command regardless of `Registry`. What actually keeps a domain "out of the
agent's reach" in this codebase is `Registry: false`, which excludes it from
the *agent's own* internal tool list (`internal/core/command/registry.go`'s
`add`, `r.registry`) — the list `internal/domain/gateway/commands.go`'s own
comment names explicitly: *"gateway, auth and tunnels stay out of the
agent's reach."* `tunnel/commands.go` mirrors that exact mechanism. If the
intent is also to hide it from a raw MCP client (not just this system's own
agent loop), that would need a change to `mcpserver.RegisterFlat`/
`RegisterComposite` to filter on `InRegistry()` too — out of scope here since
it would affect `gateway`'s and `auth`'s existing behavior as well, not just
`tunnel`'s.
