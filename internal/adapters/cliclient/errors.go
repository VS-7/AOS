package cliclient

import (
	"fmt"

	"github.com/OWNER/aos/internal/core/apperr"
)

// errNoCommand fires when a cli toolset has no Command to run.
func errNoCommand(id string) error {
	return fmt.Errorf("cli toolset %q has no command", id)
}

// errUnknownTool fires when Call names anything other than "run" — the one
// tool this adapter publishes.
func errUnknownTool(name string) error {
	return fmt.Errorf("no tool named %q — this cli toolset only publishes %q", name, toolName)
}

// errBadInput fires when Call's input is not the {args, stdin} shape ListTools
// published.
func errBadInput(name string, cause error) error {
	return fmt.Errorf("tool %s: input is not a valid {args, stdin} object: %w", name, cause)
}

// errNoSandbox fires when Call runs with no calling agent's sandbox attached
// to ctx. It is the one error in this package worth its own apperr code and
// CTA: everything else here is an ordinary configuration mistake, but this
// one is the second of the "duas portas" the toolset domain's decision doc
// requires closed by default — a cli toolset reached any way other than
// through a running agent turn has no sandbox to clear, and must refuse
// rather than run unguarded.
func errNoSandbox(id string) error {
	return apperr.New("CLICLIENT_NO_SANDBOX").
		Causer("cliclient.Adapter.Call").
		Msgf("toolset %q is a cli toolset, and a cli toolset can only run inside an agent turn — no sandbox is attached to this call", id).
		Issue("toolset", id).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "call this toolset through toolsets_call from a running agent turn, not directly",
		})
}

// errRunRefused fires when the calling agent's sandbox refuses to run the
// command — the binary is not on the agent's exec allowlist, the agent lacks
// the execute permission at all, or the rendered command line matches a
// denied pattern. See internal/runtime/sandbox.Sandbox.VerifyExec for the
// three layers this can fail at.
func errRunRefused(command string, cause error) error {
	return fmt.Errorf("running %q: %w", command, cause)
}

// errEncodeResult fires on the near-impossible case that sandbox.Result — a
// plain struct of strings and numbers — fails to marshal.
func errEncodeResult(name string, cause error) error {
	return fmt.Errorf("tool %s: encoding the result: %w", name, cause)
}
