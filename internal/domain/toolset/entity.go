// Package toolset connects external tools to the agent — MCP servers, REST
// APIs, CLIs — without letting any of them into the agent's tool registry.
//
// A toolset's tools are reached through exactly one call, toolsets_call, never
// through a per-tool entry the agent picks from a growing list. Three reasons
// carry over from the product this rebuilds — the agent's context does not
// grow with the number of integrations wired up, and discovery happens on
// demand instead of at every boot — and a fourth matters most here: the
// boundary where this system executes something outside its own process is
// auditable in exactly one place, Service.Call. A design that let a toolset
// register its tools directly into the agent's registry was the alternative,
// and it was rejected for exactly that reason — the audit trail would have had
// as many holes as there are connection types.
package toolset

import (
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Type is the discriminated union of connection kinds a toolset can be. It is
// closed on purpose: ParseType refuses anything outside the five constants
// rather than defaulting to one of them, because a toolset that silently
// became "custom" would connect to nothing and say nothing about why.
type Type string

const (
	// MCPStdio spawns a local process and speaks MCP over its stdin/stdout.
	// It is the only type this slice implements — see internal/adapters/mcpclient.
	MCPStdio Type = "mcp-server::stdio"

	// MCPHTTP speaks MCP over Streamable HTTP to a remote server.
	MCPHTTP Type = "mcp-server::http"

	// RESTAPI wraps a plain REST API behind a declared set of tools.
	RESTAPI Type = "rest-api"

	// CLI wraps a command-line program, one tool per subcommand.
	CLI Type = "cli"

	// Custom is an escape hatch for a connection this vocabulary has no name
	// for yet.
	Custom Type = "custom"
)

// types lists every member of the union, in declaration order.
var types = []Type{MCPStdio, MCPHTTP, RESTAPI, CLI, Custom}

// ParseType decodes a wire value into a Type, refusing anything the union does
// not name.
func ParseType(raw string) (Type, error) {
	t := Type(raw)
	for _, known := range types {
		if t == known {
			return t, nil
		}
	}
	return "", errTypeUnknown(raw)
}

// Valid reports whether t is one of the five declared members.
func (t Type) Valid() bool {
	for _, known := range types {
		if t == known {
			return true
		}
	}
	return false
}

// Status is where a toolset sits in its own lifecycle, independent of whether
// any particular call to it succeeds.
type Status string

const (
	// StatusEnabled is available to Call. It is the default: a toolset just
	// configured is assumed usable until proven otherwise.
	StatusEnabled Status = "enabled"

	// StatusDisabled is kept for reference but refused by Call. Deleting a
	// misbehaving toolset loses the configuration that took someone ten
	// minutes to get right; disabling it keeps that work while stopping the
	// calls.
	StatusDisabled Status = "disabled"
)

// Valid reports whether s is one of the two lifecycle states.
func (s Status) Valid() bool { return s == StatusEnabled || s == StatusDisabled }

// Toolset is one external connection, configured once and called by ID
// thereafter.
//
// The fields below cover all five Types even though only MCPStdio and
// MCPHTTP are implemented in this slice: a toolset of an unimplemented type
// still decodes and lists, it just refuses Call with a "not available in
// this build" error — see errTypeNotAvailable. That is what lets the
// remaining three types be added later by writing an adapter, not by
// widening this struct.
type Toolset struct {
	// ID identifies this toolset. It lives in the path, exactly like every
	// other native collection record:
	// .aos/toolsets/{id}.toolset.md.
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this toolset, used to address it in toolsets_call."`

	Type        Type   `yaml:"type" json:"type" jsonschema:"One of: mcp-server::stdio, mcp-server::http, rest-api, cli, custom. rest-api, cli and custom decode and list but are not connectable in this build."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this toolset is for, read by whoever decides whether to call it."`
	Status      Status `yaml:"status" json:"status" jsonschema:"enabled or disabled. A disabled toolset refuses Call without losing its configuration."`

	// Command and Args are the mcp-server::stdio connection: the binary to
	// spawn and its arguments. Either may contain ${env.VAR} placeholders,
	// resolved just before Connect — see Interpolate.
	Command string   `yaml:"command,omitempty" json:"command,omitempty" jsonschema:"Binary to spawn for mcp-server::stdio. May reference \\${env.VAR}."`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty" jsonschema:"Arguments passed to Command. Each may reference \\${env.VAR} — but a value resolved here lands in the child process's argv, visible to any co-resident user via ps for the process's lifetime. Put secrets in Env, not here."`

	// Env is passed to the spawned process, on top of a minimal PATH/HOME —
	// never the daemon's own environment, which the child has no business
	// reading. Every value may reference ${env.VAR}, which is how a secret
	// gets to a toolset without being written into its configuration file.
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty" jsonschema:"Environment passed to the spawned process. Values may reference \\${env.VAR}."`

	// BaseURL and Headers are unused by mcp-server::stdio; they exist for the
	// four types this slice does not implement, mcp-server::http and
	// rest-api chief among them.
	BaseURL string            `yaml:"baseUrl,omitempty" json:"baseUrl,omitempty" jsonschema:"Base URL for mcp-server::http and rest-api. May reference \\${env.VAR}."`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" jsonschema:"HTTP headers for mcp-server::http and rest-api. Values may reference \\${env.VAR}."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When this toolset was configured."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`
}

// Clone returns a Toolset that shares no mutable state with t: Args, Env and
// Headers are copied rather than aliased.
//
// This exists because this codebase has shipped the same defect four times
// already — a map or slice crossing an in-memory boundary by reference, so
// that one caller's edit corrupts what every other caller is served. A
// Toolset carries three such fields, and every function in this package that
// hands one out or takes one in calls Clone at the boundary rather than
// trusting the caller, or itself later, not to mutate what is still shared.
func (t Toolset) Clone() Toolset {
	c := t
	c.Args = cloneSlice(t.Args)
	c.Env = cloneStrings(t.Env)
	c.Headers = cloneStrings(t.Headers)
	return c
}

func cloneStrings(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneSlice(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

// ToolSpec is what a connected toolset says one of its tools is: the shape
// Adapter.ListTools returns, and what an agent reads through toolsets_call
// with a discovery call before it decides to invoke.
type ToolSpec struct {
	Name        string `json:"name" jsonschema:"Name of the tool, as the connected target publishes it."`
	Description string `json:"description,omitempty" jsonschema:"What the tool does, read by the agent deciding whether to call it."`

	// InputSchema is the tool's own JSON Schema, relayed opaquely: this
	// package neither validates against it nor rewrites it.
	InputSchema map[string]any `json:"inputSchema,omitempty" jsonschema:"JSON Schema of the tool's arguments, as the connected target publishes it."`
}

// Clone returns a ToolSpec whose InputSchema does not alias t's — the same
// rule Toolset.Clone applies, for the one field here that nests arbitrarily
// deep rather than one level.
func (t ToolSpec) Clone() ToolSpec {
	c := t
	if t.InputSchema != nil {
		c.InputSchema, _ = collections.CloneJSON(t.InputSchema).(map[string]any)
	}
	return c
}
