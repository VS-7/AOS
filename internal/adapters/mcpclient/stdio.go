// Package mcpclient is the client side of the Model Context Protocol.
//
// internal/transport/mcpserver already uses this SDK's server side to publish
// this project's own commands as MCP tools; this package speaks the same
// protocol from the other seat, to reach an external MCP server a toolset
// names.
//
// It lives under internal/adapters, not internal/domain/toolset, because it
// spawns an OS process: internal/architecture forbids os/exec anywhere under
// internal/domain, and a violation fails the build rather than a review.
package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/toolset"
)

// errNotConnected is returned by ListTools and Call when neither has been
// preceded by a successful Connect.
var errNotConnected = errors.New("mcp-server::stdio: not connected")

// stdio is the mcp-server::stdio implementation of toolset.Adapter: it spawns
// the toolset's configured command and speaks MCP over its stdin/stdout.
type stdio struct {
	session *mcp.ClientSession
}

// NewStdio builds a fresh mcp-server::stdio adapter.
//
// Its signature is already func() toolset.Adapter, so it is used directly as
// a toolset.Factory — Adapters{toolset.MCPStdio: mcpclient.NewStdio} — with no
// wrapping closure needed at the composition root.
func NewStdio() toolset.Adapter { return &stdio{} }

// Connect spawns ts.Command and speaks MCP over its stdio.
//
// ts arrives with every ${env.VAR} placeholder already resolved by
// toolset.Service — this adapter never sees the literal placeholder, only the
// secret or setting it named.
//
// A value interpolated into Args lands in the child's argv, which is visible
// to any co-resident user for the process's lifetime via `ps`. Rejecting
// interpolation in Args outright was considered and rejected: most uses there
// are not secrets — a workspace path is the obvious one — and refusing all of
// them would break real configurations to guard against a minority case. The
// guard instead lives where the author of a toolset actually reads it: the
// jsonschema description on Toolset.Args says a resolved value is visible in
// the process table and that secrets belong in Env.
func (s *stdio) Connect(ctx context.Context, ts toolset.Toolset) error {
	if ts.Command == "" {
		return fmt.Errorf("mcp-server::stdio toolset %q has no command", ts.ID)
	}

	env, eerr := childEnv(ts.Env)
	if eerr != nil {
		return eerr
	}

	cmd := exec.CommandContext(ctx, ts.Command, ts.Args...) //nolint:gosec // Command is workspace configuration, not a request payload
	cmd.Env = env

	client := mcp.NewClient(&mcp.Implementation{
		Name:    build.Name + "-toolset-client",
		Version: build.Current().Version,
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return fmt.Errorf("connecting to toolset %q over stdio: %w", ts.ID, err)
	}
	s.session = session
	return nil
}

// ListTools discovers what the connected server currently publishes.
func (s *stdio) ListTools(ctx context.Context) ([]toolset.ToolSpec, error) {
	if s.session == nil {
		return nil, errNotConnected
	}
	var out []toolset.ToolSpec
	for tool, err := range s.session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("listing tools: %w", err)
		}
		schema, _ := tool.InputSchema.(map[string]any)
		if schema != nil {
			// The SDK's cache may still hold this map; a caller mutating what
			// ListTools returns must not be able to reach back into it.
			schema, _ = collections.CloneJSON(schema).(map[string]any)
		}
		out = append(out, toolset.ToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}
	return out, nil
}

// Call runs one tool. input and the returned payload are opaque JSON, relayed
// without interpretation.
func (s *stdio) Call(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	if s.session == nil {
		return nil, errNotConnected
	}
	var args map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return nil, fmt.Errorf("tool %s: input is not a JSON object: %w", name, err)
		}
	}

	res, err := s.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", name, err)
	}
	raw, merr := json.Marshal(res)
	if merr != nil {
		return nil, fmt.Errorf("encoding the result of %s: %w", name, merr)
	}
	if res.IsError {
		return raw, fmt.Errorf("tool %s reported an error", name)
	}
	return raw, nil
}

// Close releases the connection and terminates the spawned process. It is
// safe to call more than once, and safe to call without a prior successful
// Connect — Service.Call defers it unconditionally.
func (s *stdio) Close() error {
	if s.session == nil {
		return nil
	}
	session := s.session
	s.session = nil
	return session.Close()
}

// reservedEnvKey reports whether a toolset's Env may not set k.
//
// PATH and HOME are refused because the daemon chooses them for the child,
// not the toolset — os/exec documents that a duplicate key in Env means the
// last value wins, so a toolset declaring its own PATH would silently
// override the one childEnv just built.
//
// Anything beginning LD_ or DYLD_ is refused as a blanket prefix rule rather
// than an exhaustive name list (LD_PRELOAD, LD_LIBRARY_PATH,
// DYLD_INSERT_LIBRARIES, DYLD_LIBRARY_PATH, ...): a name list is a list
// somebody has to keep current against every dynamic loader variable a
// platform ever ships, and missing one silently reopens exactly this hole. A
// toolset declaration arrives inside an installable skill package as of the
// next task; without this, that package could inject a shared library into
// every process this daemon spawns on someone's behalf.
func reservedEnvKey(k string) bool {
	switch k {
	case "PATH", "HOME":
		return true
	}
	return strings.HasPrefix(k, "LD_") || strings.HasPrefix(k, "DYLD_")
}

// childEnv builds the spawned process's environment from a minimal allowlist
// plus the toolset's own, already-interpolated Env — never this daemon's full
// environment, which the child has no business reading. Keys are sorted so
// the child's environment is deterministic across runs, which matters for
// diagnosing "it worked yesterday" reports.
//
// A reserved key is refused rather than silently dropped: dropping it would
// leave the toolset's author with a variable that quietly has no effect, and
// this codebase has had to undo that silent-loss pattern before. Refusing
// names the variable, in the same shape as toolset.Interpolate's own failure
// on a missing ${env.VAR}.
func childEnv(extra map[string]string) ([]string, error) {
	out := []string{"PATH=" + os.Getenv("PATH")}
	if home := os.Getenv("HOME"); home != "" {
		out = append(out, "HOME="+home)
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if reservedEnvKey(k) {
			return nil, errEnvRejected(k)
		}
		out = append(out, k+"="+extra[k])
	}
	return out, nil
}

// errEnvRejected fires when a toolset's Env sets a variable this adapter
// reserves for itself: PATH, HOME, or anything controlling the dynamic
// loader.
func errEnvRejected(name string) error {
	return apperr.New("MCPCLIENT_ENV_REJECTED").
		Causer("mcpclient.stdio.Connect").
		Msgf("toolset env may not set %q: it is reserved by this adapter", name).
		Issue("variable", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "remove PATH, HOME, and any LD_*/DYLD_* variable from the toolset's env",
		})
}
