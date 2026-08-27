package antigravity

import (
	"context"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/runtime/providers"
)

// catalogTTL is how long the model catalogue is reused.
//
// The catalogue carries this account's remaining allowance, so it is not
// static reference data and cannot be fetched once. Five minutes is long
// enough that a tool loop does not re-ask on every step, and short enough that
// an allowance which reset is noticed without restarting anything.
const catalogTTL = 5 * time.Minute

// modelInfo is what the service says about one model.
//
// The two fields that matter beyond the name are the thinking range and the
// quota. The range exists because this API rejects a thinking budget outside
// the model's own bounds with a 400 — a whole turn lost to a number this
// adapter chose — and the bounds differ per model, so guessing one is not an
// option. The quota exists so a turn that cannot succeed is refused before it
// is sent rather than after.
type modelInfo struct {
	ID          string
	Name        string
	Thinking    bool
	MinBudget   int
	Budget      int
	MaxInput    int
	MaxOutput   int
	QuotaLeft   float64
	QuotaReset  time.Time
	QuotaStated bool

	// internalOnly marks the entries a picker must not offer: routing
	// aliases with no display name, the models behind inline completion and
	// tab-jump, and anything the service flags itself.
	internalOnly bool
}

// catalog holds the two facts about this installation that a call needs and
// that only the service can answer: which project a request is billed to, and
// what each model's limits are.
//
// Both are cached, and both are best effort by design. Neither is required to
// make a call — the project field is optional on this API, verified against
// it — so a catalogue that cannot be fetched degrades the adapter to sending
// what it was given, rather than failing a turn over a lookup.
type catalog struct {
	client *providers.Client
	now    func() time.Time

	mu       sync.Mutex
	project  string
	resolved bool
	models   map[string]modelInfo
	fetched  time.Time
}

func newCatalog(client *providers.Client, clock func() time.Time) *catalog {
	if clock == nil {
		clock = timeNow
	}
	return &catalog{client: client, now: clock}
}

// loadCodeAssistResponse is the part of that call this adapter reads.
type loadCodeAssistResponse struct {
	CloudaicompanionProject string `json:"cloudaicompanionProject"`
	CurrentTier             struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"currentTier"`
}

// Project returns the Cloud AI Companion project this account's requests are
// billed to, or an empty string when the service did not name one.
//
// Empty is not an error and is not retried within a process. The field is
// optional on the generate endpoints, so a call made without it succeeds; what
// would not succeed is a turn that failed because a lookup it did not need
// went wrong. This adapter also never calls onboardUser to create a project:
// provisioning against somebody's account is the official client's business,
// and doing it from here is the kind of side effect that is very hard to
// undo.
func (c *catalog) Project(ctx context.Context) string {
	c.mu.Lock()
	if c.resolved {
		project := c.project
		c.mu.Unlock()
		return project
	}
	c.mu.Unlock()

	var out loadCodeAssistResponse
	err := c.client.PostJSON(ctx, ":loadCodeAssist", map[string]any{"metadata": clientMetadata()}, &out)

	c.mu.Lock()
	defer c.mu.Unlock()
	// A failed lookup is remembered as "no project" rather than retried on
	// every call: the endpoint is not needed, and hammering it because it
	// answered badly once is the opposite of what this package is for.
	c.resolved = true
	if err == nil {
		c.project = out.CloudaicompanionProject
	}
	return c.project
}

// fetchAvailableModelsResponse is the catalogue as the service publishes it: a
// map keyed by model id, not a list.
type fetchAvailableModelsResponse struct {
	Models map[string]struct {
		DisplayName       string `json:"displayName"`
		IsInternal        bool   `json:"isInternal"`
		SupportsThinking  bool   `json:"supportsThinking"`
		ThinkingBudget    int    `json:"thinkingBudget"`
		MinThinkingBudget int    `json:"minThinkingBudget"`
		MaxTokens         int    `json:"maxTokens"`
		MaxOutputTokens   int    `json:"maxOutputTokens"`
		QuotaInfo         *struct {
			RemainingFraction float64   `json:"remainingFraction"`
			ResetTime         time.Time `json:"resetTime"`
		} `json:"quotaInfo"`
	} `json:"models"`
}

// Models asks the service what it serves, reusing a recent answer.
func (c *catalog) Models(ctx context.Context, fresh bool) (map[string]modelInfo, error) {
	c.mu.Lock()
	if !fresh && c.models != nil && c.now().Sub(c.fetched) < catalogTTL {
		cached := c.models
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	var out fetchAvailableModelsResponse
	// The empty body is deliberate. This call takes no client metadata — it
	// answers 400 "Unknown name \"metadata\"" for the object every other
	// method here requires — and it takes no project either.
	if err := c.client.PostJSON(ctx, ":fetchAvailableModels", map[string]any{}, &out); err != nil {
		return nil, err
	}

	models := make(map[string]modelInfo, len(out.Models))
	for id, m := range out.Models {
		info := modelInfo{
			ID:        id,
			Name:      providers.NameOr(m.DisplayName, id),
			Thinking:  m.SupportsThinking,
			MinBudget: m.MinThinkingBudget,
			Budget:    m.ThinkingBudget,
			MaxInput:  m.MaxTokens,
			MaxOutput: m.MaxOutputTokens,
			// A missing display name is the tell for an entry that is not a
			// chat model: every one the official client offers has one, and
			// none of the routing aliases do.
			internalOnly: m.IsInternal || m.DisplayName == "",
		}
		if m.QuotaInfo != nil {
			info.QuotaLeft = m.QuotaInfo.RemainingFraction
			info.QuotaReset = m.QuotaInfo.ResetTime
			info.QuotaStated = true
		}
		models[id] = info
	}

	c.mu.Lock()
	c.models = models
	c.fetched = c.now()
	c.mu.Unlock()
	return models, nil
}

// Limits returns what is known about one model, or false when the catalogue
// could not be read. A caller that gets false sends a request without the
// fields it would have derived, which is what this API accepts by default.
func (c *catalog) Limits(ctx context.Context, model string) (modelInfo, bool) {
	models, err := c.Models(ctx, false)
	if err != nil {
		return modelInfo{}, false
	}
	info, ok := models[model]
	return info, ok
}

// Offerable lists the models worth putting in front of a person.
//
// The catalogue carries more than a picker should show: routing aliases with
// no display name, the models behind inline completion and tab-jump, and
// entries flagged internal. What is left is what the official client offers,
// derived from the service's own answer rather than from a list in this
// build that would go stale — which is the whole point of implementing
// discovery instead of hard-coding names.
func (c *catalog) Offerable(ctx context.Context) ([]providers.Model, error) {
	models, err := c.Models(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]providers.Model, 0, len(models))
	for _, m := range models {
		if m.internalOnly {
			continue
		}
		out = append(out, providers.Model{ID: m.ID, Name: m.Name})
	}
	providers.SortModels(out)
	return out, nil
}

// errQuotaExhausted is what a turn gets instead of being sent when this
// account has nothing left for the model it asked for.
//
// It is raised from the catalogue's own reading, before a request is made, and
// only after a fresh fetch — refusing a turn on a five-minute-old number would
// be its own kind of wrong.
func errQuotaExhausted(model string, reset time.Time) error {
	e := apperr.New("ANTIGRAVITY_QUOTA_EXHAUSTED").
		Causer("antigravity.Provider.preflight").
		Msgf("this account's Antigravity allowance for %s is used up", model).
		Issue("model", model).
		Status(apperr.StatusTooManyRequests).
		CTA(apperr.CallToAction{
			Label: "the allowance resets at the time in the issue; until then, point this agent at another provider rather than retrying",
		})
	// Appended rather than chained above so that the call to action stays in
	// the one expression the catalogue generator reads.
	if !reset.IsZero() {
		e = e.Issue("resetsAt", reset.UTC().Format(time.RFC3339))
	}
	return e
}
