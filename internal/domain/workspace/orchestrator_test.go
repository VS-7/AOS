package workspace_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/workspace"
)

// TestTheOrchestratorCanActuallyWork is the defect behind "the agent could not
// do anything".
//
// An agent with no sandbox block gets the zero value, which is read-only with
// no execution at all (sandbox.DefaultPermissions). The orchestrator — the one
// agent every workspace is created with, and the one the person actually talks
// to — was seeded without one. So the default experience of the product was an
// assistant that could read the workspace, could not write a line into it, and
// could not run a command: it planned, delegated, and then reported that the
// sandbox had refused it.
//
// Read-only is the right default for an agent somebody adds later, whose job
// nobody has declared yet. It is the wrong default for the one agent the
// system creates on the person's own machine, in their own repository, at
// their request.
func TestTheOrchestratorCanActuallyWork(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	seeded := h.seeder.seeded
	if seeded.Sandbox == nil {
		t.Fatal("the orchestrator was seeded with no sandbox, which is read-only with no execution")
	}

	granted := map[string]bool{}
	for _, p := range seeded.Sandbox.Permissions {
		granted[p] = true
	}
	for _, want := range []string{"read", "write", "delete", "execute"} {
		if !granted[want] {
			t.Errorf("the orchestrator cannot %s in its own workspace", want)
		}
	}
	_ = out
}

// TestTheOrchestratorRunsFromAnAllowlist keeps ADR-0006: execution is an
// allowlist, never a blocklist, and a shell is its own opt-in because a shell
// makes the allowlist a suggestion.
func TestTheOrchestratorRunsFromAnAllowlist(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	exec := h.seeder.seeded.Sandbox.Exec
	if exec == nil {
		t.Fatal("the orchestrator has write permission and no execution policy at all")
	}
	if exec.Policy != "allowlist" {
		t.Errorf("policy = %q, want allowlist (ADR-0006)", exec.Policy)
	}
	if len(exec.Allow) == 0 {
		t.Error("the allowlist is empty, which is deny-all by another name")
	}
	if exec.AllowShell {
		t.Error("a shell was granted by default — it makes the allowlist a suggestion")
	}

	allowed := map[string]bool{}
	for _, binary := range exec.Allow {
		allowed[binary] = true
	}
	// The tools a person watching an agent work in a repository expects it to
	// reach without being asked twice.
	for _, want := range []string{"git", "go", "node", "npm"} {
		if !allowed[want] {
			t.Errorf("%q is not on the orchestrator's allowlist", want)
		}
	}
}

// TestASpecCanStillTightenTheSandbox: the default is a default, not a policy
// imposed on whoever asked for something narrower.
func TestASpecCanStillTightenTheSandbox(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{
		Name: "Project Alpha", Path: repoRoot,
		Orchestrator: &workspace.OrchestratorSpec{
			Sandbox: &workspace.SandboxSpec{Permissions: []string{"read"}},
		},
	})

	granted := h.seeder.seeded.Sandbox.Permissions
	if len(granted) != 1 || granted[0] != "read" {
		t.Errorf("permissions = %v, want exactly what was asked for", granted)
	}
}
