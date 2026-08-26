package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// twoWorkspaces builds an installation serving `first`, and registers a second
// workspace at its own directory. It returns the application and both paths.
func twoWorkspaces(t *testing.T) (*app.App, string, string) {
	t.Helper()
	home := t.TempDir()
	first := t.TempDir()
	second := t.TempDir()

	a, err := app.New(app.Options{
		Env:           env.New(env.Map(map[string]string{env.KeyHome: home})),
		WorkspaceRoot: first,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	ctx := context.Background()
	if _, err := a.Workspaces.Introspect(ctx, workspace.IntrospectInput{Path: first}); err != nil {
		t.Fatalf("registering the first workspace: %v", err)
	}
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{
		Name: "Second", Path: second,
	}); err != nil {
		t.Fatalf("registering the second workspace: %v", err)
	}
	return a, first, second
}

// inWorkspace is what a request carries once the transport has read the
// x-workspace-id header — see httpapi's ambientIdentity.
func inWorkspace(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{WorkspaceID: id})
}

func invokeIn(t *testing.T, a *app.App, ctx context.Context, key, payload string) json.RawMessage {
	t.Helper()
	d, _, ok := a.Registry.Lookup(key)
	if !ok {
		t.Fatalf("%s is not published", key)
	}
	out, err := d.Invoke(ctx, command.SurfaceHTTP, json.RawMessage(payload))
	if err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// TestACommandWritesIntoTheWorkspaceItNames is the defect, reproduced.
//
// The daemon bound its repositories to the directory it started in and kept
// them for the life of the process, so the workspace a caller named changed
// nothing about where data went. Creating an agent while addressing the second
// workspace wrote it into the first — which is what "I cannot create more than
// one workspace" looked like from the interface: a switcher that switched
// nothing, and a second workspace showing the first one's contents.
func TestACommandWritesIntoTheWorkspaceItNames(t *testing.T) {
	a, first, second := twoWorkspaces(t)

	invokeIn(t, a, inWorkspace("second"), "agents_create", `{
		"_reasoning": "a test is checking which directory this lands in",
		"id": "only-in-second",
		"name": "OnlyInSecond",
		"role": "tester",
		"description": "an agent that must exist in the second workspace and nowhere else"
	}`)

	if _, err := os.Stat(filepath.Join(second, ".aos", "agents", "only-in-second")); err != nil {
		t.Fatalf("the agent is not in the workspace it was created in: %v", err)
	}
	if _, err := os.Stat(filepath.Join(first, ".aos", "agents", "only-in-second")); err == nil {
		t.Fatal("the agent was written into the workspace the process started in")
	}
}

// TestAListOnlySeesItsOwnWorkspace: the read side of the same thing. Every
// screen in the interface is a list, and a switcher that leaves them showing
// another workspace's records is the whole symptom.
func TestAListOnlySeesItsOwnWorkspace(t *testing.T) {
	a, _, _ := twoWorkspaces(t)

	invokeIn(t, a, inWorkspace("second"), "agents_create", `{
		"_reasoning": "a test is checking which directory this lands in",
		"id": "only-in-second", "name": "OnlyInSecond", "role": "tester",
		"description": "an agent that must exist in the second workspace and nowhere else"
	}`)

	list := func(id string) string {
		return string(invokeIn(t, a, inWorkspace(id), "agents_list",
			`{"_reasoning":"a test is checking what this workspace holds"}`))
	}

	if got := list("second"); !strings.Contains(got, "only-in-second") {
		t.Errorf("the second workspace does not list its own agent: %s", got)
	}
	if got := list("first"); strings.Contains(got, "only-in-second") {
		t.Errorf("the first workspace lists the second's agent: %s", got)
	}
}

// TestTheSameWorkspaceIsServedByOneSetOfServices. Two calls to one workspace
// have to share a lock and a record cache, or two writers to the same file
// would serialise against nothing.
func TestTheSameWorkspaceIsServedByOneSetOfServices(t *testing.T) {
	a, _, _ := twoWorkspaces(t)

	for i := 0; i < 3; i++ {
		invokeIn(t, a, inWorkspace("second"), "agents_list",
			`{"_reasoning":"a test is warming the workspace"}`)
	}
	// Nothing to assert beyond not having leaked: Close reports a failure to
	// release anything opened per workspace, and the t.Cleanup in
	// twoWorkspaces runs it.
}

// TestAWorkspaceThisInstallationDoesNotHaveFallsBackToThePrimary.
//
// The header is an inference about what the caller probably means — a browser
// sends the cookie it was left with, which can name a workspace somebody has
// since deleted. Refusing every command in that state would brick the window
// with no way back; commands that genuinely need a workspace record still
// refuse in the domain, where the error says so.
func TestAWorkspaceThisInstallationDoesNotHaveFallsBackToThePrimary(t *testing.T) {
	a, _, _ := twoWorkspaces(t)

	out := invokeIn(t, a, inWorkspace("never-registered"), "agents_list",
		`{"_reasoning":"a test is checking a stale workspace cookie"}`)
	if len(out) == 0 {
		t.Fatal("a stale workspace id made a workspace-independent command fail")
	}
}
