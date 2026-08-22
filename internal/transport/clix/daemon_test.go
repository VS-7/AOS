package clix_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
	clix.AttachDaemon(context.Background(), root, client, cfg)
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
	clix.AttachDaemon(context.Background(), root, client, cfg)
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
	clix.AttachDaemon(context.Background(), root, client, cfg)
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

	clix.AttachDaemon(context.Background(), root, client, cfg)

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

func TestDaemonCommandSchemaFlagIsRejected(t *testing.T) {
	client := &fakeCaller{infos: []wailsvc.CommandInfo{
		{Key: "gateway.start", Group: "gateway", Name: "start", Summary: "Start."},
	}}
	_, _, err := runWithDaemon(t, client, "gateway", "start", "--schema")
	if err == nil {
		t.Fatal("expected --schema to be rejected for a daemon-served command")
	}
	if client.lastKey != "" {
		t.Error("--schema must not reach the daemon")
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
