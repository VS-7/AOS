// Package cliclient is the cli implementation of toolset.Adapter: it runs a
// local binary through the calling agent's own sandbox.
//
// It lives under internal/adapters, like every other Adapter, because it
// spawns an OS process — internal/architecture forbids os/exec anywhere
// under internal/domain. Unlike mcp-server::stdio, which spawns whatever the
// toolset's Command names with no further check, a cli toolset must also
// clear the calling agent's sandbox exec allowlist before it runs anything —
// the toolset domain's own decision doc calls this "duas portas, ambas
// fechadas por padrão": the binary needs to be on the agent's allowlist and
// to have been declared in the skill's manifest, and the manifest half is
// already enforced at skill install (internal/domain/skill.VerifyManifest)
// and protected from post-install drift (toolset.Service.UpdateConfig's
// lock). This package is the other door. The sandbox itself reaches this
// package through internal/runtime/execguard, read off ctx in Call — never
// through the Adapter interface, which stays identical across every
// connection type.
package cliclient

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/runtime/execguard"
	"github.com/OWNER/aos/internal/runtime/sandbox"
)

// errNotConnected fires when ListTools or Call runs before Connect.
var errNotConnected = errors.New("cli adapter: not connected")

// toolName is the one tool every cli toolset publishes.
//
// The original discovers subcommands from a CLI's own --help text. That is a
// heuristic over free-form text with no stable grammar across programs, and
// getting it wrong silently hides or mis-shapes a tool — the same reason
// rest-api trusts a machine-readable OpenAPI document instead of guessing at
// an HTML page. A cli toolset has no such document, so this adapter treats
// the whole configured command as one callable action: Toolset.Command and
// Toolset.Args are as exact a description of what it spawns as
// mcp-server::stdio's own fields already are.
const toolName = "run"

// Adapter is the cli implementation of toolset.Adapter. A fresh one is built
// per call — see toolset.Factory's own doc comment — so the configuration it
// holds never outlives the one Service.Call that owns it.
type Adapter struct {
	id          string
	command     string
	args        []string
	env         map[string]string
	description string
}

// New builds a fresh cli adapter. Its signature is already func()
// toolset.Adapter, so it is used directly as a toolset.Factory —
// Adapters{toolset.CLI: cliclient.New} — the same shape every other adapter's
// constructor gives its own type.
func New() toolset.Adapter { return &Adapter{} }

// Connect validates the toolset names a command and stores its
// configuration. It does no I/O: unlike mcp-server::stdio, which spawns its
// process at Connect, or rest-api, which fetches a document, a cli toolset
// runs nothing until a tool is actually called — see Call.
func (a *Adapter) Connect(_ context.Context, ts toolset.Toolset) error {
	if strings.TrimSpace(ts.Command) == "" {
		return errNoCommand(ts.ID)
	}
	a.id = ts.ID
	a.command = ts.Command
	a.args = append([]string(nil), ts.Args...)
	a.env = ts.Env
	a.description = ts.Description
	return nil
}

// ListTools reports the single "run" tool this toolset publishes.
func (a *Adapter) ListTools(context.Context) ([]toolset.ToolSpec, error) {
	if a.command == "" {
		return nil, errNotConnected
	}
	return []toolset.ToolSpec{{
		Name:        toolName,
		Description: describeTool(a.command, a.description),
		InputSchema: inputSchema(),
	}}, nil
}

// describeTool renders what ListTools tells the agent this tool does: the
// toolset's own description when it declared one, and a plain statement of
// the command otherwise.
func describeTool(command, description string) string {
	if description != "" {
		return description
	}
	return "Runs " + command + " through the calling agent's sandbox."
}

// inputSchema is the JSON Schema the run tool publishes: arguments appended
// after the toolset's own configured Args, and text piped to stdin, both
// optional.
func inputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Arguments appended after the toolset's own configured args.",
			},
			"stdin": map[string]any{
				"type":        "string",
				"description": "Text piped to the process's stdin.",
			},
		},
	}
}

// callInput is what Call decodes input into.
type callInput struct {
	Args  []string `json:"args,omitempty"`
	Stdin string   `json:"stdin,omitempty"`
}

// Call runs the configured command through the calling agent's sandbox,
// found on ctx — see execguard. A cli toolset called from outside an agent
// turn, where no sandbox exists to clear, refuses rather than running
// unguarded; that is the whole point of the second door.
func (a *Adapter) Call(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	if a.command == "" {
		return nil, errNotConnected
	}
	if name != toolName {
		return nil, errUnknownTool(name)
	}

	var in callInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, errBadInput(name, err)
		}
	}

	runner, ok := execguard.From(ctx)
	if !ok {
		return nil, errNoSandbox(a.id)
	}

	args := append(append([]string(nil), a.args...), in.Args...)
	result, err := runner.Run(ctx, sandbox.Command{
		Name:  a.command,
		Args:  args,
		Env:   envSlice(a.env),
		Stdin: []byte(in.Stdin),
	})
	if err != nil {
		return nil, errRunRefused(a.command, err)
	}

	raw, merr := json.Marshal(result)
	if merr != nil {
		return nil, errEncodeResult(name, merr)
	}
	return raw, nil
}

// envSlice renders a toolset's already-interpolated Env as the KEY=VALUE
// pairs sandbox.Command.Env expects, sorted so the child's environment is
// deterministic across runs — the same reason mcpclient's childEnv sorts.
func envSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// Close releases nothing — Connect opened no connection — and is always safe
// to call more than once, same as every other Adapter.
func (a *Adapter) Close() error { return nil }
