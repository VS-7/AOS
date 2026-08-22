package mcpserver_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/transport/mcpserver"
)

// connectHTTP wires a real client to a real server over an actual HTTP round
// trip — httptest.NewServer(mcpserver.NewHTTPHandler(cfg)) — so this proves
// the Streamable HTTP transport itself works, not just that New's own
// registration is correct (already covered, over the in-memory transport, by
// connect in mcpserver_test.go).
func connectHTTP(t *testing.T, cfg mcpserver.Config) (*mcp.ClientSession, func()) {
	t.Helper()
	srv := httptest.NewServer(mcpserver.NewHTTPHandler(cfg))

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	return session, func() {
		_ = session.Close()
		srv.Close()
	}
}

// TestHTTPHandlerPublishesTheSameToolsAsStdio is the proof this handler
// exists for: the same tool surface a stdio client sees over
// mcp.NewInMemoryTransports (mcpserver_test.go's own connect) is reachable
// over a real HTTP round trip too.
func TestHTTPHandlerPublishesTheSameToolsAsStdio(t *testing.T) {
	reg, _ := newRegistry(t)
	session, closeAll := connectHTTP(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})
	defer closeAll()

	var names []string
	for _, tool := range listTools(t, session) {
		names = append(names, tool.Name)
	}
	want := []string{"gateway_restart", "memories_recall", "memories_store"}
	if len(names) != len(want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tools = %v, want %v", names, want)
		}
	}
}

// TestHTTPHandlerActuallyCallsTheCommand proves the round trip end to end,
// not just discovery: a tool call over HTTP reaches the same handler a stdio
// or in-process call would.
func TestHTTPHandlerActuallyCallsTheCommand(t *testing.T) {
	reg, counter := newRegistry(t)
	session, closeAll := connectHTTP(t, mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})
	defer closeAll()

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "memories_store",
		Arguments: map[string]any{"title": "over HTTP", "_reasoning": "proving the HTTP transport dispatches"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool reported an error: %s", text(t, res))
	}
	if counter.store != 1 {
		t.Fatalf("store was called %d times, want 1", counter.store)
	}
}

// TestHTTPHandlerServesTheSameServerAcrossRequests confirms NewHTTPHandler's
// own doc comment: New's registration work happens once, not per session —
// two independent client connections both see the full tool list, not just
// the first.
func TestHTTPHandlerServesTheSameServerAcrossRequests(t *testing.T) {
	reg, _ := newRegistry(t)
	handler := mcpserver.NewHTTPHandler(mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	for i := 0; i < 2; i++ {
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
		if err != nil {
			t.Fatal(err)
		}
		tools := listTools(t, session)
		_ = session.Close()
		if len(tools) != 3 {
			t.Fatalf("connection %d: tools = %d, want 3", i, len(tools))
		}
	}
}
