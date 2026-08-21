package providers_test

import (
	"testing"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
)

func TestCostUSDPricesAKnownModel(t *testing.T) {
	got := providers.CostUSD("anthropic", "claude-sonnet-4-5", agentloop.Usage{
		Input: 1_000_000, Output: 1_000_000, Cached: 1_000_000,
	})
	// 1M input at $3/M + 1M output at $15/M + 1M cached at $0.3/M.
	want := 3.0 + 15.0 + 0.3
	if got != want {
		t.Fatalf("CostUSD = %v, want %v", got, want)
	}
}

func TestCostUSDIsZeroForAnUnknownProvider(t *testing.T) {
	if got := providers.CostUSD("not-a-real-provider", "whatever", agentloop.Usage{Input: 1000}); got != 0 {
		t.Fatalf("CostUSD for an unknown provider = %v, want 0", got)
	}
}

func TestCostUSDIsZeroForAnUnknownModel(t *testing.T) {
	if got := providers.CostUSD("anthropic", "not-a-real-model", agentloop.Usage{Input: 1000}); got != 0 {
		t.Fatalf("CostUSD for an unknown model = %v, want 0", got)
	}
}

func TestCostUSDIsZeroForTheOAuthSubscriptionAdapters(t *testing.T) {
	// codex and gemini-cli ride an already-paid subscription rather than a
	// metered API — see Model Providers (Go)'s own note. Zero here is the
	// documented behaviour, not a lookup miss.
	for provider, model := range map[string]string{"codex": "gpt-5.1", "gemini-cli": "gemini-3-pro"} {
		if got := providers.CostUSD(provider, model, agentloop.Usage{Input: 1_000_000, Output: 1_000_000}); got != 0 {
			t.Fatalf("CostUSD(%q, %q) = %v, want 0", provider, model, got)
		}
	}
}
