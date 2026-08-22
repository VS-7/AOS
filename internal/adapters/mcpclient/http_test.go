package mcpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OWNER/aos/internal/adapters/mcpclient"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/testsuite"
	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

// newTestHTTPServer builds the identical "test" tool group stdio_test.go's
// runTestServer registers, published over Streamable HTTP instead of stdio —
// this project's own internal/transport/mcpserver on both sides, over both
// transports, is what proves this adapter and that package actually agree.
func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	reg := command.NewRegistry()
	err := command.Register(reg, command.Command[echoInput, echoOutput]{
		Group: "test", Name: "echo",
		Summary:  "Echo back text.",
		Doc:      "Echoes text back. Exists only for the mcpclient adapter contract test.",
		Registry: true,
		Handler: func(_ context.Context, in echoInput) (echoOutput, error) {
			return echoOutput{Text: in.Text}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "test", Tool: "Test", Summary: "Adapter contract test fixture."})

	srv := mcpserver.New(mcpserver.Config{Registry: reg, Shape: mcpserver.ShapeFlat})
	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return srv }, nil)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func testHTTPToolset(url string) toolset.Toolset {
	return toolset.Toolset{
		ID: "mcphttp-fixture", Type: toolset.MCPHTTP, Status: toolset.StatusEnabled,
		BaseURL: url,
	}
}

// TestHTTPSatisfiesTheAdapterContract runs the identical behavioural suite
// stdio's own contract test does — see that test's own doc comment — against
// a real Streamable HTTP server this time.
func TestHTTPSatisfiesTheAdapterContract(t *testing.T) {
	ts := newTestHTTPServer(t)
	testsuite.RunAdapterContract(t, testsuite.AdapterContract{
		Name:    "mcp-server::http",
		New:     func(t *testing.T) toolset.Adapter { t.Helper(); return mcpclient.NewHTTP() },
		Toolset: testHTTPToolset(ts.URL),
		Invalid: toolset.Toolset{
			ID: "unreachable", Type: toolset.MCPHTTP, Status: toolset.StatusEnabled,
			BaseURL: "http://127.0.0.1:1/does-not-exist",
		},
		ExpectTool: "test_echo",
	})
}

// TestHTTPCallsATool goes one step further than the contract — see
// TestStdioCallsATool's own doc comment for why.
func TestHTTPCallsATool(t *testing.T) {
	ts := newTestHTTPServer(t)
	a := mcpclient.NewHTTP()
	ctx := context.Background()
	if err := a.Connect(ctx, testHTTPToolset(ts.URL)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	out, err := a.Call(ctx, "test_echo", []byte(`{"text":"hello over http","_reasoning":"contract test"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("Call returned no output")
	}
}

// TestHTTPHeadersReachTheServer proves Toolset.Headers actually leaves the
// process on the outgoing request — the whole reason headerRoundTripper
// exists rather than relying on the SDK's own client to carry them. It does
// not need Connect to succeed: capturing the header on the first request this
// adapter makes is the whole assertion, regardless of what a server that
// speaks no MCP does with it afterward.
func TestHTTPHeadersReachTheServer(t *testing.T) {
	var gotAuth string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer fake.Close()

	ts := toolset.Toolset{
		ID: "mcphttp-headers", Type: toolset.MCPHTTP, Status: toolset.StatusEnabled,
		BaseURL: fake.URL, Headers: map[string]string{"Authorization": "Bearer test-token"},
	}
	a := mcpclient.NewHTTP()
	// Connect is expected to fail — fake speaks no MCP — the header capture
	// above is what this test is actually about.
	_ = a.Connect(context.Background(), ts)
	_ = a.Close()

	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization seen by the server = %q, want %q", gotAuth, "Bearer test-token")
	}
}

// TestHTTPConnectNeverReachesAHostOutsideTheAttachedAllowlist proves the
// guard toolset.Service.Call attaches to ctx (see toolset.WithAllowedHosts)
// is actually wired into this adapter's client — headerRoundTripper's next
// — not only unit-tested in isolation. Like TestHTTPHeadersReachTheServer,
// it does not need Connect to succeed against a server that speaks no MCP;
// the request never arriving at all is the assertion.
func TestHTTPConnectNeverReachesAHostOutsideTheAttachedAllowlist(t *testing.T) {
	reached := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer fake.Close()

	a := mcpclient.NewHTTP()
	restricted := toolset.WithAllowedHosts(context.Background(), []string{"some-other-host.example.com"})
	_ = a.Connect(restricted, testHTTPToolset(fake.URL))
	_ = a.Close()

	if reached {
		t.Fatal("a request reached a host outside the attached allowlist")
	}
}

func TestHTTPConnectReachesAHostInTheAttachedAllowlist(t *testing.T) {
	reached := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer fake.Close()

	a := mcpclient.NewHTTP()
	restricted := toolset.WithAllowedHosts(context.Background(), []string{"127.0.0.1"})
	_ = a.Connect(restricted, testHTTPToolset(fake.URL))
	_ = a.Close()

	if !reached {
		t.Fatal("a request to a host the allowlist actually declares never arrived")
	}
}
