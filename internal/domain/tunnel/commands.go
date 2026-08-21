package tunnel

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a human reads on the CLI. There is no MCP/agent surface —
// Registry stays false below, the same boundary internal/domain/gateway
// documents: "gateway, auth and tunnels stay out of the agent's reach."
// Publishing this machine to the internet is a decision a person makes, not
// a capability an agent gets to reach for.
var GroupDoc = command.GroupDoc{
	Name:    "tunnel",
	Tool:    "Tunnel",
	Summary: "Expose the local daemon on the public internet via Cloudflare Tunnel.",
	Doc: `Start, stop and inspect a Cloudflare Tunnel over the system's cloudflared
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
  desktop settings.`,
	Hint: `Stop preserves the configured hostname and token — reconnecting later needs
no reconfiguring, only Start again.`,
}

// StatusInput takes nothing — Status has no filter.
type StatusInput struct {
	command.Reasoning
}

// StartInput starts the tunnel.
type StartInput struct {
	command.Reasoning
}

// StopInput stops the tunnel.
type StopInput struct {
	command.Reasoning
}

// Register declares the group on the registry. Local because cloudflared runs
// on this machine, not over a remote API — no --base-url makes sense here,
// the same reasoning internal/domain/gateway's own Register documents.
// Registry stays false: this group is CLI/HTTP only.
func Register(reg *command.Registry, svc Service) {
	reg.DescribeGroup(GroupDoc)

	statusHandler := func(ctx context.Context, in StatusInput) (State, error) { return svc.Status(ctx) }
	command.MustRegister(reg, command.Command[StatusInput, State]{
		Group:   "tunnel",
		Name:    "status",
		Summary: "Report whether the tunnel is running.",
		Doc:     "Read the tunnel's current state: stopped, starting, running (with its public URL) or failed.",
		Examples: []command.Example{
			{Description: "is it up", Input: StatusInput{}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Tunnel status", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     statusHandler,
	})

	startHandler := func(ctx context.Context, in StartInput) (State, error) { return svc.Start(ctx) }
	command.MustRegister(reg, command.Command[StartInput, State]{
		Group:   "tunnel",
		Name:    "start",
		Summary: "Publish the local daemon on the public internet.",
		Doc: `Start cloudflared with the configured hostname and token.

Refuses with TUNNEL_INSECURE_EXPOSURE when the local API is not authenticated,
and with TUNNEL_CONFIG_INCOMPLETE when hostname or token are not set. Calling
this while already running or starting returns the current state rather than
erroring — the caller asked for it to be up, and it is (or is on its way).`,
		Examples: []command.Example{
			{Description: "bring it up", Input: StartInput{}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Start the tunnel", OpenWorldHint: true},
		Handler:     startHandler,
	})

	stopHandler := func(ctx context.Context, in StopInput) (State, error) { return svc.Stop(ctx) }
	command.MustRegister(reg, command.Command[StopInput, State]{
		Group:   "tunnel",
		Name:    "stop",
		Summary: "Tear the tunnel down.",
		Doc:     "Stop cloudflared. The configured hostname and token are left in place, so a later start needs no reconfiguring. Idempotent: stopping what is already stopped succeeds.",
		Examples: []command.Example{
			{Description: "take it down", Input: StopInput{}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Stop the tunnel", IdempotentHint: true},
		Handler:     stopHandler,
	})
}
