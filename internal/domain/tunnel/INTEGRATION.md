# Integrating the tunnel domain

**Done.** `internal/app/wire.go` builds `tunnelSvc` (`Config:
tunnelConfig{svc: configSvc}, Runner: cloudflaredproc.New(), Clock, Log`),
registers it (`tunnel.Register(reg, tunnelSvc)`), and exposes it on
`App.Tunnel`. `bot`'s registry construction depends on `tunnelSvc` too
(`tunnelPublicURL` adapter — see `internal/domain/bot/INTEGRATION.md`),
confirming the `tunnel → bots` boot order this note called for.
`frontend/src/lib/command-map.ts` has `tunnel.getStatus`/`.start`/`.stop`
(`tunnel` is not in `DORMANT_DOMAINS`) — contrary to what this note
originally guessed ("no COMMAND_MAP entry to add"), the integrator did add
one, mapping to `tunnel_status`/`tunnel_start`/`tunnel_stop`.

`config.Security`/`config.Tunnel` needed no schema change, as this note
predicted.

**Still open, not a wiring gap:** `frontend/src/features/tunnel/` has only 1
file (interfaces, no page component) — there is no settings panel to
actually click `tunnel.start`/`.stop` from. The backend and the command
mapping are both ready for one.

## A note on "no MCP tools" (still the mechanism in use)

`tunnel/commands.go` uses `Registry: false` to keep tunnel out of the
agent's own internal tool list, the same mechanism `gateway`'s and `auth`'s
commands use (`internal/domain/gateway/commands.go`'s comment: *"gateway,
auth and tunnels stay out of the agent's reach"*). `Sorted()` — what
`internal/transport/mcpserver` publishes — still returns every registered
command regardless of `Registry`; hiding tunnel from a raw MCP client too
would need `mcpserver.RegisterFlat`/`RegisterComposite` to filter on
`InRegistry()`, which would also affect `gateway` and `auth` and was out of
scope for this domain alone.
