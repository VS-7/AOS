package model_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/model"
)

// fakeCatalog is the port, scripted: who is connected, and what each provider
// answers when asked.
type fakeCatalog struct {
	connected    []string
	connectedErr error

	answers map[string][]model.Model
	fails   map[string]error

	mu    sync.Mutex
	asked []string
}

func (f *fakeCatalog) Connected(context.Context) ([]string, error) {
	if f.connectedErr != nil {
		return nil, f.connectedErr
	}
	return f.connected, nil
}

func (f *fakeCatalog) Models(_ context.Context, provider string) ([]model.Model, error) {
	f.mu.Lock()
	f.asked = append(f.asked, provider)
	f.mu.Unlock()

	if err, ok := f.fails[provider]; ok {
		return nil, err
	}
	return f.answers[provider], nil
}

func (f *fakeCatalog) askedProviders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.asked))
	copy(out, f.asked)
	return out
}

func newService(c model.Catalog) *model.Service {
	return model.NewService(model.Deps{
		Catalog: c,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func reason() command.Reasoning {
	return command.Reasoning{Reasoning: "a test asking what the providers serve"}
}

func TestListAsksEveryConnectedProviderAndOrdersTheAnswer(t *testing.T) {
	catalog := &fakeCatalog{
		connected: []string{"openai", "anthropic"},
		answers: map[string][]model.Model{
			"openai":    {{ID: "gpt-5.1", Name: "gpt-5.1"}},
			"anthropic": {{ID: "claude-opus-5", Name: "Claude Opus 5"}, {ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5"}},
		},
	}

	out, err := newService(catalog).List(context.Background(), model.ListInput{Reasoning: reason()})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out.Providers) != 2 {
		t.Fatalf("asked %d providers, wanted 2", len(out.Providers))
	}
	// Ordered by id, so the same installation never renders in two orders.
	if out.Providers[0].ID != "anthropic" || out.Providers[1].ID != "openai" {
		t.Fatalf("out of order: %s then %s", out.Providers[0].ID, out.Providers[1].ID)
	}
	if out.Total != 3 {
		t.Fatalf("total %d, wanted 3", out.Total)
	}
	// The provider's own order inside its own catalogue is preserved: it is
	// the only ranking anybody publishes, and the first entry is the one a
	// picker offers.
	if out.Providers[0].Models[0].ID != "claude-opus-5" {
		t.Fatalf("provider order not preserved: %v", out.Providers[0].Models)
	}
}

func TestOneProviderFailingDoesNotHideTheOthers(t *testing.T) {
	catalog := &fakeCatalog{
		connected: []string{"anthropic", "google"},
		answers:   map[string][]model.Model{"anthropic": {{ID: "claude-opus-5", Name: "Claude Opus 5"}}},
		fails:     map[string]error{"google": errors.New("the google provider answered 401")},
	}

	out, err := newService(catalog).List(context.Background(), model.ListInput{Reasoning: reason()})
	if err != nil {
		t.Fatalf("one broken credential failed the whole call: %v", err)
	}
	if len(out.Providers) != 2 {
		t.Fatalf("the failing provider was dropped: %v", out.Providers)
	}
	broken := out.Providers[1]
	if broken.ID != "google" {
		t.Fatalf("wrong entry: %s", broken.ID)
	}
	if !strings.Contains(broken.Error, "401") {
		t.Fatalf("the reason did not survive: %q", broken.Error)
	}
	if broken.Models == nil {
		t.Fatal("a failed catalogue must be an empty list, not a null one")
	}
	if out.Total != 1 {
		t.Fatalf("total %d, wanted 1 — only the working provider counts", out.Total)
	}
}

func TestNamingOneProviderAsksOnlyThatOne(t *testing.T) {
	catalog := &fakeCatalog{
		connected: []string{"anthropic", "openai"},
		answers: map[string][]model.Model{
			"anthropic": {{ID: "claude-opus-5", Name: "Claude Opus 5"}},
			"openai":    {{ID: "gpt-5.1", Name: "gpt-5.1"}},
		},
	}

	out, err := newService(catalog).List(context.Background(), model.ListInput{Provider: "openai", Reasoning: reason()})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if asked := catalog.askedProviders(); len(asked) != 1 || asked[0] != "openai" {
		t.Fatalf("asked %v, wanted only openai", asked)
	}
	if len(out.Providers) != 1 || out.Providers[0].ID != "openai" {
		t.Fatalf("answered %v", out.Providers)
	}
}

func TestNamingAProviderThatIsNotConnectedIsRefusedWithTheOnesThatAre(t *testing.T) {
	catalog := &fakeCatalog{connected: []string{"anthropic"}}

	_, err := newService(catalog).List(context.Background(), model.ListInput{Provider: "openai", Reasoning: reason()})
	if err == nil {
		t.Fatal("an unconnected provider answered as though it served nothing")
	}
	var e *apperr.Error
	if !errors.As(err, &e) || e.Code != "AOS_MODEL_PROVIDER_NOT_CONNECTED" {
		t.Fatalf("wrong error: %v", err)
	}
	if len(catalog.askedProviders()) != 0 {
		t.Fatal("a provider with no credential was asked anyway")
	}
}

func TestListReportsAConfigurationItCannotRead(t *testing.T) {
	catalog := &fakeCatalog{connectedErr: errors.New("the configuration is unreadable")}

	if _, err := newService(catalog).List(context.Background(), model.ListInput{Reasoning: reason()}); err == nil {
		t.Fatal("an unreadable configuration answered as though nothing were connected")
	}
}

func TestAProviderThatAnswersNothingIsAnEmptyListNotANullOne(t *testing.T) {
	catalog := &fakeCatalog{connected: []string{"crof"}}

	out, err := newService(catalog).List(context.Background(), model.ListInput{Reasoning: reason()})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if out.Providers[0].Models == nil {
		t.Fatal("a null list renders as a broken picker rather than an empty one")
	}
	if out.Providers[0].Error != "" {
		t.Fatalf("an empty catalogue is not a failure: %q", out.Providers[0].Error)
	}
}

func TestAServiceWiredWithNoCatalogSaysSoInsteadOfPanicking(t *testing.T) {
	_, err := newService(nil).List(context.Background(), model.ListInput{Reasoning: reason()})
	if err == nil {
		t.Fatal("a service with no catalogue answered")
	}
	var e *apperr.Error
	if !errors.As(err, &e) || e.Code != "AOS_MODEL_CATALOG_UNAVAILABLE" {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestNoProvidersConnectedIsAnEmptyAnswerNotAnError(t *testing.T) {
	out, err := newService(&fakeCatalog{}).List(context.Background(), model.ListInput{Reasoning: reason()})
	if err != nil {
		t.Fatalf("a fresh installation failed instead of answering empty: %v", err)
	}
	if len(out.Providers) != 0 || out.Total != 0 {
		t.Fatalf("answered %v", out)
	}
}
