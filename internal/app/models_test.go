package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
)

// refusingCatalogue is a provider whose catalogue cannot be read, counting how
// often it is asked.
type refusingCatalogue struct{ *fake.Provider }

var refusingAsked atomic.Int32

func (refusingCatalogue) Models(context.Context) ([]providers.Model, error) {
	refusingAsked.Add(1)
	return nil, errors.New("AOS_OAUTH_REFRESH_FAILED: the credential could not be renewed")
}

func init() {
	providers.Register("refusingcatalogue", func(providers.Config) (agentloop.LLMProvider, error) {
		return refusingCatalogue{fake.Text("unused")}, nil
	})
}

// Settings asks for the catalogue on every render, and a provider that cannot
// answer was asked again every time: another renewal attempt against somebody
// else's credential file, another lock file beside it, another warning in the
// log. A failure is kept briefly — briefly, because the usual fixes (a key
// pasted again, a login refreshed) are made and retried within seconds.
func TestAFailedCatalogueIsNotAskedAgainOnEveryRender(t *testing.T) {
	ctx := context.Background()
	cfg := config.NewService(fsconfig.New(filepath.Join(t.TempDir(), "config.json")))
	if _, err := cfg.Update(ctx, config.UpdateInput{Set: map[string]any{
		"agents.providers": []map[string]any{{"id": "refusingcatalogue", "key": ""}},
	}}); err != nil {
		t.Fatal(err)
	}
	clock := &clockx.Stepping{At: time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)}
	catalog := newModelCatalog(cfg, t.TempDir(), clock)
	refusingAsked.Store(0)

	for range 3 {
		if _, err := catalog.Models(ctx, "refusingcatalogue"); err == nil {
			t.Fatal("a failed catalogue answered as if it had succeeded")
		}
	}
	if got := refusingAsked.Load(); got != 1 {
		t.Fatalf("the provider was asked %d times in a row, want once", got)
	}

	clock.Set(clock.Now().Add(catalogFailureTTL + time.Second))
	if _, err := catalog.Models(ctx, "refusingcatalogue"); err == nil {
		t.Fatal("a failed catalogue answered as if it had succeeded")
	}
	if got := refusingAsked.Load(); got != 2 {
		t.Fatalf("the provider was asked %d times, want it asked again once the failure is stale", got)
	}

	// A changed credential is a reason to ask at once.
	if _, err := cfg.Update(ctx, config.UpdateInput{Set: map[string]any{
		"agents.providers": []map[string]any{{"id": "refusingcatalogue", "key": "a-new-key"}},
	}}); err != nil {
		t.Fatal(err)
	}
	_, _ = catalog.Models(ctx, "refusingcatalogue")
	if got := refusingAsked.Load(); got != 3 {
		t.Fatalf("the provider was asked %d times, want a new credential tried at once", got)
	}
}
