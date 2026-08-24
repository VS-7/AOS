package google

import (
	"context"
	"net/url"
	"strings"

	"github.com/OWNER/aos/internal/runtime/providers"
)

// generateContent is the method this adapter calls. A model that does not list
// it cannot answer a turn here, whatever else it can do.
const generateContent = "generateContent"

// maxModelPages bounds the walk over a paginated catalogue. See the same
// constant in the anthropic adapter for why a bound exists at all.
const maxModelPages = 5

// Models asks ListModels which models it serves.
//
// This is the one provider that publishes what each model can actually do, so
// the filter here is the API's own answer rather than a guess about names: a
// model is offered when it lists generateContent among its supported methods.
// That drops the embedding, image and video models exactly, without this build
// having to know which those are — the reason the catalogue it replaces
// carried Imagen and Veo entries that no code path here could ever have used.
func (p *Provider) Models(ctx context.Context) ([]providers.Model, error) {
	var models []providers.Model
	token := ""

	for page := 0; page < maxModelPages; page++ {
		var out struct {
			Models []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		query := url.Values{"pageSize": {"1000"}}
		if token != "" {
			query.Set("pageToken", token)
		}
		if err := p.client.GetJSON(ctx, "/models?"+query.Encode(), &out); err != nil {
			return nil, err
		}
		for _, m := range out.Models {
			// The name comes back as a resource path; the id to configure is
			// the last segment of it.
			id := strings.TrimPrefix(m.Name, "models/")
			if id == "" || !supports(m.SupportedGenerationMethods, generateContent) {
				continue
			}
			models = append(models, providers.Model{
				ID:   id,
				Name: providers.NameOr(m.DisplayName, id),
			})
		}
		if out.NextPageToken == "" {
			break
		}
		token = out.NextPageToken
	}
	// ListModels returns no ranking and no stable order of its own.
	providers.SortModels(models)
	return models, nil
}

func supports(methods []string, want string) bool {
	for _, m := range methods {
		if m == want {
			return true
		}
	}
	return false
}
