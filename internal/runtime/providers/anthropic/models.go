package anthropic

import (
	"context"
	"net/url"

	"github.com/OWNER/aos/internal/runtime/providers"
)

// maxModelPages bounds the walk over a paginated catalogue.
//
// A page holds a thousand models and no provider has ever served a fraction of
// that, so reaching this bound means the cursor is not advancing — a server
// that always answers has_more is otherwise an infinite loop in a request
// handler, which is a hang rather than an error and therefore much harder to
// diagnose than the truncated list this produces instead.
const maxModelPages = 5

// Models asks the endpoint which models it serves.
//
// Every model this API returns is a conversational one — unlike the OpenAI
// catalogue, there is nothing to filter out and therefore nothing to guess at.
// The order is the API's own, newest first, and it is preserved: it is the only
// ranking anybody publishes, and the first entry is what a picker offers.
func (p *Provider) Models(ctx context.Context) ([]providers.Model, error) {
	var models []providers.Model
	after := ""

	for page := 0; page < maxModelPages; page++ {
		var out struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		query := url.Values{"limit": {"1000"}}
		if after != "" {
			query.Set("after_id", after)
		}
		if err := p.client.GetJSON(ctx, "/models?"+query.Encode(), &out); err != nil {
			return nil, err
		}
		for _, m := range out.Data {
			if m.ID == "" {
				continue
			}
			models = append(models, providers.Model{
				ID:   m.ID,
				Name: providers.NameOr(m.DisplayName, m.ID),
			})
		}
		if !out.HasMore || out.LastID == "" {
			break
		}
		after = out.LastID
	}
	return models, nil
}
