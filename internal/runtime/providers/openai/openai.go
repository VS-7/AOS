// Package openai speaks the Responses API.
//
// It is the Responses API rather than Chat Completions for one reason that is
// a product decision rather than a technical one: the original sets
// store: false and include: ["reasoning.encrypted_content"], so conversations
// are not retained by the provider and the model's reasoning travels between
// turns encrypted. Chat Completions cannot express that.
package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/oauthfile"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// DefaultBaseURL is the public endpoint.
const DefaultBaseURL = "https://api.openai.com/v1"

// codexBaseURL is where a ChatGPT subscription's Codex access lives.
//
// It is not the public API with a different credential, which is what this
// adapter assumed: the token the Codex CLI writes carries no `api.*` scopes
// at all, so every call to api.openai.com came back 401 "Missing scopes:
// api.responses.write" and the provider could never answer. Same Responses
// protocol, different host and a few headers of its own.
const codexBaseURL = "https://chatgpt.com/backend-api/codex"

func init() {
	providers.Register("openai", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return New("openai", cfg, nil), nil
	})
	// codex is the same API reached with the OAuth token the ChatGPT desktop
	// tooling wrote on this machine. It is a documented way to use a
	// subscription somebody already pays for rather than paying per token.
	providers.Register("codex", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return newCodex(cfg), nil
	})
}

// newCodex builds the ChatGPT-subscription variant.
//
// The endpoint identifies the caller by account as well as by token — a
// subscription request has to say which subscription — so the account id
// travels from the same credential file as the token, per request, rather
// than being configured.
func newCodex(cfg providers.Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = codexBaseURL
	}
	tokens := oauthfile.Codex(cfg.Home)
	p := New("codex", cfg, tokens)
	p.client.Auth = func(ctx context.Context, r *http.Request) error {
		token, err := tokens.Token(ctx)
		if err != nil {
			return err
		}
		r.Header.Set("Authorization", "Bearer "+token)
		// Identifies the surface being spoken to. Without them the host
		// answers as though this were a browser session, not Codex.
		r.Header.Set("OpenAI-Beta", "responses=experimental")
		r.Header.Set("originator", "codex_cli_rs")
		account, err := tokens.Account(ctx)
		if err != nil {
			return err
		}
		if account != "" {
			r.Header.Set("chatgpt-account-id", account)
		}
		return nil
	}
	p.codex = true
	return p
}

// Provider is the adapter.
type Provider struct {
	name   string
	client *providers.Client

	// codex marks the ChatGPT-subscription variant. The two speak the same
	// Responses protocol but publish their catalogues on different endpoints
	// in different shapes, which is the one place the distinction matters
	// after construction.
	codex bool
}

// New builds it. tokens, when given, replaces the API key with a token read
// from a credential file another tool owns.
func New(name string, cfg providers.Config, tokens *oauthfile.Store) *Provider {
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	c := providers.NewClient(name, base, nil)
	c.Auth = func(ctx context.Context, r *http.Request) error {
		token := cfg.APIKey
		if tokens != nil {
			t, err := tokens.Token(ctx)
			if err != nil {
				return err
			}
			token = t
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		return nil
	}
	return &Provider{name: name, client: c}
}

// Name reports the provider id.
func (p *Provider) Name() string { return p.name }

// Generate makes one call.
func (p *Provider) Generate(ctx context.Context, req agentloop.Request) (agentloop.Response, error) {
	var out response
	if err := p.client.PostJSON(ctx, "/responses", p.body(req, false), &out); err != nil {
		return agentloop.Response{}, err
	}
	return translate(out, req.Model), nil
}

// Stream makes the same call with the event stream on.
func (p *Provider) Stream(ctx context.Context, req agentloop.Request) (agentloop.Stream, error) {
	reader, err := p.client.PostSSE(ctx, "/responses", p.body(req, true))
	if err != nil {
		return nil, err
	}
	return &stream{reader: reader, model: req.Model}, nil
}

// body builds the request.
func (p *Provider) body(req agentloop.Request, stream bool) map[string]any {
	body := map[string]any{
		"model": req.Model,
		"input": inputItems(req.Messages),

		// The privacy stance, inherited: nothing is retained on the provider,
		// and the reasoning that travels between turns is encrypted.
		"store":   false,
		"include": []string{"reasoning.encrypted_content"},
	}
	if req.Instructions != "" {
		body["instructions"] = req.Instructions
	}
	if len(req.Tools) > 0 {
		body["tools"] = toolDefs(req.Tools)
	}
	if req.Reasoning != "" && req.Reasoning != agentloop.ReasoningNone {
		body["reasoning"] = map[string]any{"effort": string(req.Reasoning)}
	}
	if stream {
		body["stream"] = true
	}
	for k, v := range req.Options {
		body[k] = v
	}
	return body
}

func toolDefs(tools []toolexec.Spec) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		def := map[string]any{
			"type":        "function",
			"name":        t.Name,
			"description": t.Description,
		}
		if t.InputSchema != nil {
			def["parameters"] = t.InputSchema
		}
		out = append(out, def)
	}
	return out
}

// inputItems turns the conversation into the item list the Responses API takes.
func inputItems(messages []agentloop.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case agentloop.RoleTool:
			out = append(out, map[string]any{
				"type":    "function_call_output",
				"call_id": m.CallID,
				"output":  providers.JSONString(m.Result),
			})

		case agentloop.RoleAssistant:
			// The encrypted reasoning is handed back so the model can pick up
			// its own thread. It is opaque here and never read.
			if m.Encrypted != "" {
				out = append(out, map[string]any{
					"type": "reasoning", "encrypted_content": m.Encrypted, "summary": []any{},
				})
			}
			if m.Text != "" {
				out = append(out, map[string]any{
					"role": "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": m.Text},
					},
				})
			}
			for _, c := range m.ToolCalls {
				out = append(out, map[string]any{
					"type": "function_call", "call_id": c.ID,
					"name": c.Name, "arguments": providers.JSONString(c.Input),
				})
			}

		default:
			out = append(out, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": m.Text},
				},
			})
		}
	}
	return out
}

// response is the part of the answer this adapter reads.
type response struct {
	Output []outputItem `json:"output"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
		InputDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		OutputDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
	Status string `json:"status"`
	Model  string `json:"model"`
}

type outputItem struct {
	Type      string `json:"type"`
	Role      string `json:"role"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Encrypted string `json:"encrypted_content"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Summary []struct {
		Text string `json:"text"`
	} `json:"summary"`
}

func translate(r response, model string) agentloop.Response {
	msg := agentloop.Message{Role: agentloop.RoleAssistant}
	var calls []agentloop.ToolCall
	var text, reasoning strings.Builder

	for _, item := range r.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					text.WriteString(c.Text)
				}
			}
		case "reasoning":
			msg.Encrypted = item.Encrypted
			for _, s := range item.Summary {
				reasoning.WriteString(s.Text)
			}
		case "function_call":
			calls = append(calls, agentloop.ToolCall{
				ID: item.CallID, Name: item.Name,
				Input: providers.ToolArguments(item.Arguments),
			})
		}
	}

	msg.Text = text.String()
	msg.Reasoning = reasoning.String()
	msg.ToolCalls = calls

	answer := agentloop.Response{
		Message: msg, ToolCalls: calls, StopReason: stopOf(r.Status, calls),
		Model: firstNonEmpty(r.Model, model),
	}
	answer.Usage = agentloop.Usage{
		Input:     r.Usage.InputTokens,
		Output:    r.Usage.OutputTokens,
		Reasoning: r.Usage.OutputDetails.ReasoningTokens,
		Cached:    r.Usage.InputDetails.CachedTokens,
		Total:     r.Usage.TotalTokens,
	}
	return answer
}

func stopOf(status string, calls []agentloop.ToolCall) string {
	if len(calls) > 0 {
		return agentloop.StopToolCalls
	}
	if status == "incomplete" {
		return agentloop.StopLength
	}
	return agentloop.StopEnd
}

// stream reads the event stream, forwarding text as it arrives and assembling
// the whole answer from the completion event.
type stream struct {
	reader *providers.EventReader
	model  string
	final  agentloop.Response

	// items is the answer assembled from the per-item events.
	//
	// The public API repeats the whole answer in `response.completed`, so
	// reading it there was enough. The Codex backend does not: its completed
	// event carries `output: []` and the content only ever arrives through
	// `response.output_item.done`. Trusting the completed event alone made
	// every Codex answer parse as empty — and an empty answer is not
	// harmless, because agentloop.Result falls back to the last assistant
	// message already in the transcript, so the turn silently "replied" with
	// the *previous* turn's answer.
	items []outputItem
}

func (s *stream) Recv() (agentloop.Chunk, error) {
	for {
		e, err := s.reader.Next()
		if err != nil {
			return agentloop.Chunk{}, err
		}
		var frame struct {
			Type     string     `json:"type"`
			Delta    string     `json:"delta"`
			Item     outputItem `json:"item"`
			Response response   `json:"response"`
		}
		if err := json.Unmarshal(e.Data, &frame); err != nil {
			continue // a frame this build does not understand is not a reason to stop
		}
		switch frame.Type {
		case "response.output_item.done":
			s.items = append(s.items, frame.Item)
		case "response.output_text.delta":
			return agentloop.Chunk{Text: frame.Delta}, nil
		case "response.reasoning_summary_text.delta":
			return agentloop.Chunk{Reasoning: frame.Delta}, nil
		case "response.completed", "response.incomplete":
			// Prefer what the completed event holds; fall back to the items
			// when it holds nothing, which is the Codex backend's shape.
			answer := frame.Response
			if len(answer.Output) == 0 {
				answer.Output = s.items
			}
			s.final = translate(answer, s.model)
			return agentloop.Chunk{}, io.EOF
		}
	}
}

func (s *stream) Response() agentloop.Response { return s.final }
func (s *stream) Close() error                 { return s.reader.Close() }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
