package providers

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// pricing.json states USD per 1,000,000 tokens: public list pricing as
// published by each provider, not contractual, and it drifts — the file is
// meant to be edited in place when a provider changes theirs, not treated as
// a source of truth. A model or provider the file does not list costs $0 via
// CostUSD's own lookup miss, which undercounts rather than guesses; codex and
// gemini-cli are priced at 0 deliberately, not by omission — see Model
// Providers (Go)'s note on the OAuth adapters riding an already-paid
// subscription instead of a metered API.
//
//go:embed pricing.json
var pricingRaw []byte

// rate is one model's price, in USD per token — pricing.json states it per
// million, converted once here so CostUSD is a multiply, not a divide on
// every call.
type rate struct {
	Input  float64
	Output float64
	Cached float64
}

var pricing = loadPricing()

func loadPricing() map[string]map[string]rate {
	var raw map[string]map[string]struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
		Cached float64 `json:"cached"`
	}
	if err := json.Unmarshal(pricingRaw, &raw); err != nil {
		// pricing.json is embedded at build time — a parse failure here is a
		// broken build, not a runtime condition; fail loudly rather than
		// silently pricing every call at zero.
		panic("providers: pricing.json does not parse: " + err.Error())
	}
	out := make(map[string]map[string]rate, len(raw))
	for provider, models := range raw {
		out[provider] = make(map[string]rate, len(models))
		for model, p := range models {
			out[provider][model] = rate{
				Input: p.Input / 1_000_000, Output: p.Output / 1_000_000, Cached: p.Cached / 1_000_000,
			}
		}
	}
	return out
}

// CostUSD prices one turn's Usage against the provider/model it was billed
// under. A provider or model this table does not know about — a new release,
// a typo, an OAuth-subscription adapter with nothing in the table at all —
// costs $0 rather than refusing: undercounting a real spend is a table to
// update, not a reason to break every turn against an unlisted model.
//
// Cached tokens are priced once, at the cached rate, and are not also counted
// as Input — Usage.Input is already the non-cached remainder, the same
// convention every provider adapter in this package reports it under.
func CostUSD(provider, model string, u agentloop.Usage) float64 {
	byModel, ok := pricing[strings.ToLower(provider)]
	if !ok {
		return 0
	}
	r, ok := byModel[model]
	if !ok {
		return 0
	}
	return float64(u.Input)*r.Input + float64(u.Output)*r.Output + float64(u.Cached)*r.Cached
}
