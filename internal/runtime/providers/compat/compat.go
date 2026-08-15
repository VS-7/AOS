// Package compat speaks Chat Completions, which three of the eight providers
// serve behind their own base URL.
//
// One adapter for openrouter, crof and opencode. They differ in the endpoint
// they answer on and in nothing this code has to know about, which is the
// argument for the shape: a fourth compatible provider is a registration, not
// a package.
package compat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// The endpoints of the compatible providers.
const (
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	CrofBaseURL       = "https://api.crof.ai/v1"
	OpenCodeBaseURL   = "https://opencode.ai/zen/v1"
	// OpenCodeFreeBaseURL is where this provider routes the models whose name
	// ends in -free, a routing rule the original carries and this reproduces.
	OpenCodeFreeBaseURL = "https://opencode.ai/zen/free/v1"
)

func init() {
	providers.Register("openrouter", registerAs("openrouter", OpenRouterBaseURL))
	providers.Register("crof", registerAs("crof", CrofBaseURL))
	providers.Register("opencode", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		p := New("opencode", cfg, OpenCodeBaseURL)
		// The routing rule only applies to the provider's own endpoints. An
		// installation that overrode the base URL pointed at something else,
		// and sending half its traffic elsewhere would be a surprise.
		if cfg.BaseURL == "" {
			p.freeBaseURL = OpenCodeFreeBaseURL
		}
		return p, nil
	})
}

func registerAs(name, base string) providers.Factory {
	return func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return New(name, cfg, base), nil
	}
}

// Provider is the adapter.
type Provider struct {
	name        string
	client      *providers.Client
	free        *providers.Client
	freeBaseURL string
}

// New builds it against a default endpoint the configuration may override.
func New(name string, cfg providers.Config, defaultBase string) *Provider {
	base := cfg.BaseURL
	if base == "" {
		base = defaultBase
	}
	c := providers.NewClient(name, base, nil)
	c.Auth = func(_ context.Context, r *http.Request) error {
		if cfg.APIKey != "" {
			r.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		return nil
	}
	return &Provider{name: name, client: c}
}

// Name reports the provider id.
func (p *Provider) Name() string { return p.name }

// clientFor routes a model whose name ends in -free to the alternative
// endpoint, which is the one routing rule any of these three has.
func (p *Provider) clientFor(model string) *providers.Client {
	if p.freeBaseURL == "" || !strings.HasSuffix(model, "-free") {
		return p.client
	}
	if p.free == nil {
		free := *p.client
		free.BaseURL = strings.TrimRight(p.freeBaseURL, "/")
		p.free = &free
	}
	return p.free
}

// Generate makes one call.
func (p *Provider) Generate(ctx context.Context, req agentloop.Request) (agentloop.Response, error) {
	var out completion
	if err := p.clientFor(req.Model).PostJSON(ctx, "/chat/completions", body(req, false), &out); err != nil {
		return agentloop.Response{}, err
	}
	return translate(out, req.Model), nil
}

// Stream makes the same call with the event stream on.
func (p *Provider) Stream(ctx context.Context, req agentloop.Request) (agentloop.Stream, error) {
	reader, err := p.clientFor(req.Model).PostSSE(ctx, "/chat/completions", body(req, true))
	if err != nil {
		return nil, err
	}
	return &stream{reader: reader, model: req.Model}, nil
}

func body(req agentloop.Request, stream bool) map[string]any {
	out := map[string]any{
		"model":    req.Model,
		"messages": messages(req),
	}
	if len(req.Tools) > 0 {
		out["tools"] = toolDefs(req.Tools)
	}
	if req.Reasoning != "" && req.Reasoning != agentloop.ReasoningNone {
		out["reasoning_effort"] = string(req.Reasoning)
	}
	if stream {
		out["stream"] = true
		out["stream_options"] = map[string]any{"include_usage": true}
	}
	for k, v := range req.Options {
		out[k] = v
	}
	return out
}

func toolDefs(tools []toolexec.Spec) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		fn := map[string]any{"name": t.Name, "description": t.Description}
		if t.InputSchema != nil {
			fn["parameters"] = t.InputSchema
		}
		out = append(out, map[string]any{"type": "function", "function": fn})
	}
	return out
}

// messages renders the conversation, with the assembled context document as
// the system message.
func messages(req agentloop.Request) []map[string]any {
	out := make([]map[string]any, 0, len(req.Messages)+1)
	if req.Instructions != "" {
		out = append(out, map[string]any{"role": "system", "content": req.Instructions})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case agentloop.RoleTool:
			out = append(out, map[string]any{
				"role": "tool", "tool_call_id": m.CallID,
				"content": providers.JSONString(m.Result),
			})

		case agentloop.RoleAssistant:
			entry := map[string]any{"role": "assistant", "content": m.Text}
			if len(m.ToolCalls) > 0 {
				calls := make([]map[string]any, 0, len(m.ToolCalls))
				for _, c := range m.ToolCalls {
					calls = append(calls, map[string]any{
						"id": c.ID, "type": "function",
						"function": map[string]any{
							"name": c.Name, "arguments": providers.JSONString(c.Input),
						},
					})
				}
				entry["tool_calls"] = calls
			}
			out = append(out, entry)

		default:
			out = append(out, map[string]any{"role": "user", "content": m.Text})
		}
	}
	return out
}

type completion struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		PromptDetails    struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

func translate(c completion, model string) agentloop.Response {
	out := agentloop.Response{
		Message: agentloop.Message{Role: agentloop.RoleAssistant},
		Model:   firstNonEmpty(c.Model, model),
		Usage: agentloop.Usage{
			Input:     c.Usage.PromptTokens,
			Output:    c.Usage.CompletionTokens,
			Reasoning: c.Usage.CompletionDetails.ReasoningTokens,
			Cached:    c.Usage.PromptDetails.CachedTokens,
			Total:     c.Usage.TotalTokens,
		},
		StopReason: agentloop.StopEnd,
	}
	if len(c.Choices) == 0 {
		return out
	}
	choice := c.Choices[0]
	out.Message.Text = choice.Message.Content
	out.Message.Reasoning = choice.Message.Reasoning

	for _, call := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, agentloop.ToolCall{
			ID: call.ID, Name: call.Function.Name,
			Input: providers.ToolArguments(call.Function.Arguments),
		})
	}
	out.Message.ToolCalls = out.ToolCalls

	switch choice.FinishReason {
	case "tool_calls":
		out.StopReason = agentloop.StopToolCalls
	case "length":
		out.StopReason = agentloop.StopLength
	case "content_filter":
		out.StopReason = agentloop.StopFiltered
	}
	if len(out.ToolCalls) > 0 {
		out.StopReason = agentloop.StopToolCalls
	}
	return out
}

// stream assembles the answer from the deltas.
//
// Tool calls arrive in pieces addressed by index, and the name comes in the
// first piece while the arguments dribble in over the rest.
type stream struct {
	reader *providers.EventReader
	model  string

	text      strings.Builder
	reasoning strings.Builder
	calls     map[int]*partialCall
	order     []int
	finish    string
	usage     agentloop.Usage
	modelName string
}

type partialCall struct {
	id   string
	name string
	args strings.Builder
}

func (s *stream) Recv() (agentloop.Chunk, error) {
	if s.calls == nil {
		s.calls = map[int]*partialCall{}
	}
	for {
		e, err := s.reader.Next()
		if err != nil {
			return agentloop.Chunk{}, err
		}
		if strings.TrimSpace(string(e.Data)) == "[DONE]" {
			return agentloop.Chunk{}, io.EOF
		}
		var frame struct {
			Model   string `json:"model"`
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Delta        struct {
					Content   string `json:"content"`
					Reasoning string `json:"reasoning"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(e.Data, &frame); err != nil {
			continue
		}
		if frame.Model != "" {
			s.modelName = frame.Model
		}
		if frame.Usage != nil {
			s.usage = agentloop.Usage{
				Input: frame.Usage.PromptTokens, Output: frame.Usage.CompletionTokens,
				Total: frame.Usage.TotalTokens,
			}
		}
		if len(frame.Choices) == 0 {
			continue
		}
		choice := frame.Choices[0]
		if choice.FinishReason != "" {
			s.finish = choice.FinishReason
		}
		for _, c := range choice.Delta.ToolCalls {
			pc := s.calls[c.Index]
			if pc == nil {
				pc = &partialCall{}
				s.calls[c.Index] = pc
				s.order = append(s.order, c.Index)
			}
			if c.ID != "" {
				pc.id = c.ID
			}
			if c.Function.Name != "" {
				pc.name = c.Function.Name
			}
			pc.args.WriteString(c.Function.Arguments)
		}
		switch {
		case choice.Delta.Content != "":
			s.text.WriteString(choice.Delta.Content)
			return agentloop.Chunk{Text: choice.Delta.Content}, nil
		case choice.Delta.Reasoning != "":
			s.reasoning.WriteString(choice.Delta.Reasoning)
			return agentloop.Chunk{Reasoning: choice.Delta.Reasoning}, nil
		}
	}
}

func (s *stream) Response() agentloop.Response {
	out := agentloop.Response{
		Message: agentloop.Message{
			Role: agentloop.RoleAssistant,
			Text: s.text.String(), Reasoning: s.reasoning.String(),
		},
		Model: firstNonEmpty(s.modelName, s.model),
		Usage: s.usage, StopReason: agentloop.StopEnd,
	}
	for _, i := range s.order {
		pc := s.calls[i]
		out.ToolCalls = append(out.ToolCalls, agentloop.ToolCall{
			ID: pc.id, Name: pc.name, Input: providers.ToolArguments(pc.args.String()),
		})
	}
	out.Message.ToolCalls = out.ToolCalls

	switch s.finish {
	case "length":
		out.StopReason = agentloop.StopLength
	case "content_filter":
		out.StopReason = agentloop.StopFiltered
	}
	if len(out.ToolCalls) > 0 {
		out.StopReason = agentloop.StopToolCalls
	}
	return out
}

func (s *stream) Close() error { return s.reader.Close() }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// EndpointFor reports where a model's call would go, so the routing rule can be
// asserted on without making one.
func (p *Provider) EndpointFor(model string) string { return p.clientFor(model).BaseURL }
