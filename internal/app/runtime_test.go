package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
)

// keyCapture records the APIKey a provider factory was actually built with,
// so a test can tell a real secret apart from config.Fingerprint's masked
// stand-in for it.
var keyCapture struct {
	mu  sync.Mutex
	got string
}

func init() {
	providers.Register("keycapture", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		keyCapture.mu.Lock()
		keyCapture.got = cfg.APIKey
		keyCapture.mu.Unlock()
		return fake.Text("captured"), nil
	})
}

// TestModelResolutionUsesTheRealProviderKeyNotItsFingerprint is the
// regression test for a defect where models.For (internal/app/runtime.go)
// asked config.Service for the same redacted view command_get hands an
// agent, instead of Raw. keyFor then read the fingerprint back out of it, so
// every API-key provider call would have authenticated against the real
// provider with something like "●●●●…alue" instead of the key the user
// saved through the AI Provider settings screen — silently, since the
// config file itself always held the real value; only the runtime's own
// read of it was wrong.
func TestModelResolutionUsesTheRealProviderKeyNotItsFingerprint(t *testing.T) {
	a, _ := conversing(t)
	ctx := agentCtx()

	const secret = "sk-live-provider-secret-value"
	if _, err := a.Config.Update(context.Background(), config.UpdateInput{Set: map[string]any{
		"agents.providers": []map[string]any{
			{"id": "keycapture", "key": secret},
		},
		// Same slots conversing() already set, default's provider swapped
		// for the one this test can inspect; subconscious is repeated
		// unchanged since the map is a patch leaf (whole-value replace).
		"agents.models": map[string]any{
			"default": map[string]any{
				"provider": "keycapture", "model": "test-model", "reasoning": "medium",
			},
			"subconscious": map[string]any{
				"provider": "observer", "model": "small-model",
			},
		},
	}}); err != nil {
		t.Fatal(err)
	}

	sent, err := a.Chats.Send(ctx, chat.SendInput{
		Chat: mustChat(t, a), Text: "hi", Agent: "atlas",
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForAnswer(t, a, sent.Message.ID)

	keyCapture.mu.Lock()
	got := keyCapture.got
	keyCapture.mu.Unlock()

	if got != secret {
		t.Fatalf("the keycapture provider factory received APIKey %q, want the real secret %q — "+
			"model resolution is reading the redacted config instead of Raw", got, secret)
	}
}
