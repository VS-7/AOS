package clix_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/transport/clix"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// daemonSurface is the manifest of a daemon that publishes more than the
// handful of commands this binary links in — which is every real daemon.
func daemonSurface() []wailsvc.CommandInfo {
	return []wailsvc.CommandInfo{
		{Key: "memories_store", Group: "memories", Name: "store", Summary: "Record a durable memory."},
		{Key: "memories_recall", Group: "memories", Name: "recall", Summary: "Scan the memory graph.", ReadOnly: true},
		{Key: "tasks_list", Group: "tasks", Name: "list", Summary: "List tasks.", ReadOnly: true},
	}
}

// rootWithDaemon is main.go's own wiring: a root over the local registry, a
// daemon to ask for everything else, and the tree attached from its manifest.
func rootWithDaemon(t *testing.T, client wailsvc.Caller) *cobra.Command {
	t.Helper()
	root, _ := rootAndOutput(t, client)
	return root
}

func rootWithCaller(t *testing.T, client wailsvc.Caller) *cobra.Command {
	t.Helper()
	return rootWithDaemon(t, client)
}

func rootAndOutput(t *testing.T, client wailsvc.Caller) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	cfg := clix.Config{
		Registry: command.NewRegistry(),
		Daemon:   client,
		Out:      out,
		Err:      &bytes.Buffer{},
		IsTTY:    func() bool { return false },
	}
	root := clix.NewRoot(cfg)
	if err := clix.AttachDaemon(context.Background(), root, client, cfg); err != nil {
		t.Fatal(err)
	}
	return root, out
}

// run executes args against a root wired to client, and returns stdout.
func runSurface(t *testing.T, client wailsvc.Caller, args ...string) string {
	t.Helper()
	root, out := rootAndOutput(t, client)
	root.SetArgs(args)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return out.String()
}

// manifestCaller is a daemon that can also describe its whole surface.
type manifestCaller struct {
	*fakeCaller
	manifest command.Manifest
}

func (m *manifestCaller) Manifest(context.Context) (command.Manifest, error) {
	return m.manifest, nil
}

func describing(infos []wailsvc.CommandInfo) *manifestCaller {
	groups := map[string]int{}
	manifest := command.Manifest{Version: "test"}
	for _, info := range infos {
		idx, seen := groups[info.Group]
		if !seen {
			manifest.Groups = append(manifest.Groups, command.ManifestGroup{
				Name: info.Group, Summary: info.Group + " commands",
			})
			idx = len(manifest.Groups) - 1
			groups[info.Group] = idx
		}
		manifest.Groups[idx].Commands = append(manifest.Groups[idx].Commands,
			command.ManifestCommand{
				Key: info.Key, Group: info.Group, Name: info.Name,
				Summary: info.Summary, Doc: info.Summary,
			})
	}
	return &manifestCaller{fakeCaller: &fakeCaller{infos: infos}, manifest: manifest}
}

// TestSelfToolsListsTheDaemonSurface is defect #1: `self tools` is documented
// as "the tool surface exactly as it is published", and it listed only the
// four commands this binary happens to link in — under 5% of the real
// surface — because it read the local registry and the rest of the tree
// arrives from the daemon after that registry is built.
func TestSelfToolsListsTheDaemonSurface(t *testing.T) {
	out := runSurface(t, describing(daemonSurface()), "self", "tools")
	for _, key := range []string{"memories_store", "memories_recall", "tasks_list"} {
		if !strings.Contains(out, key) {
			t.Errorf("%s is missing from the published tool list:\n%s", key, out)
		}
	}
}

// TestSelfLLMsCoversTheDaemonSurface: the manifest a model reads to learn the
// surface in one request has the same defect and the same fix.
func TestSelfLLMsCoversTheDaemonSurface(t *testing.T) {
	out := runSurface(t, describing(daemonSurface()), "self", "llms")
	for _, key := range []string{"memories_store", "tasks_list"} {
		if !strings.Contains(out, key) {
			t.Errorf("%s is missing from the manifest:\n%s", key, out)
		}
	}
}

// TestSchemaWorksForADaemonCommand is defect #2 on the terminal: --schema
// refused outright for everything the daemon serves, which is everything but
// `gateway`. The daemon answers the question itself now, so the flag asks it.
func TestSchemaWorksForADaemonCommand(t *testing.T) {
	client := describing(daemonSurface())
	client.invoke = func(_ context.Context, key string, input json.RawMessage) (json.RawMessage, error) {
		var payload map[string]any
		if err := json.Unmarshal(input, &payload); err != nil {
			return nil, err
		}
		if payload["schema"] != true {
			t.Errorf("--schema must ask the daemon for the contract, sent %s", input)
		}
		return json.Marshal(command.Envelope{Data: command.Detail{
			Tool: key, Description: "Record a durable memory.",
		}})
	}
	out := runSurface(t, client, "memories", "store", "--schema")
	if !strings.Contains(out, "memories_store") {
		t.Errorf("output = %s", out)
	}
}

// TestAgentFlagIsAvailableOnDaemonCommands is defect #4: `agents me` reads an
// identity but nothing attaches one, so every write refused with
// MEMORY_AGENT_REQUIRED and the only way through was curl with the header set
// by hand.
func TestAgentFlagIsAvailableOnDaemonCommands(t *testing.T) {
	root := rootWithDaemon(t, describing(daemonSurface()))
	cmd, _, err := root.Find([]string{"memories", "store"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range clix.TransportFlags() {
		if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
			t.Errorf("--%s is not offered on a command served by the daemon", name)
		}
	}
}

// TestAgentFlagReachesTheClient: declaring the flag is not the fix; carrying
// it to the call is.
func TestAgentFlagReachesTheClient(t *testing.T) {
	identified := &recordingIdentity{Caller: describing(daemonSurface())}
	root := rootWithCaller(t, identified)
	root.SetArgs([]string{"memories", "store", "--agent", "atlas", "--set", "title=x"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if identified.agent != "atlas" {
		t.Errorf("agent = %q, want the one --agent named", identified.agent)
	}
}

// recordingIdentity is a caller that also accepts an identity, the way
// *daemonclient.Client does.
type recordingIdentity struct {
	wailsvc.Caller
	agent string
}

func (r *recordingIdentity) SetAgent(id string) { r.agent = id }
