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

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
func (s *stdio) Connect(ctx context.Context, ts toolset.Toolset) error {
	if ts.Command == "" {
		return fmt.Errorf("mcp-server::stdio toolset %q has no command", ts.ID)
	}

	cmd := exec.CommandContext(ctx, ts.Command, ts.Args...) //nolint:gosec // Command is workspace configuration, not a request payload
	cmd.Env = childEnv(ts.Env)

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

// childEnv builds the spawned process's environment from a minimal allowlist
// plus the toolset's own, already-interpolated Env — never this daemon's full
// environment, which the child has no business reading. Keys are sorted so
// the child's environment is deterministic across runs, which matters for
// diagnosing "it worked yesterday" reports.
func childEnv(extra map[string]string) []string {
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
		out = append(out, k+"="+extra[k])
	}
	return out
}
