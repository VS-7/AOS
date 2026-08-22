package app_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/domain/view"
)

// TestTheDeliveryOfPhaseEight is the phase in one test: installing a skill
// installs a team. crm brings its own agent, a "contacts" collection and a
// table view over it — install, create a record in the same session with no
// restart, render the view and see the record, then uninstall and confirm
// both the collection and the view are gone.
//
// Everything below the skill installer is the real thing: the collection
// domain validates and writes the record, the view domain resolves the
// source and reads it back, and the runtime registry is what makes "the
// same session" true — nothing here waits for a watcher or a restart.
func TestTheDeliveryOfPhaseEight(t *testing.T) {
	a := newApp(t)
	ctx := parityCtx()

	if !a.Collections.Exists("contacts") {
		// Not installed yet — the assertion belongs after Install, kept here
		// only to document the baseline a fresh workspace starts from.
		t.Log("contacts does not exist before install, as expected")
	}

	installed, err := a.Skills.Install(ctx, skill.InstallInput{
		Source:      "testdata/crm-skill",
		AcceptedAll: func(skill.Permissions) bool { return true },
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	if !a.Collections.Exists("contacts") {
		t.Fatal("installing crm did not register its contacts collection")
	}
	if _, err := a.Views.Get(ctx, view.GetInput{ID: "contacts-table", Skill: "crm"}); err != nil {
		t.Fatalf("the skill's view is not reachable right after install: %v", err)
	}

	// Same session, no restart: create a record against the collection the
	// skill just brought.
	rec, err := a.Collections.Records().Create(ctx, "contacts", map[string]any{
		"name": "Ada Lovelace", "stage": "won",
	})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	if rec.Collection != "contacts" {
		t.Fatalf("collection = %q, want contacts", rec.Collection)
	}

	// Render the view and see the record — what the frontend's renderer
	// would consume, with no build and no deploy.
	rendered, err := a.Views.Render(ctx, view.RenderInput{ID: "contacts-table", Skill: "crm"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(rendered.Records) != 1 {
		t.Fatalf("rendered %d records, want 1", len(rendered.Records))
	}
	if rendered.Records[0].Data["name"] != "Ada Lovelace" {
		t.Fatalf("rendered record = %+v, want Ada Lovelace", rendered.Records[0].Data)
	}

	// Uninstall. Both the collection and the view go with the skill.
	if err := a.Skills.Uninstall(ctx, skill.UninstallInput{ID: installed.ID}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if a.Collections.Exists("contacts") {
		t.Fatal("the contacts collection outlived the skill that brought it")
	}
	if _, err := a.Views.Get(ctx, view.GetInput{ID: "contacts-table", Skill: "crm"}); err == nil {
		t.Fatal("the contacts-table view outlived the skill that brought it")
	}
}

// TestASkillInstalledToolsetLocksCommandAndBaseURL proves the real
// composition root, not just the domain's own fixtures: a toolset the
// lockbox skill installs cannot have its Command or BaseURL walked away from
// what VerifyManifest already checked at install (internal/domain/skill's
// own check) — toolset.Service.UpdateConfig's own lock refuses either field
// once a toolset's Skill is non-empty, see that method's doc comment. Fields
// VerifyManifest never checked stay editable regardless.
func TestASkillInstalledToolsetLocksCommandAndBaseURL(t *testing.T) {
	a := newApp(t)
	ctx := parityCtx()

	if _, err := a.Skills.Install(ctx, skill.InstallInput{
		Source:      "testdata/lockbox-skill",
		AcceptedAll: func(skill.Permissions) bool { return true },
	}); err != nil {
		t.Fatalf("install: %v", err)
	}

	got, err := a.Toolsets.Get(ctx, toolset.GetInput{ID: "vault"})
	if err != nil {
		t.Fatalf("the skill's toolset is not reachable right after install: %v", err)
	}
	if got.Command != "true" {
		t.Fatalf("Command = %q, want true", got.Command)
	}

	newCommand := "rm"
	_, err = a.Toolsets.UpdateConfig(ctx, toolset.UpdateConfigInput{ID: "vault", Command: &newCommand})
	if err == nil {
		t.Fatal("changing Command on a skill-installed toolset must be refused")
	}

	newURL := "https://attacker.example.com"
	_, err = a.Toolsets.UpdateConfig(ctx, toolset.UpdateConfigInput{ID: "vault", BaseURL: &newURL})
	if err == nil {
		t.Fatal("changing BaseURL on a skill-installed toolset must be refused")
	}

	// What VerifyManifest never checked stays editable.
	desc := "reconfigured"
	updated, err := a.Toolsets.UpdateConfig(ctx, toolset.UpdateConfigInput{ID: "vault", Description: &desc})
	if err != nil {
		t.Fatalf("editing Description must still succeed: %v", err)
	}
	if updated.Description != desc {
		t.Fatalf("Description = %q, want %q", updated.Description, desc)
	}

	// The lock never touched Command — the rejected attempts above must not
	// have partially applied.
	again, err := a.Toolsets.Get(ctx, toolset.GetInput{ID: "vault"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Command != "true" {
		t.Fatalf("Command = %q, want true — a rejected UpdateConfig must not partially apply", again.Command)
	}
}

// TestASkillsHookActuallyInterceptsThroughTheRealCompositionRoot proves the
// other half of dynamic hook registration end to end: not internal/adapters/
// skillhooks' own unit tests, which cover Register/Deregister in isolation
// over a fake bus, but a.Skills.Install wiring a real skill's hook onto
// a.Hooks — the very *event.Service an agent's turn emits against — with
// nothing else touched by hand.
func TestASkillsHookActuallyInterceptsThroughTheRealCompositionRoot(t *testing.T) {
	a := newApp(t)
	ctx := parityCtx()

	// No skill installed yet: a PreToolUse goes through with no opinion.
	before, err := a.Hooks.Emit(ctx, event.Event{Type: event.PreToolUse, Tool: "toolsets_call"})
	if err != nil {
		t.Fatal(err)
	}
	if before.Blocked() {
		t.Fatal("something blocked PreToolUse before any hook was installed")
	}

	installed, err := a.Skills.Install(ctx, skill.InstallInput{
		Source:      "testdata/hookish-skill",
		AcceptedAll: func(skill.Permissions) bool { return true },
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if len(installed.Metadata.Hooks) != 1 || installed.Metadata.Hooks[0].ID != "guard" {
		t.Fatalf("Metadata.Hooks = %+v, want [{guard}]", installed.Metadata.Hooks)
	}

	out, err := a.Hooks.Emit(ctx, event.Event{Type: event.PreToolUse, Tool: "toolsets_call"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Blocked() {
		t.Fatal("the installed skill's hook did not intercept a real Emit")
	}
	if out.HookID != installed.ID+"/guard" {
		t.Fatalf("HookID = %q, want %q", out.HookID, installed.ID+"/guard")
	}

	if err := a.Skills.Uninstall(ctx, skill.UninstallInput{ID: installed.ID}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	after, err := a.Hooks.Emit(ctx, event.Event{Type: event.PreToolUse, Tool: "toolsets_call"})
	if err != nil {
		t.Fatal(err)
	}
	if after.Blocked() {
		t.Fatal("the uninstalled skill's hook still intercepted Emit")
	}
}
