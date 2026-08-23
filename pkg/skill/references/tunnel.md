# Tunnel

Expose the local daemon on the public internet via Cloudflare Tunnel.

Start, stop and inspect a Cloudflare Tunnel over the system's cloudflared
binary. Requires cloudflared on PATH, and a hostname/token configured for the
tunnel.

Start refuses when the local API is not authenticated: opening a tunnel onto
an unauthenticated API publishes every route, including the docs playground,
to anyone who finds the hostname. Enable authentication and issue an API
token first.

## When to use
- To reach this daemon from outside the local network — a Telegram webhook
  needs one, and so does remote access to the workspace

## When NOT to use
- Not from an agent: there is no MCP tool for this group. Exposing the
  machine is configured by the person running it, from the CLI or the
  desktop settings.

## Commands

### `tunnel_start`

Publish the local daemon on the public internet.

Start cloudflared with the configured hostname and token.

Refuses with TUNNEL_INSECURE_EXPOSURE when the local API is not authenticated,
and with TUNNEL_CONFIG_INCOMPLETE when hostname or token are not set. Calling
this while already running or starting returns the current state rather than
erroring — the caller asked for it to be up, and it is (or is on its way).

- bring it up

### `tunnel_status`

Report whether the tunnel is running.

Read the tunnel's current state: stopped, starting, running (with its public URL) or failed.

- is it up

### `tunnel_stop`

Tear the tunnel down.

Stop cloudflared. The configured hostname and token are left in place, so a later start needs no reconfiguring. Idempotent: stopping what is already stopped succeeds.

- take it down

