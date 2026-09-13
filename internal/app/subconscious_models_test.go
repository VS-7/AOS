package app

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
)

// observerModels builds the observer's resolver over a real installation,
// capturing what it hands the provider factory.
func observerModels(t *testing.T, slots map[string]any) (subconsciousModels, *App, *struct{ provider, key string }) {
	t.Helper()
	root := t.TempDir()
	a, err := New(Options{
		Env:           env.New(env.Map(map[string]string{env.KeyHome: t.TempDir(), env.KeyWorkspaceID: "atelier"})),
		WorkspaceRoot: root,
		Clock:         &clockx.Stepping{At: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC), Step: time.Second},
		IDs:           &ids.Sequence{Prefix: "id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	ctx := context.Background()
	if _, err := a.Config.Update(ctx, config.UpdateInput{Set: map[string]any{
		"agents.providers": []map[string]any{
			{"id": "openai", "key": "sk-proj-REALKEYabcdefgh1234"},
			{"id": "codex", "key": ""},
		},
		"agents.models": slots,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: "Atelier", Path: root}); err != nil {
		t.Fatal(err)
	}

	built := &struct{ provider, key string }{}
	return subconsciousModels{
		config: a.Config, agents: a.Agents,
		build: func(provider, key string) (agentloop.LLMProvider, error) {
			built.provider, built.key = provider, key
			return fake.Text("observed"), nil
		},
	}, a, built
}

// The observer was handed the redacted configuration, so the key it built an
// API-key provider with was the fingerprint "***…1234": every observation
// authenticated with that, was refused, and background memory never formed for
// anybody on openai, anthropic, google or openrouter.
func TestTheObserverAuthenticatesWithTheRealKey(t *testing.T) {
	models, _, built := observerModels(t, map[string]any{
		"default":      map[string]any{"provider": "openai", "model": "gpt-5.5"},
		"subconscious": map[string]any{"provider": "openai", "model": "gpt-5.4-mini"},
	})

	_, ref, err := models.Subconscious(context.Background(), "atlas")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Provider != "openai" || ref.Model != "gpt-5.4-mini" {
		t.Errorf("ref = %+v, want the subconscious slot", ref)
	}
	if built.key != "sk-proj-REALKEYabcdefgh1234" {
		t.Fatalf("the provider was built with key %q, want the key the person saved", built.key)
	}
}

// The subconscious slot exists to run a cheap observer beside an expensive
// agent. An agent that names its own model — api-builder on gpt-5.6-terra —
// was observed with that model instead, because the agent's own model outranks
// the configuration in the turn's cascade and the observer reused it.
func TestTheSubconsciousSlotWinsOverTheAgentsOwnModel(t *testing.T) {
	models, a, _ := observerModels(t, map[string]any{
		"default":      map[string]any{"provider": "openai", "model": "gpt-5.5"},
		"subconscious": map[string]any{"provider": "openai", "model": "gpt-5.4-mini", "reasoning": "low"},
	})
	provider, model := "codex", "gpt-5.6-terra"
	if _, err := a.Agents.Update(context.Background(), agent.UpdateInput{ID: "atlas", Provider: &provider, Model: &model}); err != nil {
		t.Fatal(err)
	}

	_, ref, err := models.Subconscious(context.Background(), "atlas")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Provider != "openai" || ref.Model != "gpt-5.4-mini" || ref.Reasoning != agentloop.ReasoningLow {
		t.Fatalf("ref = %+v, want the subconscious slot as configured", ref)
	}
}

// With no subconscious slot, the observer falls back the way a turn resolves:
// the agent's own model, then the default slot.
func TestWithoutASubconsciousSlotTheObserverFollowsTheAgent(t *testing.T) {
	models, a, _ := observerModels(t, map[string]any{
		"default": map[string]any{"provider": "openai", "model": "gpt-5.5"},
	})

	_, ref, err := models.Subconscious(context.Background(), "atlas")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Provider != "openai" || ref.Model != "gpt-5.5" {
		t.Errorf("ref = %+v, want the default slot for an agent with no model", ref)
	}

	provider, model := "codex", "gpt-5.6-terra"
	if _, err := a.Agents.Update(context.Background(), agent.UpdateInput{ID: "atlas", Provider: &provider, Model: &model}); err != nil {
		t.Fatal(err)
	}
	_, ref, err = models.Subconscious(context.Background(), "atlas")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Provider != "codex" || ref.Model != "gpt-5.6-terra" {
		t.Errorf("ref = %+v, want the agent's own model", ref)
	}
}
