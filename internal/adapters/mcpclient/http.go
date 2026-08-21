package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/toolset"
)

// streamableHTTP is the mcp-server::http implementation of toolset.Adapter:
// it speaks MCP over Streamable HTTP to a remote server named by ts.BaseURL.
//
// ListTools, Call and Close are identical to stdio's own — a *mcp.
// ClientSession behaves the same regardless of the transport underneath it —
// so this type embeds session and reuses them rather than duplicating three
// methods across two files.
type streamableHTTP struct {
	session
}

// NewHTTP builds a fresh mcp-server::http adapter.
//
// Its signature is already func() toolset.Adapter, so it is used directly as
// a toolset.Factory — Adapters{toolset.MCPHTTP: mcpclient.NewHTTP} — with no
// wrapping closure needed at the composition root.
func NewHTTP() toolset.Adapter { return &streamableHTTP{} }

// Connect opens a Streamable HTTP session against ts.BaseURL. ts arrives with
// every ${env.VAR} placeholder already resolved — this adapter never sees the
// literal placeholder, only the header value or setting it named.
func (h *streamableHTTP) Connect(ctx context.Context, ts toolset.Toolset) error {
	if ts.BaseURL == "" {
		return fmt.Errorf("mcp-server::http toolset %q has no baseUrl", ts.ID)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    build.Name + "-toolset-client",
		Version: build.Current().Version,
	}, nil)

	transport := &mcp.StreamableClientTransport{
		Endpoint:   ts.BaseURL,
		HTTPClient: &http.Client{Transport: headerRoundTripper{headers: ts.Headers}},
	}
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connecting to toolset %q over streamable HTTP: %w", ts.ID, err)
	}
	h.set(sess)
	return nil
}

// headerRoundTripper adds a toolset's own headers to every request — its
// declared Authorization or API key, never anything this daemon's process
// otherwise carries.
type headerRoundTripper struct {
	headers map[string]string
}

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// session is the transport-independent half of an mcpclient adapter: once
// connected, discovery and calling a tool do not care whether the session
// underneath is a spawned process or an HTTP connection.
type session struct {
	s *mcp.ClientSession
}

func (c *session) set(s *mcp.ClientSession) { c.s = s }

func (c *session) ListTools(ctx context.Context) ([]toolset.ToolSpec, error) {
	if c.s == nil {
		return nil, errNotConnected
	}
	var out []toolset.ToolSpec
	for tool, err := range c.s.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("listing tools: %w", err)
		}
		schema, _ := tool.InputSchema.(map[string]any)
		if schema != nil {
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

func (c *session) Call(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	if c.s == nil {
		return nil, errNotConnected
	}
	var args map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return nil, fmt.Errorf("tool %s: input is not a JSON object: %w", name, err)
		}
	}
	res, err := c.s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
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

func (c *session) Close() error {
	if c.s == nil {
		return nil
	}
	s := c.s
	c.s = nil
	return s.Close()
}
