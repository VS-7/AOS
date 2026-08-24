package compat

import (
	"context"

	"github.com/OWNER/aos/internal/runtime/providers"
)

// Models asks the gateway which models it serves.
//
// This is the case discovery was most needed for. These three route to hundreds
// of models from many vendors, and the catalogue changes without this build
// being rebuilt — which is why the static list for all three was empty rather
// than wrong. Empty was the honest answer to a question nothing could ask; now
// something can ask it.
//
// The free-model routing rule stays out of this deliberately: which endpoint a
// model's *call* goes to is clientFor's business, and the catalogue is served
// off the configured base URL either way.
func (p *Provider) Models(ctx context.Context) ([]providers.Model, error) {
	var out struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := p.client.GetJSON(ctx, "/models", &out); err != nil {
		return nil, err
	}
	models := make([]providers.Model, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID == "" {
			continue
		}
		// OpenRouter publishes a display name; the plainer OpenAI-compatible
		// gateways publish only an id.
		models = append(models, providers.Model{ID: m.ID, Name: providers.NameOr(m.Name, m.ID)})
	}
	providers.SortModels(models)
	return models, nil
}
