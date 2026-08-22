// Package execguard carries the calling agent's sandbox through a turn, the
// same way internal/core/identity carries who is acting.
//
// A cli toolset's Call reaches an OS process exactly like mcp-server::stdio
// already does, but the toolset domain's own decision for cli is stricter:
// the binary must also clear the calling agent's own sandbox allowlist, not
// only the manifest the installing skill declared at install time — see
// internal/domain/toolset's decision doc, "duas portas, ambas fechadas por
// padrão". internal/domain/toolset cannot import internal/runtime/sandbox to
// express that itself — internal/architecture forbids internal/domain from
// importing internal/runtime — so this package is how
// internal/runtime/session, which builds the sandbox once per turn, reaches
// internal/adapters/cliclient three layers of context.Context away, with
// neither one importing the other.
package execguard

import (
	"context"

	"github.com/OWNER/aos/internal/runtime/sandbox"
)

type ctxKey struct{}

// With attaches the calling agent's sandbox to ctx. It is called once per
// turn by internal/runtime/session, before an agent can reach toolsets_call.
func With(ctx context.Context, runner sandbox.CommandRunner) context.Context {
	return context.WithValue(ctx, ctxKey{}, runner)
}

// From reads the sandbox attached to ctx. It returns false outside a turn —
// a direct call to toolsets_call over HTTP, MCP or the CLI, or a test that
// built no sandbox — which is exactly when a cli toolset must refuse rather
// than run unguarded.
func From(ctx context.Context) (sandbox.CommandRunner, bool) {
	runner, ok := ctx.Value(ctxKey{}).(sandbox.CommandRunner)
	return runner, ok
}
