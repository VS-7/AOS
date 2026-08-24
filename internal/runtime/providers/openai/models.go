package openai

import (
	"context"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/runtime/providers"
)

// codexClientVersion is the Codex CLI version this adapter claims when it asks
// which models the subscription can reach.
//
// The endpoint requires it and gates on it: the same account answers three
// models at 0.111.0 and six at 0.144.0, because the server withholds a model
// whose `minimal_client_version` is above what the client says it is. Asking as
// an old client would reproduce, one level up, exactly the problem this file
// exists to remove — a list that silently goes stale and has to be edited by
// hand whenever the provider ships something.
//
// So the value is deliberately above every gate rather than pinned to a real
// release. That is only defensible because it is not a claim about a protocol:
// every tier the endpoint offers was driven through this adapter's plain
// /responses call and answered, including the ones whose metadata asks a
// first-party client to prefer a lighter transport. A model this adapter could
// not actually drive would have to be excluded here, not merely hidden.
const codexClientVersion = "0.999.0"

// Models asks the endpoint which models it serves.
func (p *Provider) Models(ctx context.Context) ([]providers.Model, error) {
	if p.codex {
		return p.codexModels(ctx)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := p.client.GetJSON(ctx, "/models", &out); err != nil {
		return nil, err
	}
	models := make([]providers.Model, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID == "" || !conversational(m.ID) {
			continue
		}
		// This API publishes no display name — the id is the name everywhere
		// OpenAI writes about these models, so it is what a person recognises.
		models = append(models, providers.Model{ID: m.ID, Name: m.ID})
	}
	// Alphabetical, which groups a family together. The endpoint returns no
	// ranking to preserve and its own order is not stable between calls.
	providers.SortModels(models)
	return models, nil
}

// codexModels asks the ChatGPT backend instead.
//
// A different shape from the public API's, and a richer one: this endpoint
// publishes a display name, a visibility, and a rank. All three are used —
// inventing any of them is what the static list did.
func (p *Provider) codexModels(ctx context.Context) ([]providers.Model, error) {
	var out struct {
		Models []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Visibility  string `json:"visibility"`
			Priority    int    `json:"priority"`
		} `json:"models"`
	}
	if err := p.client.GetJSON(ctx, "/models?client_version="+codexClientVersion, &out); err != nil {
		return nil, err
	}
	type ranked struct {
		model    providers.Model
		priority int
	}
	list := make([]ranked, 0, len(out.Models))
	for _, m := range out.Models {
		// "hide" is the server telling a client not to offer this one — it is
		// reachable, but it is an internal surface (the review model), not
		// something to put in front of a person.
		if m.Slug == "" || m.Visibility != "list" {
			continue
		}
		list = append(list, ranked{
			model:    providers.Model{ID: m.Slug, Name: providers.NameOr(m.DisplayName, m.Slug)},
			priority: m.Priority,
		})
	}
	// The endpoint's own ranking, lowest number first — which is how it says
	// "this is the one to offer". Preserving it means the model a picker
	// suggests first is the provider's recommendation, not this build's guess.
	sort.SliceStable(list, func(i, j int) bool { return list[i].priority < list[j].priority })

	models := make([]providers.Model, 0, len(list))
	for _, r := range list {
		models = append(models, r.model)
	}
	return models, nil
}

// notConversational are the id fragments of the families this adapter cannot
// hold a conversation with.
//
// The public /models endpoint returns every model on the account — embeddings,
// speech, transcription, moderation, image and video generation — with no field
// saying which is which, so this is a filter on names, and a filter on names is
// a guess by construction. It is kept to families that are unambiguously not
// chat models, and errs towards showing too much: a model wrongly listed is one
// a person tries once and abandons, while a model wrongly hidden is one they
// cannot reach at all and have no way to discover is missing.
var notConversational = []string{
	"embedding", "moderation", "whisper", "tts", "transcribe",
	"dall-e", "image", "audio", "realtime", "sora",
	// The 002 base completions models, which are not chat models at all.
	"babbage", "davinci",
}

func conversational(id string) bool {
	lower := strings.ToLower(id)
	for _, fragment := range notConversational {
		if strings.Contains(lower, fragment) {
			return false
		}
	}
	return true
}
