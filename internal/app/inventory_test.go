package app_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/instruction"
	"github.com/OWNER/aos/internal/domain/project"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/template"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
)

// TestTheAssembledPromptCarriesEveryInventoryCategory closes the gap
// runtime.go's own reader.Inventory used to document with a comment: "the
// remaining categories are the aggregates of the phases that come after this
// one." Those phases (skills, templates, views, goals, routines, projects,
// artifacts, instructions) are all built now, so an agent's own context
// document should say what exists in every one of them, not just
// collections and agents — this proves it does, against the real
// composition root and a real assembled prompt, not the reader in
// isolation.
func TestTheAssembledPromptCarriesEveryInventoryCategory(t *testing.T) {
	a, _ := conversing(t)
	ctx := agentCtx()

	// A skill brings a view (and a collection, already covered before this
	// fix) along with it — installing one is the cheapest way to also prove
	// a skill-scoped view reaches the inventory, not only a user-owned one.
	if _, err := a.Skills.Install(ctx, skill.InstallInput{
		Source:      "testdata/crm-skill",
		AcceptedAll: func(skill.Permissions) bool { return true },
	}); err != nil {
		t.Fatalf("install: %v", err)
	}

	tpl, err := a.Templates.Create(ctx, template.CreateInput{
		ID: "brief", Name: "Brief", Content: "# {{ name }}",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	gl, err := a.Goals.Create(ctx, goal.CreateInput{Title: "Ship the inventory fix"})
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	rt, err := a.Routines.Create(ctx, routine.CreateInput{
		Name: "Morning triage", Agent: "atlas", Content: "Triage anything new.",
	})
	if err != nil {
		t.Fatalf("create routine: %v", err)
	}
	pr, err := a.Projects.Create(ctx, project.CreateInput{Name: "Rewrite"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	art, err := a.Artifacts.Create(ctx, artifact.CreateInput{Name: "Status dashboard"})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	ins, err := a.Instructions.Create(ctx, instruction.CreateInput{Name: "Feature Protocol"})
	if err != nil {
		t.Fatalf("create instruction: %v", err)
	}

	provider := play(t, fake.Step{
		Text:  "Everything is in place.",
		Usage: agentloop.Usage{Input: 100, Output: 10},
	})

	sent, err := a.Chats.Send(ctx, chat.SendInput{
		Chat: mustChat(t, a), Text: "what do we have?", Agent: "atlas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sent.Dispatched {
		t.Fatal("the message was stored and nobody was asked to answer it")
	}
	waitForAnswer(t, a, sent.Message.ID)

	instructions := provider.Requests()[0].Instructions
	want := map[string]string{
		"skills":       "crm",
		"views":        "contacts-table",
		"templates":    tpl.ID,
		"goals":        gl.ID,
		"routines":     rt.Routine.ID,
		"projects":     pr.ID,
		"artifacts":    art.ID,
		"instructions": ins.ID,
	}
	for category, id := range want {
		if !strings.Contains(instructions, id) {
			t.Errorf("the assembled prompt's %s category is missing %q\n%s", category, id, instructions)
		}
	}
}
