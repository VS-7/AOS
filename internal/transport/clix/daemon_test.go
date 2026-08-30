package clix_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/clix"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// fakeCaller is a wailsvc.Caller a test can script: what Commands() returns,
// and what Invoke() does with the last call it saw.
type fakeCaller struct {
	infos    []wailsvc.CommandInfo
	infosErr error

	invoke func(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error)

	lastKey   string
	lastInput json.RawMessage
}

func (f *fakeCaller) Commands(context.Context) ([]wailsvc.CommandInfo, error) {
	return f.infos, f.infosErr
}

func (f *fakeCaller) Invoke(ctx context.Context, key string, input json.RawMessage) (json.RawMessage, error) {
	f.lastKey, f.lastInput = key, input
	if f.invoke != nil {
		return f.invoke(ctx, key, input)
	}
	raw, _ := json.Marshal(command.Envelope{Data: map[string]string{"key": key}})
	return raw, nil
}

// runWithDaemon builds a root over an empty registry, attaches the fake
// daemon to it, and executes args — the shape every daemon command actually
// runs through: main.go's own AttachDaemon call, just without a real process
// on the other end.
func runWithDaemon(t *testing.T, client wailsvc.Caller, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cfg := clix.Config{
		Registry: command.NewRegistry(),
		Out:      &out, Err: &errOut,
		IsTTY: func() bool { return false },
	}
	root := clix.NewRoot(cfg)
	_ = clix.AttachDaemon(context.Background(), root, client, cfg)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

func TestAttachDaemonAddsOneCommandPerManifestEntryGroupedByGroup(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start the gateway."},
		{Key: "gateway.stop", Group: "gateway", Name: "stop", Summary: "Stop the gateway."},
	}}
	out, _, err := runWithDaemon(t, client, "gateway", "start")
	if err != nil {
		t.Fatal(err)
	}
	if client.lastKey != "gateway.start" {
		t.Errorf("key = %q", client.lastKey)
	}
	if !strings.Contains(out, "gateway.start") {
		t.Errorf("output = %s", out)
	}
}

func TestAttachDaemonIsBestEffortWhenCommandsFails(t *testing.T) {
	client := &fakeCaller{infosErr: errors.New("daemon unreachable")}
	var out, errOut bytes.Buffer
	cfg := clix.Config{Registry: command.NewRegistry(), Out: &out, Err: &errOut, IsTTY: func() bool { return false }}
	root := clix.NewRoot(cfg)
	before := len(root.Commands())
	if err := clix.AttachDaemon(context.Background(), root, client, cfg); err == nil {
		t.Fatal("an unreachable daemon was reported as a success")
	}
	if len(root.Commands()) != before {
		t.Fatalf("root grew from an unreachable daemon: %d -> %d", before, len(root.Commands()))
	}
}

func TestAttachDaemonIsBestEffortWhenCommandsIsEmpty(t *testing.T) {
	client := &fakeCaller{infos: nil}
	var out, errOut bytes.Buffer
	cfg := clix.Config{Registry: command.NewRegistry(), Out: &out, Err: &errOut, IsTTY: func() bool { return false }}
	root := clix.NewRoot(cfg)
	before := len(root.Commands())
	if err := clix.AttachDaemon(context.Background(), root, client, cfg); err == nil {
		t.Fatal("an unreachable daemon was reported as a success")
	}
	if len(root.Commands()) != before {
		t.Fatalf("root grew from an empty manifest: %d -> %d", before, len(root.Commands()))
	}
}

// TestAttachDaemonSharesAnExistingGroupCommand covers the case main.go
// actually hits: `self` already exists on root (NewRoot always adds it), and
// a daemon whose manifest also used group "self" must not shadow it or
// create a second "self" command.
func TestAttachDaemonSharesAnExistingGroupCommand(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "self.remote", Group: "self", Name: "remote", Summary: "A daemon-only self command."},
	}}
	var out, errOut bytes.Buffer
	cfg := clix.Config{Registry: command.NewRegistry(), Out: &out, Err: &errOut, IsTTY: func() bool { return false }}
	root := clix.NewRoot(cfg)

	selfCount := 0
	for _, c := range root.Commands() {
		if c.Name() == "self" {
			selfCount++
		}
	}
	if selfCount != 1 {
		t.Fatalf("precondition: expected exactly one self command, got %d", selfCount)
	}

	_ = clix.AttachDaemon(context.Background(), root, client, cfg)

	selfCount = 0
	for _, c := range root.Commands() {
		if c.Name() == "self" {
			selfCount++
		}
	}
	if selfCount != 1 {
		t.Fatalf("AttachDaemon created a second self group: %d", selfCount)
	}
	if _, _, err := root.Find([]string{"self", "remote"}); err != nil {
		t.Fatalf("the daemon command was not attached under the existing group: %v", err)
	}
	// And the pre-existing builtins under self are still reachable.
	if _, _, err := root.Find([]string{"self", "tools"}); err != nil {
		t.Fatalf("attaching to an existing group must not remove what was already there: %v", err)
	}
}

func TestDaemonCommandMergesJSONSetAndReasonWithSetWinningOverJSON(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	_, _, err := runWithDaemon(t, client, "gateway", "start",
		"--json", `{"mode":"json-mode","count":1}`,
		"--set", "mode=set-mode",
		"--set", "count=3",
		"--reason", "because")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if uerr := json.Unmarshal(client.lastInput, &payload); uerr != nil {
		t.Fatalf("payload is not JSON: %s", client.lastInput)
	}
	if payload["mode"] != "set-mode" {
		t.Errorf("--set did not override --json: %v", payload)
	}
	if payload["count"] != float64(3) {
		t.Errorf("--set did not parse a typed value: %v", payload)
	}
	if payload["_reasoning"] != "because" {
		t.Errorf("--reason did not set _reasoning: %v", payload)
	}
}

func TestDaemonCommandSetSupportsDottedPaths(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	_, _, err := runWithDaemon(t, client, "gateway", "start", "--set", "opts.verbose=true")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if uerr := json.Unmarshal(client.lastInput, &payload); uerr != nil {
		t.Fatalf("payload is not JSON: %s", client.lastInput)
	}
	opts, ok := payload["opts"].(map[string]any)
	if !ok {
		t.Fatalf("opts is not an object: %v", payload)
	}
	if opts["verbose"] != true {
		t.Errorf("verbose = %v", opts["verbose"])
	}
}

func TestDaemonCommandRejectsASetWithoutEquals(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	_, _, err := runWithDaemon(t, client, "gateway", "start", "--set", "no-equals-sign")
	if err == nil {
		t.Fatal("expected an error for a --set without path=value")
	}
}

func TestDaemonCommandRejectsInvalidJSON(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	_, _, err := runWithDaemon(t, client, "gateway", "start", "--json", `not json`)
	if err == nil {
		t.Fatal("expected an error for invalid --json")
	}
}

// TestDaemonCommandSchemaFlagAsksTheDaemon replaces the assertion that used to
// stand here, which pinned the defect: --schema was refused for every command
// the daemon serves, which is every command but the four this binary links in.
// The daemon answers the question on the command's own route now, so the flag
// asks it rather than refusing.
func TestDaemonCommandSchemaFlagAsksTheDaemon(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	if _, _, err := runWithDaemon(t, client, "gateway", "start", "--schema"); err != nil {
		t.Fatalf("--schema must reach the daemon: %v", err)
	}
	if client.lastKey != "gateway.start" {
		t.Fatalf("key = %q, want the command being inspected", client.lastKey)
	}
	var payload map[string]any
	if err := json.Unmarshal(client.lastInput, &payload); err != nil {
		t.Fatal(err)
	}
	if payload[command.SchemaField] != true {
		t.Errorf("payload = %s, want %s:true", client.lastInput, command.SchemaField)
	}
}

func TestDaemonCommandSurfacesAnInvokeError(t *testing.T) {
	client := &fakeCaller{
		infos: []wailsvc.CommandInfo{{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."}},
		invoke: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return nil, errors.New("connection reset")
		},
	}
	// A plain transport error (not an apperr.Error) is not rendered by
	// writeError — see its own doc comment — it is only returned, so cobra
	// (SilenceErrors) fails the command without printing it a second time.
	_, _, err := runWithDaemon(t, client, "gateway", "start", "--format", "json")
	if err == nil {
		t.Fatal("expected the transport error to fail the command")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("err = %v", err)
	}
}

// TestDaemonCommandSurfacesAStructuredError covers the path a daemon actually
// uses to report a domain failure: HTTP 200 with an apperr.Error body, not a
// transport error. buildDaemonCommand must notice the body is an error rather
// than treating it as a successful envelope.
func TestDaemonCommandSurfacesAStructuredError(t *testing.T) {
	appErr := apperr.New("GATEWAY_LOCKED").Msgf("the gateway is locked")
	client := &fakeCaller{
		infos: []wailsvc.CommandInfo{{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."}},
		invoke: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(map[string]any{"error": appErr})
		},
	}
	_, stderr, err := runWithDaemon(t, client, "gateway", "start", "--format", "json")
	if err == nil {
		t.Fatal("expected the structured error to fail the command")
	}
	var payload map[string]any
	if uerr := json.Unmarshal([]byte(stderr), &payload); uerr != nil {
		t.Fatalf("stderr is not the structured error: %s", stderr)
	}
	if payload["code"] != "AOS_GATEWAY_LOCKED" {
		t.Errorf("code = %v", payload["code"])
	}
}

func TestDaemonCommandTokenCountReportsSizeInsteadOfResult(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	out, _, err := runWithDaemon(t, client, "gateway", "start", "--format", "json", "--token-count")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "{") {
		t.Errorf("--token-count should print a number, not the result: %s", out)
	}
}

// The daemon publishes `gateway` too, and this binary builds it locally
// because supervising a process cannot be delegated to the process being
// supervised. Attaching both put start, stop, status and restart in the group
// twice — and the daemon's copy of `start` asked the daemon to start itself.
func TestALocalGroupIsNotDuplicatedByTheDaemonsCopy(t *testing.T) {
	reg := command.NewRegistry()
	if err := command.Register(reg, command.Command[demoInput, demoOutput]{
		Group: "gateway", Name: "start", Summary: "Start the daemon.", Local: true,
		Handler: func(context.Context, demoInput) (demoOutput, error) { return demoOutput{}, nil },
	}); err != nil {
		t.Fatal(err)
	}
	reg.DescribeGroup(command.GroupDoc{Name: "gateway", Summary: "The daemon."})

	root := clix.NewRoot(clix.Config{Registry: reg, IsTTY: func() bool { return false }})
	caller := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway_start", Group: "gateway", Name: "start", Summary: "Start the daemon."},
		{Key: "agents_list", Group: "agents", Name: "list", Summary: "List agents."},
	}}
	if err := clix.AttachDaemon(context.Background(), root, caller, clix.Config{Registry: reg}); err != nil {
		t.Fatal(err)
	}

	var gateway *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "gateway" {
			gateway = c
		}
	}
	if gateway == nil {
		t.Fatal("the gateway group is gone")
	}
	starts := 0
	for _, c := range gateway.Commands() {
		if c.Name() == "start" {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("gateway has %d start commands, want the local one only", starts)
	}
	// The daemon's own groups still arrive.
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "agents" {
			found = true
		}
	}
	if !found {
		t.Error("a group only the daemon publishes was not attached")
	}
}

// A domain command with no daemon behind it reads as "unknown command",
// which sounds like the command does not exist. It does — this binary just
// could not ask for it.
func TestAMissingTreeExplainsItself(t *testing.T) {
	cause := errors.New("the daemon at http://127.0.0.1:5326 did not answer")
	explained := clix.ExplainMissingTree(errors.New(`unknown command "agents" for "aos"`), cause)

	e, ok := apperr.As(explained)
	if !ok {
		t.Fatalf("the failure was not classified: %v", explained)
	}
	if e.Code != "AOS_CLI_SURFACE_UNAVAILABLE" {
		t.Errorf("code = %s", e.Code)
	}
	if len(e.Actions) == 0 {
		t.Error("no call to action to follow")
	}

	// Anything else passes through untouched, and so does success.
	other := errors.New("required flag(s) \"id\" not set")
	if got := clix.ExplainMissingTree(other, cause); !errors.Is(got, other) {
		t.Errorf("an unrelated failure was rewritten: %v", got)
	}
	if got := clix.ExplainMissingTree(nil, cause); got != nil {
		t.Errorf("success became a failure: %v", got)
	}
}
