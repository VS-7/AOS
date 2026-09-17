package app

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/runtime/providers"
)

func modelsOver(t *testing.T, set map[string]any) models {
	t.Helper()
	cfg := config.NewService(fsconfig.New(filepath.Join(t.TempDir(), "config.json")))
	if _, err := cfg.Update(context.Background(), config.UpdateInput{Set: set}); err != nil {
		t.Fatal(err)
	}
	return models{config: cfg, home: t.TempDir()}
}

// Disconnecting Anthropic left the Default slot naming it, and the next chat
// went out to Anthropic with no key at all: the person read "the anthropic
// provider answered 401: x-api-key header is required" about a provider they
// had just disconnected, with nothing on the Providers screen to explain it.
func TestATurnOnAProviderThatIsNotConnectedIsRefusedBeforeItIsSent(t *testing.T) {
	m := modelsOver(t, map[string]any{
		"agents.providers": []map[string]any{{"id": "openai", "key": "sk-live"}},
		"agents.models": map[string]any{
			"default": map[string]any{"provider": "anthropic", "model": "claude-opus-5"},
		},
	})

	_, _, err := m.For(context.Background(), &agent.Agent{ID: "luara"})
	e, ok := apperr.As(err)
	if !ok || e.Code != "AOS_AGENT_PROVIDER_NOT_CONNECTED" {
		t.Fatalf("error = %v, want the provider named as not connected", err)
	}
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("error kind = %v, want a configuration problem", err)
	}
	if !strings.Contains(e.Message, "anthropic") || !strings.Contains(e.Message, "Default") {
		t.Errorf("message = %q, want the provider and where it was chosen", e.Message)
	}

	// The same provider named by the agent itself says so.
	_, _, err = m.For(context.Background(), &agent.Agent{ID: "api-builder", Provider: "anthropic", Model: "claude-opus-5"})
	if e, _ := apperr.As(err); e == nil || !strings.Contains(e.Message, "api-builder") {
		t.Errorf("error = %v, want the agent whose own model names the provider", err)
	}
}

// A provider whose credential is another tool's login file needs no entry of
// its own to be reached, and one connected with a key is used as before.
func TestConnectedAndLoginFileProvidersStillResolve(t *testing.T) {
	m := modelsOver(t, map[string]any{
		"agents.providers": []map[string]any{{"id": "openai", "key": "sk-live"}},
		"agents.models": map[string]any{
			"default": map[string]any{"provider": "openai", "model": "gpt-5.1"},
		},
	})
	if _, ref, err := m.For(context.Background(), &agent.Agent{ID: "luara"}); err != nil || ref.Provider != "openai" {
		t.Fatalf("ref = %+v, err = %v", ref, err)
	}
	if _, _, err := m.For(context.Background(), &agent.Agent{ID: "api-builder", Provider: "codex", Model: "gpt-5.6-terra"}); err != nil {
		if e, _ := apperr.As(err); e != nil && e.Code == "AOS_AGENT_PROVIDER_NOT_CONNECTED" {
			t.Fatalf("a login-file provider was refused for having no key entry: %v", err)
		}
	}
}

// The list is names, so a renamed adapter must break this rather than quietly
// stop being checked.
func TestEveryKeyedProviderIsARegisteredOne(t *testing.T) {
	for id := range keyedProviders {
		if !slices.Contains(providers.Names(), id) {
			t.Errorf("%q is not a registered provider", id)
		}
	}
}
