package agentloop_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// TestTheFiveLevelsOfTheModelCascade. The table is the specification; a
// resolution that quietly picks something other than what an agent asked for is
// how a person ends up paying for a model they did not choose.
func TestTheFiveLevelsOfTheModelCascade(t *testing.T) {
	cases := []struct {
		name          string
		agent         agentloop.AgentModel
		cfg           agentloop.ConfigModel
		wantProvider  string
		wantModel     string
		wantReasoning agentloop.ReasoningLevel
	}{
		{
			name:         "the agent named both",
			agent:        agentloop.AgentModel{Provider: "anthropic", Model: "claude-sonnet-5"},
			cfg:          agentloop.ConfigModel{Provider: "openai", Model: "gpt-5"},
			wantProvider: "anthropic", wantModel: "claude-sonnet-5",
			wantReasoning: agentloop.ReasoningMedium,
		},
		{
			name:         "the agent named only a model",
			agent:        agentloop.AgentModel{Model: "gpt-5-mini"},
			cfg:          agentloop.ConfigModel{Provider: "openai", Model: "gpt-5"},
			wantProvider: "openai", wantModel: "gpt-5-mini",
			wantReasoning: agentloop.ReasoningMedium,
		},
		{
			name:         "the agent named nothing",
			cfg:          agentloop.ConfigModel{Provider: "google", Model: "gemini-3-flash", Reasoning: "low"},
			wantProvider: "google", wantModel: "gemini-3-flash",
			wantReasoning: agentloop.ReasoningLow,
		},
		{
			name:         "only a provider is configured",
			cfg:          agentloop.ConfigModel{Provider: "openai"},
			wantProvider: "openai", wantModel: agentloop.LastResortModel,
			wantReasoning: agentloop.ReasoningMedium,
		},
		{
			name:         "the agent named a provider the configuration does not know",
			agent:        agentloop.AgentModel{Provider: "anthropic"},
			cfg:          agentloop.ConfigModel{Provider: "openai", Model: "gpt-5"},
			wantProvider: "anthropic", wantModel: agentloop.LastResortModel,
			wantReasoning: agentloop.ReasoningMedium,
		},
		{
			name:         "the parenthesised form the original introduced",
			agent:        agentloop.AgentModel{Model: "Gemini 3 Flash (google)"},
			cfg:          agentloop.ConfigModel{Provider: "openai", Model: "gpt-5"},
			wantProvider: "google", wantModel: "Gemini 3 Flash",
			wantReasoning: agentloop.ReasoningMedium,
		},
		{
			name:         "an explicit provider beats the parenthesised one",
			agent:        agentloop.AgentModel{Provider: "openrouter", Model: "Gemini 3 Flash (google)"},
			wantProvider: "openrouter", wantModel: "Gemini 3 Flash",
			wantReasoning: agentloop.ReasoningMedium,
		},
		{
			name:         "the agent overrides the reasoning level",
			agent:        agentloop.AgentModel{Provider: "openai", Model: "gpt-5", Reasoning: "high"},
			cfg:          agentloop.ConfigModel{Provider: "openai", Model: "gpt-5", Reasoning: "low"},
			wantProvider: "openai", wantModel: "gpt-5",
			wantReasoning: agentloop.ReasoningHigh,
		},
		{
			name:         "an unrecognised level falls back to the default rather than to nothing",
			agent:        agentloop.AgentModel{Provider: "openai", Model: "gpt-5", Reasoning: "extreme"},
			wantProvider: "openai", wantModel: "gpt-5",
			wantReasoning: agentloop.ReasoningMedium,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := agentloop.Resolve(c.agent, c.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got.Provider != c.wantProvider || got.Model != c.wantModel {
				t.Fatalf("resolved to %s/%s, want %s/%s",
					got.Provider, got.Model, c.wantProvider, c.wantModel)
			}
			if got.Reasoning != c.wantReasoning {
				t.Errorf("reasoning = %s, want %s", got.Reasoning, c.wantReasoning)
			}
		})
	}
}

// TestNothingConfiguredAnywhereSaysWhatToConfigure, which is the fifth level.
func TestNothingConfiguredAnywhereSaysWhatToConfigure(t *testing.T) {
	_, err := agentloop.Resolve(agentloop.AgentModel{}, agentloop.ConfigModel{})
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_AGENT_PROVIDER_NOT_ENABLED" {
		t.Fatalf("err = %v", err)
	}
	// The slot is a map, so the fix is a payload rather than a command line —
	// and the payload has to be the one that actually works.
	if len(app.Actions) == 0 || app.Actions[0].Tool != "config_update" {
		t.Fatalf("the error does not say what to configure: %+v", app.Actions)
	}
	set, _ := app.Actions[0].Input.(map[string]any)["set"].(map[string]any)
	if _, ok := set["agents.models"]; !ok {
		t.Fatalf("the suggested payload does not set the slot: %+v", set)
	}
}

// TestAModelWithNoOwnerNamesTheAgentFileAsAFix. An agent that names a model and
// an installation that names no provider is the common half-configured state,
// and the fix is one line in either place.
func TestAModelWithNoOwnerNamesTheAgentFileAsAFix(t *testing.T) {
	_, err := agentloop.Resolve(agentloop.AgentModel{Model: "gpt-5"}, agentloop.ConfigModel{})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err = %v", err)
	}
	if app.Issues["model"] != "gpt-5" {
		t.Errorf("the error does not name the model: %+v", app.Issues)
	}
	var mentionsFrontMatter bool
	for _, a := range app.Actions {
		if strings.Contains(a.Label, "front matter") {
			mentionsFrontMatter = true
		}
	}
	if !mentionsFrontMatter {
		t.Errorf("the error offers only one of the two fixes: %+v", app.Actions)
	}
}
