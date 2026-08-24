package providers

import (
	"context"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// Model is one model a provider says it can serve.
//
// Two fields, because two are what a picker needs and everything past them
// differs per provider: a context window, a modality list, a price. A third
// field here would be real for one adapter and invented for the rest, which is
// the failure mode this whole file exists to end.
type Model struct {
	ID   string `json:"id" jsonschema:"The identifier to configure, spelled the way the provider spells it."`
	Name string `json:"name" jsonschema:"What to show a person: the provider's own display name, or the id when it publishes none."`
}

// Lister is the optional half of a provider adapter: asking the provider what
// it can serve, rather than this build carrying a list somebody has to keep up
// with.
//
// Optional on purpose. It is a real capability, not a formality — an adapter
// whose API publishes no catalogue should fail to implement this and be
// reported as undiscoverable, not return a hand-written list and call it
// discovery. That distinction is the whole point: a wrong list is worse than a
// missing one, because a wrong list is believed.
type Lister interface {
	Models(ctx context.Context) ([]Model, error)
}

// Models asks a provider what it can serve, with the credential in cfg.
func Models(ctx context.Context, id string, cfg Config) ([]Model, error) {
	p, err := Build(id, cfg)
	if err != nil {
		return nil, err
	}
	lister, ok := p.(Lister)
	if !ok {
		return nil, errNoModelDiscovery(id)
	}
	return lister.Models(ctx)
}

// SortModels orders a catalogue by name, for the providers whose API returns
// one in no meaningful order.
//
// It is a helper rather than something Models applies, because for some
// providers the order the API returns *is* the signal — Codex ranks its models
// and Anthropic returns newest first, and sorting either alphabetically would
// throw away the only ranking anybody has. The first entry is what a picker
// offers first, so this is not cosmetic.
func SortModels(models []Model) {
	sort.Slice(models, func(i, j int) bool {
		a, b := models[i], models[j]
		la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if la != lb {
			return la < lb
		}
		// A total order, so the same catalogue never renders in two orders.
		return a.ID < b.ID
	})
}

// NameOr returns the provider's own display name, or the id when it published
// none.
//
// Deliberately not a prettifier. Turning "gpt-5.4-mini" into "GPT-5.4 Mini"
// requires knowing that GPT is an initialism and 5.4 is a version, and the
// first model whose id does not follow that shape renders as nonsense. The id
// is what the person will see in the configuration file anyway.
func NameOr(display, id string) string {
	if s := strings.TrimSpace(display); s != "" {
		return s
	}
	return id
}

func errNoModelDiscovery(id string) error {
	return apperr.New("PROVIDER_NO_MODEL_DISCOVERY").
		Causer("providers.Models").
		Msgf("the %q provider cannot be asked which models it serves", id).
		Issue("provider", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "this provider publishes no catalogue; name the model yourself in the configuration",
			Command: build.Name + " config get",
		})
}
