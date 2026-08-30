package workspace_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// TestIntrospectAdoptsWhenTheNameIsTakenByAnotherDirectory is defect #3.
//
// Introspect matched an existing registration by path alone, and fell through
// to Create when nothing matched. Create derives the id from the name, and the
// name comes from the repository — so two directories whose repository has the
// same name made the mandatory first step of every session answer
// WORKSPACE_ALREADY_EXISTS instead of a workspace.
func TestIntrospectRegistersASecondDirectoryWithTheSameRepositoryName(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "project-alpha", Path: "/elsewhere/project-alpha"})

	out, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatalf("introspect must register the directory it is standing in: %v", err)
	}
	if out.Workspace.Path != repoRoot {
		t.Errorf("path = %q, want the directory introspected", out.Workspace.Path)
	}
	if out.Workspace.ID == "project-alpha" {
		t.Error("the existing registration for another directory was overwritten")
	}
	// And it stays idempotent: the second run finds what the first registered.
	again, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatalf("the second run failed: %v", err)
	}
	if again.Workspace.ID != out.Workspace.ID || !again.Adopted {
		t.Errorf("second run = %+v, want the first one adopted", again.Workspace)
	}
}

// TestIntrospectNeverAnswersAlreadyExists guards the property the mandatory
// session protocol depends on: this command answers with a workspace.
func TestIntrospectNeverAnswersAlreadyExists(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.create(t, workspace.CreateInput{
			Name: nameFor(i), Path: "/elsewhere/" + nameFor(i),
		})
	}
	h.git.origin = "git@github.com:someone/project-alpha.git"
	h.create(t, workspace.CreateInput{Name: "project-alpha", Path: "/elsewhere/taken"})

	if _, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{}); err != nil {
		e, _ := apperr.As(err)
		t.Fatalf("introspect refused: %v (%v)", err, e)
	}
}

func nameFor(i int) string { return []string{"one", "two", "three"}[i] }

// TestIntrospectPrefersTheCallersWorkingDirectory: the daemon serves many
// clients and its own working directory is nobody's. A terminal that says
// where it is standing gets that directory registered, not the daemon's.
func TestIntrospectPrefersTheCallersWorkingDirectory(t *testing.T) {
	h := newHarness(t)
	callerCtx := identity.With(ctx(), identity.Identity{WorkingDir: "/home/me/other-repo"})

	out, err := h.svc.Introspect(callerCtx, workspace.IntrospectInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workspace.Path != "/home/me/other-repo" {
		t.Errorf("path = %q, want the caller's directory", out.Workspace.Path)
	}
}

// TestIntrospectPathBeatsEverything: an explicit path is the caller being
// specific, and nothing ambient may override it.
func TestIntrospectPathBeatsEverything(t *testing.T) {
	h := newHarness(t)
	callerCtx := identity.With(ctx(), identity.Identity{WorkingDir: "/home/me/other-repo"})

	out, err := h.svc.Introspect(callerCtx, workspace.IntrospectInput{Path: "/home/me/explicit"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workspace.Path != "/home/me/explicit" {
		t.Errorf("path = %q, want the explicit one", out.Workspace.Path)
	}
}

// TestWorkspaceIsAddressableByID is defect #7: every other group addresses its
// own resource with `id`, and a caller building calls by convention — a
// generated form, an agent — had no way to know this one group is different.
func TestWorkspaceIsAddressableByID(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	got, err := h.svc.Get(ctx(), workspace.GetInput{ID: "project-alpha"})
	if err != nil {
		t.Fatalf("workspace_get must accept id: %v", err)
	}
	if got.ID != "project-alpha" {
		t.Errorf("id = %q", got.ID)
	}

	inventory, err := h.svc.Inventory(ctx(), workspace.InventoryInput{ID: "project-alpha"})
	if err != nil {
		t.Fatalf("workspace_inventory must accept id: %v", err)
	}
	if inventory.Workspace != "project-alpha" {
		t.Errorf("inventory = %q", inventory.Workspace)
	}

	updated, err := h.svc.Update(ctx(), workspace.UpdateInput{
		ID: "project-alpha", Set: map[string]any{"color": "#10b981"},
	})
	if err != nil {
		t.Fatalf("workspace_update must accept id: %v", err)
	}
	if updated.Color != "#10b981" {
		t.Errorf("colour = %q", updated.Color)
	}

	out, err := h.svc.Delete(ctx(), workspace.DeleteInput{ID: "project-alpha"})
	if err != nil {
		t.Fatalf("workspace_delete must accept id: %v", err)
	}
	if !out.Deleted {
		t.Error("the workspace was not unregistered")
	}
}

// TestTheWorkspaceFieldStillWorks: `workspace` is what every existing caller
// sends — the desktop's own stores.ts among them — and renaming a published
// field is the breakage ADR-0011 exists to prevent.
func TestTheWorkspaceFieldStillWorks(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	if _, err := h.svc.Get(ctx(), workspace.GetInput{Workspace: "project-alpha"}); err != nil {
		t.Fatalf("the original field name must keep working: %v", err)
	}
}
