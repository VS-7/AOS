// Package mcpproxy speaks MCP on stdio for clients that can only spawn a
// subprocess, and forwards every call to the daemon's own /mcp.
//
// This is what `aos --mcp` runs. The obvious alternative — the client binary
// hosting the tool surface itself — is ruled out by the dependency rule: the
// registry needs the domain, and the domain lives in aosd. Running `aosd --mcp`
// directly would work but is a second process owning the workspace next to the
// daemon that already does, which is the one-writer property this system is
// built around. A proxy keeps the daemon as the only place the domain runs and
// still gives an MCP client that has no HTTP transport (or no way to send a
// bearer token) a `command` to spawn.
//
// The tools an MCP client sees through the proxy are the daemon's tools as the
// daemon publishes them — same names, same schemas, same annotations, same
// server instructions — because they are read from the daemon at startup
// rather than described a second time here.
package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/apperr"
)

// Options say where the daemon is and how to identify to it.
type Options struct {
	// Endpoint is the daemon's MCP mount, e.g. http://127.0.0.1:5326/mcp.
	Endpoint string
	// Token is the bearer credential the daemon's /mcp mount requires when
	// security is on. Empty is allowed, and is what a daemon with security
	// off accepts.
	Token string
	// Workspace scopes every forwarded call, sent as the same header the
	// desktop and the CLI send. Empty leaves the daemon's own default.
	Workspace string

	// Agent is who the client is calling as, when it is one of the
	// workspace's own agents. A coding agent that sets AOS_AGENT_ID is that
	// agent through this proxy too — otherwise every tool call arrives
	// anonymous and the ones that need an identity (agents_me, every memory)
	// fail with nothing the client can do about it.
	Agent string

	// WorkingDir is where the client stands, forwarded as the same
	// X-Working-Dir header the CLI sends.
	//
	// Without it a coding agent operating AOS from inside repository B
	// addressed whatever the daemon's primary workspace happened to be:
	// `workspace_introspect` registered the daemon's own directory, and
	// every read and write landed in another repository's `.aos/` with
	// nothing to say it had.
	WorkingDir string

	// Name and Version identify this proxy to the daemon and to the client
	// on stdio.
	Name    string
	Version string

	// HTTPClient is the base client for the upstream connection. Nil uses a
	// client with a generous timeout: a tool call can legitimately take as
	// long as the agent turn behind it.
	HTTPClient *http.Client
}

// Proxy is a connected upstream session and the stdio-facing server that
// mirrors it.
type Proxy struct {
	server  *mcp.Server
	session *mcp.ClientSession
}

// Connect opens the upstream session, reads the daemon's tool list and
// builds the local server that forwards to it. Close must be called when
// done, which Serve does.
func Connect(ctx context.Context, opts Options) (*Proxy, error) {
	if strings.TrimSpace(opts.Endpoint) == "" {
		return nil, fmt.Errorf("mcpproxy: Options.Endpoint is empty")
	}
	if opts.Name == "" {
		opts.Name = "aos"
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}

	base := opts.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: 10 * time.Minute}
	}
	upstream := *base
	upstream.Transport = &identifying{
		next: base.Transport, token: opts.Token,
		workspace: opts.Workspace, agent: opts.Agent, workingDir: opts.WorkingDir,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: opts.Name + "-proxy", Version: opts.Version}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   opts.Endpoint,
		HTTPClient: &upstream,
	}, nil)
	if err != nil {
		return nil, errUpstream(opts.Endpoint, err)
	}

	instructions := ""
	if init := session.InitializeResult(); init != nil {
		instructions = init.Instructions
	}
	server := mcp.NewServer(&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		&mcp.ServerOptions{Instructions: instructions})

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			_ = session.Close()
			return nil, errUpstream(opts.Endpoint, err)
		}
		server.AddTool(mirror(tool), forward(session, tool.Name))
	}

	return &Proxy{server: server, session: session}, nil
}

// Server is the stdio-facing side, for a caller that wants to run it over a
// transport of its own — a test over mcp.NewInMemoryTransports, say.
func (p *Proxy) Server() *mcp.Server { return p.server }

// Run serves the mirrored surface over t until the client disconnects or ctx
// ends.
func (p *Proxy) Run(ctx context.Context, t mcp.Transport) error {
	return p.server.Run(ctx, t)
}

// Close ends the upstream session.
func (p *Proxy) Close() error { return p.session.Close() }

// Serve is the whole of `aos --mcp`: connect to the daemon, mirror its tools
// on stdio, and forward until the client hangs up.
func Serve(ctx context.Context, opts Options) error {
	proxy, err := Connect(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = proxy.Close() }()
	return proxy.Run(ctx, &mcp.StdioTransport{})
}

// mirror copies what the daemon said about a tool into a definition the local
// server can publish. Every field a client reads is carried over; the schema
// travels as the JSON object it arrived as, which the SDK accepts verbatim.
func mirror(tool *mcp.Tool) *mcp.Tool {
	return &mcp.Tool{
		Name:         tool.Name,
		Title:        tool.Title,
		Description:  tool.Description,
		InputSchema:  tool.InputSchema,
		OutputSchema: tool.OutputSchema,
		Annotations:  tool.Annotations,
		Icons:        tool.Icons,
		Meta:         tool.Meta,
	}
}

// forward is the handler of every mirrored tool: hand the raw arguments to
// the daemon under the same name, and return whatever it returned — a
// result the daemon marked as an error stays an error result, so the client
// reads the daemon's own explanation and call to action rather than a
// proxy's paraphrase of it.
func forward(session *mcp.ClientSession, name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.Params.Arguments
		if len(args) == 0 {
			args = json.RawMessage("{}")
		}
		return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	}
}

// identifying adds the daemon's credentials to every upstream request.
type identifying struct {
	next       http.RoundTripper
	token      string
	workspace  string
	agent      string
	workingDir string
}

func (i *identifying) RoundTrip(req *http.Request) (*http.Response, error) {
	next := i.next
	if next == nil {
		next = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	if i.token != "" {
		cloned.Header.Set("Authorization", "Bearer "+i.token)
	}
	if i.workspace != "" {
		cloned.Header.Set("X-Workspace-Id", i.workspace)
	}
	if i.agent != "" {
		cloned.Header.Set("X-Agent-Id", i.agent)
	}
	if i.workingDir != "" {
		cloned.Header.Set("X-Working-Dir", i.workingDir)
	}
	return next.RoundTrip(cloned)
}

func errUpstream(endpoint string, cause error) error {
	return apperr.New("MCP_DAEMON_UNREACHABLE").
		Causer("mcpproxy.Connect").
		Msgf("the daemon's MCP endpoint at %s did not answer", endpoint).
		Issue("endpoint", endpoint).
		Status(apperr.StatusServiceUnavailable).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label:   "start the daemon, then launch the MCP server again",
			Command: "aos gateway start",
			Tool:    "gateway_start",
		})
}
