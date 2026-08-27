package mcpproxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/mcpproxy"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

type storeInput struct {
	Title string `json:"title" jsonschema:"What to remember." validate:"required"`
	command.Reasoning
}

type storeOutput struct {
	ID string `json:"id"`
}

// daemon stands in for aosd: the real mcpserver handler over a real HTTP
// round trip, behind a middleware that records the credentials it was sent.
type daemon struct {
	srv    *httptest.Server
	mu     sync.Mutex
	auth   []string
	ws     []string
	agents []string
	calls  int
}

func newDaemon(t *testing.T) *daemon {
	t.Helper()
	d := &daemon{}

	reg := command.NewRegistry()
	if err := command.Register(reg, command.Command[storeInput, storeOutput]{
		Group: "memories", Name: "store",
		Summary:     "Record a durable memory.",
		Registry:    true,
		Annotations: command.Annotations{Title: "Store a memory", IdempotentHint: true},
		Handler: func(_ context.Context, in storeInput) (storeOutput, error) {
			d.mu.Lock()
			d.calls++
			d.mu.Unlock()
			return storeOutput{ID: "m-" + in.Title}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "memories", Summary: "Remember and recall."})

	handler := mcpserver.NewHTTPHandler(mcpserver.Config{
		Registry:     reg,
		Shape:        mcpserver.ShapeFlat,
		Instructions: "every call needs _reasoning",
	})
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		d.auth = append(d.auth, r.Header.Get("Authorization"))
		d.ws = append(d.ws, r.Header.Get("X-Workspace-Id"))
		d.agents = append(d.agents, r.Header.Get("X-Agent-Id"))
		d.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(d.srv.Close)
	return d
}

// connectThroughProxy stands up the proxy against the fake daemon and
// connects an MCP client to the proxy's own server over an in-memory
// transport — the stdio side, without the stdio.
func connectThroughProxy(t *testing.T, d *daemon, token string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	proxy, err := mcpproxy.Connect(ctx, mcpproxy.Options{
		Endpoint:  d.srv.URL,
		Token:     token,
		Workspace: "ws-1",
		Agent:     "atlas",
		Name:      "aos",
		Version:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })

	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := proxy.Server().Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "editor", Version: "1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestTheProxyMirrorsTheDaemonsToolsAndInstructions(t *testing.T) {
	d := newDaemon(t)
	session := connectThroughProxy(t, d, "secret")

	if got := session.InitializeResult().Instructions; got != "every call needs _reasoning" {
		t.Fatalf("instructions = %q", got)
	}

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "memories_store" {
		t.Fatalf("tools = %+v", tools.Tools)
	}
	tool := tools.Tools[0]
	if tool.Annotations == nil || tool.Annotations.Title != "Store a memory" || !tool.Annotations.IdempotentHint {
		t.Fatalf("annotations were not carried over: %+v", tool.Annotations)
	}
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("input schema was not carried over: %#v", tool.InputSchema)
	}
	props, _ := schema["properties"].(map[string]any)
	if _, ok := props["title"]; !ok {
		t.Fatalf("input schema lost its properties: %#v", schema)
	}
}

func TestACallThroughTheProxyReachesTheDaemonWithItsCredentials(t *testing.T) {
	d := newDaemon(t)
	session := connectThroughProxy(t, d, "secret")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "memories_store",
		Arguments: map[string]any{"title": "x", "_reasoning": "proving the forward"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("the daemon reported an error: %+v", res.Content)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "m-x") {
		t.Fatalf("result = %q, want the daemon's own answer", text)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.calls != 1 {
		t.Fatalf("the daemon's handler ran %d times, want 1", d.calls)
	}
	for i := range d.auth {
		if d.auth[i] != "Bearer secret" || d.ws[i] != "ws-1" {
			t.Fatalf("request %d carried auth=%q workspace=%q", i, d.auth[i], d.ws[i])
		}
		// The agent identity travels too. Without it every call through the
		// proxy is anonymous, `agents_me` has nobody to answer with, and a
		// memory — which belongs to an agent — cannot be stored at all.
		if d.agents[i] != "atlas" {
			t.Fatalf("request %d carried agent=%q, want atlas", i, d.agents[i])
		}
	}
}

// The daemon's own refusal — a validation error, say — must reach the client
// as the tool's error result, not as a proxy failure that hides the message.
func TestADaemonErrorResultIsPassedThroughUnchanged(t *testing.T) {
	d := newDaemon(t)
	session := connectThroughProxy(t, d, "secret")

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "memories_store",
		Arguments: map[string]any{"title": "no reasoning given"},
	})
	if err != nil {
		t.Fatalf("a refused call became a protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("the daemon's refusal was not marked as an error: %+v", res)
	}
}

func TestAWrongTokenIsReportedAsAnUnreachableDaemon(t *testing.T) {
	d := newDaemon(t)
	_, err := mcpproxy.Connect(context.Background(), mcpproxy.Options{Endpoint: d.srv.URL, Token: "wrong"})
	if err == nil {
		t.Fatal("connected with a wrong token")
	}
	if !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("err = %v", err)
	}
}
