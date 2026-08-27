package antigravity

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/google"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// DefaultBaseURL is the endpoint the shipped Antigravity CLI calls.
//
// The "daily-" host is not a staging address despite how it reads: it is what
// the current client hard-codes and the only one that answers for a personal
// account, confirmed against both this machine's client logs and the endpoint
// itself. The plain cloudcode-pa host is still in the binary and still serves
// the older Code Assist surface; pointing this adapter at it is a BaseURL away
// for an installation that needs to.
const DefaultBaseURL = "https://daily-cloudcode-pa.googleapis.com/v1internal"

// clientVersion is the version this adapter announces.
//
// It tracks the CLI whose protocol was read, and it is a constant rather than
// something discovered: a request that claims to be a version that never
// existed is worse than one that claims an older real one.
const clientVersion = "1.1.21"

// budgetCeiling is the largest thinking budget this API accepts. It answers
// 400 with the range in the message for anything above it, which is where this
// number comes from rather than from documentation.
const budgetCeiling = 65535

func init() {
	providers.Register("antigravity", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return New(cfg), nil
	})
}

// Provider is the adapter.
type Provider struct {
	client  *providers.Client
	catalog *catalog
	guard   *guard
}

// New builds it.
//
// cfg.APIKey is accepted and ignored on purpose. Every other provider in the
// registry treats it as the credential, and an installation that pasted one
// here should not have it quietly sent to Google as a bearer token; the
// credential for this provider is the login on the machine, and nothing else.
func New(cfg providers.Config) *Provider {
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	creds := newTokens(cfg.Home, nil)

	client := providers.NewClient("antigravity", base, headers())
	client.Auth = func(ctx context.Context, r *http.Request) error {
		token, err := creds.Token(ctx)
		if err != nil {
			return err
		}
		r.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
	return &Provider{
		client:  client,
		catalog: newCatalog(client, nil),
		guard:   newGuard(nil),
	}
}

// Name reports the provider id.
func (p *Provider) Name() string { return "antigravity" }

// Generate makes one call.
func (p *Provider) Generate(ctx context.Context, req agentloop.Request) (agentloop.Response, error) {
	body, err := p.envelope(ctx, req)
	if err != nil {
		return agentloop.Response{}, err
	}
	var out envelopedResponse
	err = p.client.PostJSON(ctx, ":generateContent", body, &out)
	p.guard.observe(err)
	if err != nil {
		return agentloop.Response{}, err
	}
	return translate(out.Response, req.Model), nil
}

// Stream makes the same call against the streaming endpoint.
func (p *Provider) Stream(ctx context.Context, req agentloop.Request) (agentloop.Stream, error) {
	body, err := p.envelope(ctx, req)
	if err != nil {
		return nil, err
	}
	reader, err := p.client.PostSSE(ctx, ":streamGenerateContent?alt=sse", body)
	p.guard.observe(err)
	if err != nil {
		return nil, err
	}
	return &stream{reader: reader, model: req.Model, guard: p.guard}, nil
}

// Models asks the service what it serves. It is the Lister half of the
// adapter, and it answers from the service's own catalogue rather than a list
// carried in this build.
func (p *Provider) Models(ctx context.Context) ([]providers.Model, error) {
	if err := p.guard.enter(ctx); err != nil {
		return nil, err
	}
	found, err := p.catalog.Offerable(ctx)
	p.guard.observe(err)
	return found, err
}

// envelope builds the request and clears it to be sent.
//
// The two things that happen before a call are here rather than in Generate
// and Stream separately, because they are the same two things and forgetting
// one in the streaming path is exactly the sort of omission that costs an
// account: pace what leaves this process, and refuse a turn whose allowance is
// already spent instead of discovering it from a 429.
func (p *Provider) envelope(ctx context.Context, req agentloop.Request) (map[string]any, error) {
	limits, known := p.catalog.Limits(ctx, req.Model)
	if err := p.preflight(ctx, req.Model, limits, known); err != nil {
		return nil, err
	}
	if err := p.guard.enter(ctx); err != nil {
		return nil, err
	}

	inner := map[string]any{"contents": contents(req.Messages)}
	if req.Instructions != "" {
		inner["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.Instructions}},
		}
	}
	if len(req.Tools) > 0 {
		inner["tools"] = []map[string]any{{"functionDeclarations": toolDefs(req.Tools)}}
	}
	if cfg := generationConfig(limits, known, req.Reasoning); cfg != nil {
		inner["generationConfig"] = cfg
	}
	// The escape hatch, applied last so an installation can override anything
	// derived above — including a generationConfig this adapter got wrong for
	// a model that shipped after it.
	for k, v := range req.Options {
		inner[k] = v
	}

	out := map[string]any{
		"model": req.Model,
		// The literal the official client sends. It is not a user agent in
		// the HTTP sense — that header is set separately — and the service
		// takes it as the name of the surface making the request.
		"userAgent": "antigravity",
		"request":   inner,
	}
	// Optional, and verified so: a call with no project succeeds. It is sent
	// when the service named one so that usage is attributed the way the
	// official client attributes it, and omitted rather than invented when
	// it did not.
	if project := p.catalog.Project(ctx); project != "" {
		out["project"] = project
	}
	return out, nil
}

// preflight refuses a turn this account cannot pay for.
//
// It only ever refuses on a number it has just re-read. A five-minute-old zero
// is not evidence the allowance is still empty, and refusing a turn that would
// have worked is its own failure — so the cached reading is the trigger to go
// and look, never the reason to stop.
func (p *Provider) preflight(ctx context.Context, model string, limits modelInfo, known bool) error {
	if !known || !limits.QuotaStated || limits.QuotaLeft > 0 {
		return nil
	}
	models, err := p.catalog.Models(ctx, true)
	if err != nil {
		// The catalogue is not the authority on whether a call may be made;
		// it is an optimisation over finding out. If it cannot be read, the
		// call goes and the service answers for itself.
		return nil
	}
	again, found := models[model]
	if !found || !again.QuotaStated || again.QuotaLeft > 0 {
		return nil
	}
	p.guard.block(again.QuotaReset, "the Antigravity allowance for "+model+" is used up")
	return errQuotaExhausted(model, again.QuotaReset)
}

// headers is what every request carries.
//
// These are the official client's, and matching them is a deliberate part of
// this adapter rather than cargo cult: the endpoint is an internal one that
// serves a specific product, and a request that presents itself as that
// product is the one that is served and not investigated. Client-Metadata in
// particular is a header here and a message field in the body of the methods
// that take one — the service reads both.
func headers() map[string]string {
	metadata, _ := json.Marshal(clientMetadata())
	return map[string]string{
		"User-Agent":      "antigravity/" + clientVersion + " " + runtime.GOOS + "/" + runtime.GOARCH,
		"Client-Metadata": string(metadata),
	}
}

// clientMetadata names the surface making the request.
//
// The three values are enum names in the service's own proto, not free text:
// it answers 400 naming the field for anything else, which is how the platform
// list below was established rather than assumed.
func clientMetadata() map[string]any {
	return map[string]any{
		"ideType":    "ANTIGRAVITY",
		"platform":   platform(),
		"pluginType": "GEMINI",
	}
}

// platform maps this build onto the service's enum. Five values exist; a build
// for anything else says so rather than claiming to be a machine it is not,
// and the unspecified value is accepted.
func platform() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "DARWIN_ARM64"
	case "darwin/amd64":
		return "DARWIN_AMD64"
	case "linux/amd64":
		return "LINUX_AMD64"
	case "linux/arm64":
		return "LINUX_ARM64"
	case "windows/amd64":
		return "WINDOWS_AMD64"
	default:
		return "PLATFORM_UNSPECIFIED"
	}
}

// generationConfig renders the reasoning level, or nothing at all.
//
// Nothing at all is the honest answer more often than it looks. This vendor
// publishes effort as separate model ids — gemini-3.6-flash-low, -medium and
// -high are three entries in its catalogue, not one model with a dial — so the
// model somebody configured already carries an effort, and the budget below is
// a second, finer adjustment on top of it rather than the primary control.
//
// When the catalogue could not be read this sends no generationConfig at all.
// That is deliberate: a budget outside a model's own bounds is answered with a
// 400 and the whole turn is lost, and this API's bounds are per model — a
// budget of zero is rejected by a model whose floor is 32. Guessing is worse
// than declining to set it, because the default the service applies is right.
func generationConfig(limits modelInfo, known bool, level agentloop.ReasoningLevel) map[string]any {
	if !known || !limits.Thinking {
		return nil
	}
	thinking := map[string]any{"includeThoughts": true}
	if budget, ok := budgetFor(limits, level); ok {
		thinking["thinkingBudget"] = budget
	}
	return map[string]any{"thinkingConfig": thinking}
}

// budgetFor turns a reasoning level into a number inside this model's range.
//
// Minus one is the API's own "decide for yourself", which is what high means
// here. Medium is the model's advertised default, so the common case is the
// vendor's own tuning rather than this build's opinion of it. None cannot mean
// zero: a thinking model's floor is above it and zero is refused, so the
// floor is as close to off as this gets — which is the other half of why the
// model id, not this, is the real lever.
func budgetFor(limits modelInfo, level agentloop.ReasoningLevel) (int, bool) {
	floor := limits.MinBudget
	if floor < 1 {
		floor = 1
	}
	if limits.Budget < 0 {
		switch level {
		case agentloop.ReasoningHigh, agentloop.ReasoningMedium:
			return -1, true
		default:
			return floor, true
		}
	}
	if limits.Budget == 0 {
		return 0, false
	}
	var budget int
	switch level {
	case agentloop.ReasoningHigh:
		return -1, true
	case agentloop.ReasoningLow:
		budget = limits.Budget / 4
	case agentloop.ReasoningNone:
		budget = floor
	default:
		budget = limits.Budget
	}
	if budget < floor {
		budget = floor
	}
	if budget > budgetCeiling {
		budget = budgetCeiling
	}
	return budget, true
}

// toolDefs renders the toolset. The schema translation is google's, imported
// rather than copied: this endpoint takes the same Schema proto, and a second
// copy of that whitelist would diverge from the first the day a JSON Schema
// keyword is added to one of them.
func toolDefs(tools []toolexec.Spec) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		def := map[string]any{"name": t.Name, "description": t.Description}
		if t.InputSchema != nil {
			if params := google.Parameters(t.InputSchema); params != nil {
				def["parameters"] = params
			}
		}
		out = append(out, def)
	}
	return out
}

// contents renders the conversation.
//
// The shape is Gemini's — the assistant is "model", a tool result is a
// functionResponse on a user turn — with one difference that matters. This
// service gives every function call an id and takes it back on the response,
// where the public Gemini API has neither. Pairing by id instead of by
// position is what keeps two tools called in one turn from being answered in
// the wrong order.
func contents(messages []agentloop.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case agentloop.RoleTool:
			response := map[string]any{"name": m.Name, "response": responseValue(m.Result)}
			if m.CallID != "" {
				response["id"] = m.CallID
			}
			out = append(out, map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"functionResponse": response}},
			})

		case agentloop.RoleAssistant:
			var parts []map[string]any
			if m.Text != "" {
				parts = append(parts, map[string]any{"text": m.Text})
			}
			for _, c := range m.ToolCalls {
				call := map[string]any{
					"name": c.Name,
					"args": json.RawMessage(providers.JSONString(c.Input)),
				}
				if c.ID != "" {
					call["id"] = c.ID
				}
				part := map[string]any{"functionCall": call}
				// The signature rides beside the call and has to come back
				// unchanged, or the next request is refused for having lost
				// it. See agentloop.ToolCall.Signature.
				if c.Signature != "" {
					part["thoughtSignature"] = c.Signature
				}
				parts = append(parts, part)
			}
			if len(parts) == 0 {
				continue
			}
			out = append(out, map[string]any{"role": "model", "parts": parts})

		default:
			out = append(out, map[string]any{
				"role": "user", "parts": []map[string]any{{"text": m.Text}},
			})
		}
	}
	return out
}

// responseValue wraps a tool result, which this API requires to be an object
// even when the tool returned a list or a string.
func responseValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj
	}
	return map[string]any{"result": raw}
}

// envelopedResponse is the wrapper this service puts around a Gemini answer.
// The public API returns the candidates at the top level; this one nests them
// under "response" and adds its own trace id beside it.
type envelopedResponse struct {
	Response generated `json:"response"`
	TraceID  string    `json:"traceId"`
}

type generated struct {
	Candidates    []candidate `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
		ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

type candidate struct {
	Content struct {
		Parts []part `json:"parts"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type part struct {
	Text         string `json:"text"`
	Thought      bool   `json:"thought"`
	FunctionCall *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
		ID   string          `json:"id"`
	} `json:"functionCall"`
	ThoughtSignature string `json:"thoughtSignature"`
}

func translate(g generated, model string) agentloop.Response {
	out := agentloop.Response{
		Message: agentloop.Message{Role: agentloop.RoleAssistant},
		Model:   firstNonEmpty(g.ModelVersion, model),
		Usage: agentloop.Usage{
			Input:     g.UsageMetadata.PromptTokenCount,
			Output:    g.UsageMetadata.CandidatesTokenCount,
			Reasoning: g.UsageMetadata.ThoughtsTokenCount,
			Cached:    g.UsageMetadata.CachedContentTokenCount,
			Total:     g.UsageMetadata.TotalTokenCount,
		},
		StopReason: agentloop.StopEnd,
	}
	if len(g.Candidates) == 0 {
		return out
	}
	c := g.Candidates[0]

	var text, thinking strings.Builder
	var index int
	for _, p := range c.Content.Parts {
		switch {
		case p.FunctionCall != nil:
			index++
			id := p.FunctionCall.ID
			if id == "" {
				// The public Gemini API names no call, and a model served
				// through this one may not either. The loop pairs a result
				// with its call by id, so position becomes it.
				id = p.FunctionCall.Name + "-" + strconv.Itoa(index)
			}
			out.ToolCalls = append(out.ToolCalls, agentloop.ToolCall{
				ID:        id,
				Name:      p.FunctionCall.Name,
				Input:     providers.ToolArguments(string(p.FunctionCall.Args)),
				Signature: p.ThoughtSignature,
			})
		case p.Thought:
			thinking.WriteString(p.Text)
		default:
			text.WriteString(p.Text)
		}
	}
	out.Message.Text = text.String()
	out.Message.Reasoning = thinking.String()
	out.Message.ToolCalls = out.ToolCalls

	switch c.FinishReason {
	case "MAX_TOKENS":
		out.StopReason = agentloop.StopLength
	case "SAFETY", "PROHIBITED_CONTENT", "BLOCKLIST":
		out.StopReason = agentloop.StopFiltered
	}
	if len(out.ToolCalls) > 0 {
		out.StopReason = agentloop.StopToolCalls
	}
	return out
}

// stream reads the chunked answer. Every frame carries the whole envelope with
// its own slice of the parts, so the answer is their concatenation.
type stream struct {
	reader *providers.EventReader
	model  string
	guard  *guard
	whole  generated
}

func (s *stream) Recv() (agentloop.Chunk, error) {
	for {
		e, err := s.reader.Next()
		if err != nil {
			return agentloop.Chunk{}, err
		}
		var frame envelopedResponse
		if err := json.Unmarshal(e.Data, &frame); err != nil {
			continue
		}
		s.merge(frame.Response)
		if len(frame.Response.Candidates) == 0 {
			continue
		}
		var text, thinking strings.Builder
		for _, p := range frame.Response.Candidates[0].Content.Parts {
			if p.FunctionCall != nil {
				continue
			}
			if p.Thought {
				thinking.WriteString(p.Text)
			} else {
				text.WriteString(p.Text)
			}
		}
		if text.Len() > 0 || thinking.Len() > 0 {
			return agentloop.Chunk{Text: text.String(), Reasoning: thinking.String()}, nil
		}
	}
}

func (s *stream) merge(frame generated) {
	if frame.ModelVersion != "" {
		s.whole.ModelVersion = frame.ModelVersion
	}
	if frame.UsageMetadata.TotalTokenCount > 0 {
		s.whole.UsageMetadata = frame.UsageMetadata
	}
	if len(frame.Candidates) == 0 {
		return
	}
	if len(s.whole.Candidates) == 0 {
		s.whole.Candidates = make([]candidate, 1)
	}
	s.whole.Candidates[0].Content.Parts = append(
		s.whole.Candidates[0].Content.Parts, frame.Candidates[0].Content.Parts...)
	if frame.Candidates[0].FinishReason != "" {
		s.whole.Candidates[0].FinishReason = frame.Candidates[0].FinishReason
	}
}

func (s *stream) Response() agentloop.Response { return translate(s.whole, s.model) }

// Close releases the connection and tells the guard the call ended cleanly, so
// a stream that ran to completion clears a back-off the same way a
// non-streaming call does.
func (s *stream) Close() error {
	err := s.reader.Close()
	if s.guard != nil {
		s.guard.observe(nil)
	}
	return err
}

// The registry resolves optional capabilities by type assertion, so a
// signature that drifts fails here rather than at run time.
var (
	_ agentloop.LLMProvider = (*Provider)(nil)
	_ providers.Lister      = (*Provider)(nil)
	_ agentloop.Stream      = (*stream)(nil)
)
