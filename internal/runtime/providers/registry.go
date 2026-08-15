// Package providers is the registry of model providers and the adapters that
// implement them.
//
// A provider registers itself from an init function, so adding one never
// touches the core. That is the open-closed principle doing something concrete
// rather than decorating a diagram: the loop knows the interface, the registry
// knows the names, and neither knows that Anthropic exists.
package providers

import (
	"sort"
	"sync"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// Config is what a provider is built from.
type Config struct {
	// APIKey is the credential. Empty is normal for the providers that
	// authenticate through an OAuth file another tool on this machine wrote.
	APIKey string

	// BaseURL overrides the endpoint, which is how one adapter serves several
	// OpenAI-compatible providers.
	BaseURL string

	// Home is the user's home directory, for the adapters that read a
	// credential file from it. Injected rather than read, so a test does not
	// have to own the real one.
	Home string
}

// Factory builds a provider.
type Factory func(cfg Config) (agentloop.LLMProvider, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register wires a provider id to its factory. Adapters call it from init.
func Register(id string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[id] = f
}

// Names lists the registered providers, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Build constructs a provider by id.
func Build(id string, cfg Config) (agentloop.LLMProvider, error) {
	mu.RLock()
	f, ok := registry[id]
	mu.RUnlock()
	if !ok {
		return nil, errUnknownProvider(id, Names())
	}
	return f(cfg)
}

// Slot is one capability of the installation: which provider and model answer
// speech, images, or the default conversation.
type Slot struct {
	Name     string
	Provider string
	Model    string
}

// Capability resolves a modality slot to a provider that implements it, and
// reports the two failure modes with distinct codes.
//
// The distinction is the original's and it is worth keeping: "you have not
// configured a voice model" and "the provider you configured cannot do speech"
// send a person to two different places, and one message for both sends them
// to the wrong one half the time.
func Capability[T any](slot Slot, cfg Config) (T, error) {
	var zero T
	if slot.Provider == "" || slot.Model == "" {
		return zero, errCapabilityModelMissing(slot.Name)
	}
	p, err := Build(slot.Provider, cfg)
	if err != nil {
		return zero, err
	}
	typed, ok := any(p).(T)
	if !ok {
		return zero, errCapabilityNotSupported(slot.Name, slot.Provider)
	}
	return typed, nil
}

func errUnknownProvider(id string, known []string) error {
	return apperr.New("PROVIDER_UNKNOWN").
		Causer("providers.Build").
		Msgf("there is no provider called %q", id).
		Issue("provider", id).
		Issue("available", known).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "the providers this build knows are in the issue; pick one of those",
			Command: build.Name + " config get",
		})
}

func errCapabilityModelMissing(slot string) error {
	return apperr.New("CAPABILITY_MODEL_MISSING").
		Causer("providers.Capability").
		Msgf("no model is configured for %s", slot).
		Issue("slot", slot).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "point the " + slot + " slot at a provider and a model in " + build.StateDir + "/config.json",
			Tool:  "config_update",
			Input: map[string]any{"set": map[string]any{
				"agents.models": map[string]any{slot: map[string]any{"provider": "openai", "model": "..."}},
			}},
		})
}

func errCapabilityNotSupported(slot, provider string) error {
	return apperr.New("CAPABILITY_NOT_SUPPORTED").
		Causer("providers.Capability").
		Msgf("the %s provider does not do %s", provider, slot).
		Issue("slot", slot).
		Issue("provider", provider).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "configure a different provider for this slot; the one you chose has no such capability",
		})
}
