package mcpclient_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/OWNER/aos/internal/adapters/mcpclient"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/testsuite"
	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

// runAsServer is the fork-and-exec trick the SDK's own tests use
// (mcp/cmd_test.go): the test binary, run again with this variable set, does
// not run tests at all — it becomes the MCP server the tests below spawn as a
// child process and speak to over stdio. That is what proves this adapter and
// this project's own internal/transport/mcpserver actually agree on the
// protocol, rather than each merely agreeing with the SDK in isolation.
const runAsServer = "AOS_MCPCLIENT_TEST_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(runAsServer) != "" {
		runTestServer()
		return
	}
	os.Exit(m.Run())
}

// echoInput is the one tool the test server publishes: an intentionally
// trivial command, since these tests are about the transport, not the domain.
type echoInput struct {
	Text string `json:"text" jsonschema:"Text to echo back."`

	command.Reasoning
}

type echoOutput struct {
	Text string `json:"text"`
}

func runTestServer() {
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
		log.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "test", Tool: "Test", Summary: "Adapter contract test fixture."})

	if err := mcpserver.ServeStdio(context.Background(), mcpserver.Config{
		Registry: reg,
		Shape:    mcpserver.ShapeFlat,
	}); err != nil {
		log.Fatal(err)
	}
}

// testServerToolset points an mcp-server::stdio Toolset at this same test
// binary, re-executed as the server above.
func testServerToolset(t *testing.T) toolset.Toolset {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return toolset.Toolset{
		ID:      "mcpserver-fixture",
		Type:    toolset.MCPStdio,
		Status:  toolset.StatusEnabled,
		Command: exe,
		Env:     map[string]string{runAsServer: "1"},
	}
}

// TestStdioSatisfiesTheAdapterContract is what makes this adapter mean
// anything: it runs the same behavioural suite every mcp-server::stdio
// implementation — and, later, every implementation of the other four
// Types — must pass, against a real MCP server rather than a hand-rolled
// fake.
func TestStdioSatisfiesTheAdapterContract(t *testing.T) {
	testsuite.RunAdapterContract(t, testsuite.AdapterContract{
		Name:    "mcp-server::stdio",
		New:     func(t *testing.T) toolset.Adapter { t.Helper(); return mcpclient.NewStdio() },
		Toolset: testServerToolset(t),
		Invalid: toolset.Toolset{
			ID: "unreachable", Type: toolset.MCPStdio, Status: toolset.StatusEnabled,
			Command: "aos-mcpclient-test-binary-that-does-not-exist-anywhere",
		},
		ExpectTool: "test_echo",
	})
}

// TestStdioCallsATool goes one step further than the contract, which only
// proves ListTools and a failing Call — this proves a real round trip through
// this project's own server actually returns the tool's output.
func TestStdioCallsATool(t *testing.T) {
	a := mcpclient.NewStdio()
	ctx := context.Background()
	if err := a.Connect(ctx, testServerToolset(t)); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	out, err := a.Call(ctx, "test_echo", []byte(`{"text":"hi","_reasoning":"contract test"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("Call returned an empty result")
	}
}
