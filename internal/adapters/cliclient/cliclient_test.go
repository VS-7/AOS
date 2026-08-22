package cliclient_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/adapters/cliclient"
	"github.com/OWNER/aos/internal/domain/testsuite"
	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/runtime/execguard"
	"github.com/OWNER/aos/internal/runtime/sandbox"
)

// newSandbox builds a real sandbox rooted in a scratch directory, execute
// permission granted and only allow on its allowlist — the minimum a cli
// toolset's Call needs to run anything at all.
func newSandbox(t *testing.T, allow ...string) *sandbox.Sandbox {
	t.Helper()
	box, err := sandbox.New(sandbox.Options{
		WorkspacePath: t.TempDir(),
		Permissions:   sandbox.Permissions{Execute: true},
		Exec: sandbox.ExecPolicy{
			Policy: sandbox.PolicyAllowlist,
			Allow:  allow,
		},
	})
	if err != nil {
		t.Fatalf("sandbox.New: %v", err)
	}
	return box
}

// decodedResult is the shape of what Call returns: sandbox.Result, marshaled
// as-is.
type decodedResult struct {
	ExitCode int `json:"exitCode"`
	Stdout   struct {
		Content string `json:"content"`
	} `json:"stdout"`
}

func decode(t *testing.T, raw json.RawMessage) decodedResult {
	t.Helper()
	var r decodedResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decoding result %s: %v", raw, err)
	}
	return r
}

func TestCLISatisfiesTheAdapterContract(t *testing.T) {
	testsuite.RunAdapterContract(t, testsuite.AdapterContract{
		Name: "cli",
		New:  func(t *testing.T) toolset.Adapter { return cliclient.New() },
		Toolset: toolset.Toolset{
			ID: "echoer", Type: toolset.CLI, Command: "echo", Args: []string{"hi"},
		},
		Invalid: toolset.Toolset{
			ID: "empty", Type: toolset.CLI, Command: "",
		},
		ExpectTool: "run",
	})
}

func TestCLIRunsTheConfiguredCommandThroughTheCallersSandbox(t *testing.T) {
	box := newSandbox(t, "echo")
	ctx := execguard.With(context.Background(), box)

	a := cliclient.New()
	ts := toolset.Toolset{ID: "echoer", Type: toolset.CLI, Command: "echo", Args: []string{"hello"}}
	if err := a.Connect(ctx, ts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = a.Close() }()

	input, _ := json.Marshal(map[string]any{"args": []string{"world"}})
	raw, err := a.Call(ctx, "run", input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := decode(t, raw)
	if result.ExitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout.Content); got != "hello world" {
		t.Fatalf("stdout = %q, want %q", got, "hello world")
	}
}

func TestCLIRefusesWithoutASandboxOnContext(t *testing.T) {
	a := cliclient.New()
	ctx := context.Background()
	if err := a.Connect(ctx, toolset.Toolset{ID: "echoer", Type: toolset.CLI, Command: "echo"}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := a.Call(ctx, "run", nil); err == nil {
		t.Fatal("Call with no sandbox on ctx reported success — a cli toolset must refuse, not run unguarded")
	}
}

func TestCLIRefusesABinaryOutsideTheCallersAllowlist(t *testing.T) {
	box := newSandbox(t, "true") // "echo" is deliberately not on this agent's allowlist
	ctx := execguard.With(context.Background(), box)

	a := cliclient.New()
	if err := a.Connect(ctx, toolset.Toolset{ID: "echoer", Type: toolset.CLI, Command: "echo"}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := a.Call(ctx, "run", nil); err == nil {
		t.Fatal("Call ran a binary the calling agent's own sandbox never allowlisted")
	}
}

func TestCLIStdinReachesTheProcess(t *testing.T) {
	box := newSandbox(t, "cat")
	ctx := execguard.With(context.Background(), box)

	a := cliclient.New()
	if err := a.Connect(ctx, toolset.Toolset{ID: "catter", Type: toolset.CLI, Command: "cat"}); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	input, _ := json.Marshal(map[string]any{"stdin": "from the toolset\n"})
	raw, err := a.Call(ctx, "run", input)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := decode(t, raw)
	if result.Stdout.Content != "from the toolset\n" {
		t.Fatalf("stdout = %q, want %q", result.Stdout.Content, "from the toolset\n")
	}
}

func TestCLIEnvReachesTheProcess(t *testing.T) {
	box := newSandbox(t, "env")
	ctx := execguard.With(context.Background(), box)

	a := cliclient.New()
	ts := toolset.Toolset{
		ID: "enver", Type: toolset.CLI, Command: "env",
		Env: map[string]string{"TOOLSET_MARKER": "present"},
	}
	if err := a.Connect(ctx, ts); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	raw, err := a.Call(ctx, "run", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	result := decode(t, raw)
	if !strings.Contains(result.Stdout.Content, "TOOLSET_MARKER=present") {
		t.Fatalf("stdout = %q, want it to contain TOOLSET_MARKER=present", result.Stdout.Content)
	}
}

func TestCLIListToolsPublishesExactlyOneRunTool(t *testing.T) {
	a := cliclient.New()
	ts := toolset.Toolset{ID: "echoer", Type: toolset.CLI, Command: "echo", Description: "echoes things"}
	if err := a.Connect(context.Background(), ts); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	specs, err := a.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "run" {
		t.Fatalf("ListTools = %+v, want exactly one tool named %q", specs, "run")
	}
	if specs[0].Description != "echoes things" {
		t.Fatalf("Description = %q, want the toolset's own description", specs[0].Description)
	}
}

func TestCLIListToolsFallsBackToADescriptionOfTheCommand(t *testing.T) {
	a := cliclient.New()
	if err := a.Connect(context.Background(), toolset.Toolset{ID: "echoer", Type: toolset.CLI, Command: "echo"}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	specs, err := a.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if !strings.Contains(specs[0].Description, "echo") {
		t.Fatalf("Description = %q, want it to name the command", specs[0].Description)
	}
}

func TestCLIConnectRejectsAnEmptyCommand(t *testing.T) {
	a := cliclient.New()
	if err := a.Connect(context.Background(), toolset.Toolset{ID: "x", Type: toolset.CLI}); err == nil {
		t.Fatal("Connect with no command reported success")
	}
}

func TestCLICallingBeforeConnectFails(t *testing.T) {
	a := cliclient.New()
	if _, err := a.Call(context.Background(), "run", nil); err == nil {
		t.Fatal("Call before Connect reported success")
	}
	if _, err := a.ListTools(context.Background()); err == nil {
		t.Fatal("ListTools before Connect reported success")
	}
}
