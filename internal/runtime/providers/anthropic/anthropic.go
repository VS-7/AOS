// Package anthropic speaks the Messages API.
package anthropic

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

// DefaultBaseURL is the public endpoint.
const DefaultBaseURL = "https://api.anthropic.com/v1"

// APIVersion is the dated contract this adapter was written against. It is a
// constant rather than "latest" because an API that changes under a running
// daemon is a failure nobody can reproduce.
const APIVersion = "2023-06-01"

// DefaultMaxTokens is required by this API and has no server-side default.
const DefaultMaxTokens = 8192

// thinkingBudget maps the reasoning levels onto the token budget the *older*
// models take. It is no longer the default path: on every model released from
// Claude 4.6 onward `thinking: {type: "enabled", budget_tokens: N}` is
// rejected outright with a 400, and two of the three Anthropic models this
// build prices (claude-opus-5, claude-sonnet-5) are in that group — so an
// agent on either of them, with reasoning anything but "none", could not take
// a single turn. See takesAdaptiveThinking.
var thinkingBudget = map[agentloop.ReasoningLevel]int{
	agentloop.ReasoningLow:    2048,
	agentloop.ReasoningMedium: 8192,
	agentloop.ReasoningHigh:   24576,
}

// effortFor maps the reasoning levels onto the effort word the current models
// take. The three names line up, which is not a coincidence: the provider's
// own scale starts at the same three and adds two above them.
var effortFor = map[agentloop.ReasoningLevel]string{
	agentloop.ReasoningLow:    "low",
	agentloop.ReasoningMedium: "medium",
	agentloop.ReasoningHigh:   "high",
}

// adaptiveFamilies are the model families that take adaptive thinking and an
// effort word rather than a token budget.
//
// Matched on a substring of the model id rather than an exact list, so a
// dated snapshot ("claude-opus-4-6@20260101", "anthropic.claude-sonnet-5")
// resolves the same way as the bare alias, and a model released after this
// build still works if it carries a family name this recognises. An id
// nothing here matches falls back to the older budget form, which is the safe
// direction: an unknown id is more likely to be an older model than a newer
// one, and a budget on a model that wanted effort fails loudly at the first
// call rather than silently costing more.
var adaptiveFamilies = []string{
	"claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8", "claude-opus-5",
	"claude-sonnet-4-6", "claude-sonnet-5",
	"claude-fable-5", "claude-mythos-5",
}

func takesAdaptiveThinking(model string) bool {
	id := strings.ToLower(model)
	for _, family := range adaptiveFamilies {
		if strings.Contains(id, family) {
			return true
		}
	}
	return false
}

func init() {
	providers.Register("anthropic", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return New(cfg), nil
	})
}

// Provider is the adapter.
type Provider struct{ client *providers.Client }

// New builds it.
func New(cfg providers.Config) *Provider {
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	c := providers.NewClient("anthropic", base, map[string]string{
		"anthropic-version": APIVersion,
	})
	c.Auth = func(_ context.Context, r *http.Request) error {
		if cfg.APIKey != "" {
			r.Header.Set("x-api-key", cfg.APIKey)
		}
		return nil
	}
	return &Provider{client: c}
}

// Name reports the provider id.
func (p *Provider) Name() string { return "anthropic" }

// Generate makes one call.
func (p *Provider) Generate(ctx context.Context, req agentloop.Request) (agentloop.Response, error) {
	var out message
	if err := p.client.PostJSON(ctx, "/messages", body(req, false), &out); err != nil {
		return agentloop.Response{}, err
	}
	return translate(out, req.Model), nil
}

// Stream makes the same call with the event stream on.
func (p *Provider) Stream(ctx context.Context, req agentloop.Request) (agentloop.Stream, error) {
	reader, err := p.client.PostSSE(ctx, "/messages", body(req, true))
	if err != nil {
		return nil, err
	}
	return &stream{reader: reader, model: req.Model}, nil
}

func body(req agentloop.Request, stream bool) map[string]any {
	out := map[string]any{
		"model":      req.Model,
		"max_tokens": DefaultMaxTokens,
		"messages":   messages(req.Messages),
	}
	if req.Instructions != "" {
		out["system"] = req.Instructions
	}
	if len(req.Tools) > 0 {
		out["tools"] = toolDefs(req.Tools)
	}
	if effort, ok := effortFor[req.Reasoning]; ok {
		if takesAdaptiveThinking(req.Model) {
			// display: summarized, because this system shows the model's
			// reasoning — the default on these models is "omitted", which
			// streams thinking blocks with empty text and leaves the
			// reasoning pane blank while the turn appears to hang.
			out["thinking"] = map[string]any{"type": "adaptive", "display": "summarized"}
			out["output_config"] = map[string]any{"effort": effort}
		} else if budget, ok := thinkingBudget[req.Reasoning]; ok {
			out["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
			// The budget has to fit inside the answer, and a budget larger
			// than the ceiling is rejected by the provider rather than
			// clamped.
			if budget >= DefaultMaxTokens {
				out["max_tokens"] = budget + 4096
			}
		}
	}
	if stream {
		out["stream"] = true
	}
	for k, v := range req.Options {
		out[k] = v
	}
	return out
}

func toolDefs(tools []toolexec.Spec) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		def := map[string]any{"name": t.Name, "description": t.Description}
		if t.InputSchema != nil {
			def["input_schema"] = t.InputSchema
		}
		out = append(out, def)
	}
	return out
}

// messages turns the conversation into this API's shape.
//
// Two things differ from every other provider. A tool result is a user message
// rather than a role of its own, and consecutive results have to be collapsed
// into one — sending them separately is rejected.
func messages(in []agentloop.Message) []map[string]any {
	var out []map[string]any
	var pending []map[string]any

	flush := func() {
		if len(pending) == 0 {
			return
		}
		out = append(out, map[string]any{"role": "user", "content": pending})
		pending = nil
	}

	for _, m := range in {
		switch m.Role {
		case agentloop.RoleTool:
			pending = append(pending, map[string]any{
				"type": "tool_result", "tool_use_id": m.CallID,
				"content": providers.JSONString(m.Result),
			})

		case agentloop.RoleAssistant:
			flush()
			var content []map[string]any
			// The signed thinking block goes first, and it goes back exactly
			// as it arrived. Dropping it is what broke every tool-using turn
			// with reasoning on: the provider requires the thinking block of
			// the assistant turn that asked for the tool, and refuses the
			// request without it. An empty thinking text is normal on the
			// current models (display defaults to omitted) and is still
			// valid — what must not change is the signature.
			if m.Encrypted != "" {
				content = append(content, map[string]any{
					"type": "thinking", "thinking": m.Reasoning, "signature": m.Encrypted,
				})
			}
			if m.Text != "" {
				content = append(content, map[string]any{"type": "text", "text": m.Text})
			}
			for _, c := range m.ToolCalls {
				content = append(content, map[string]any{
					"type": "tool_use", "id": c.ID, "name": c.Name,
					"input": json.RawMessage(providers.JSONString(c.Input)),
				})
			}
			if len(content) == 0 {
				continue
			}
			out = append(out, map[string]any{"role": "assistant", "content": content})

		default:
			flush()
			out = append(out, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "text", "text": m.Text}},
			})
		}
	}
	flush()
	return out
}

type message struct {
	Content []struct {
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		Thinking  string          `json:"thinking"`
		Signature string          `json:"signature"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Model      string `json:"model"`
	Usage      struct {
		InputTokens      int `json:"input_tokens"`
		OutputTokens     int `json:"output_tokens"`
		CacheReadTokens  int `json:"cache_read_input_tokens"`
		CacheWriteTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func translate(m message, model string) agentloop.Response {
	msg := agentloop.Message{Role: agentloop.RoleAssistant}
	var calls []agentloop.ToolCall
	var text, thinking strings.Builder
	// The signature is what makes a thinking block replayable. A turn that
	// used a tool has to come back with its thinking block attached,
	// unchanged and signed, or the provider refuses the next call — which is
	// every tool-using turn with reasoning on. Kept in Encrypted, the field
	// that exists for exactly this: opaque, carried between turns, never
	// read (see agentloop.Message).
	var signature string

	for _, c := range m.Content {
		switch c.Type {
		case "text":
			text.WriteString(c.Text)
		case "thinking":
			thinking.WriteString(c.Thinking)
			if c.Signature != "" {
				signature = c.Signature
			}
		case "tool_use":
			calls = append(calls, agentloop.ToolCall{ID: c.ID, Name: c.Name, Input: c.Input})
		}
	}
	msg.Text = text.String()
	msg.Reasoning = thinking.String()
	msg.Encrypted = signature
	msg.ToolCalls = calls

	stop := agentloop.StopEnd
	switch m.StopReason {
	case "tool_use":
		stop = agentloop.StopToolCalls
	case "max_tokens":
		stop = agentloop.StopLength
	case "refusal":
		stop = agentloop.StopFiltered
	}

	name := m.Model
	if name == "" {
		name = model
	}
	return agentloop.Response{
		Message: msg, ToolCalls: calls, StopReason: stop, Model: name,
		Usage: agentloop.Usage{
			Input:  m.Usage.InputTokens,
			Output: m.Usage.OutputTokens,
			Cached: m.Usage.CacheReadTokens,
			Total:  m.Usage.InputTokens + m.Usage.OutputTokens,
		},
	}
}

// stream assembles the answer from the event stream.
//
// This API streams a skeleton and then fills it in, so the adapter rebuilds the
// message as it goes rather than waiting for a final frame that carries it all.
type stream struct {
	reader *providers.EventReader
	model  string

	final    message
	building map[int]*block
}

type block struct {
	kind      string
	id        string
	name      string
	signature string
	text      strings.Builder
	input     strings.Builder
}

func (s *stream) Recv() (agentloop.Chunk, error) {
	if s.building == nil {
		s.building = map[int]*block{}
	}
	for {
		e, err := s.reader.Next()
		if err != nil {
			return agentloop.Chunk{}, err
		}
		var frame struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Block struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Text string `json:"text"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				Signature   string `json:"signature"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Message struct {
				Model string `json:"model"`
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(e.Data, &frame); err != nil {
			continue
		}

		switch frame.Type {
		case "message_start":
			s.final.Model = frame.Message.Model
			s.final.Usage.InputTokens = frame.Message.Usage.InputTokens

		case "content_block_start":
			b := &block{kind: frame.Block.Type, id: frame.Block.ID, name: frame.Block.Name}
			b.text.WriteString(frame.Block.Text)
			s.building[frame.Index] = b

		case "content_block_delta":
			b := s.building[frame.Index]
			if b == nil {
				continue
			}
			switch frame.Delta.Type {
			case "text_delta":
				b.text.WriteString(frame.Delta.Text)
				return agentloop.Chunk{Text: frame.Delta.Text}, nil
			case "thinking_delta":
				b.text.WriteString(frame.Delta.Thinking)
				return agentloop.Chunk{Reasoning: frame.Delta.Thinking}, nil
			case "signature_delta":
				// The signature arrives as its own delta at the end of a
				// thinking block, not on the block that opened it. Without
				// this the block was assembled unsigned and could not be
				// replayed — see translate.
				b.signature += frame.Delta.Signature
				continue
			case "input_json_delta":
				b.input.WriteString(frame.Delta.PartialJSON)
			}

		case "message_delta":
			s.final.StopReason = frame.Delta.StopReason
			s.final.Usage.OutputTokens = frame.Usage.OutputTokens

		case "message_stop":
			s.assemble()
			return agentloop.Chunk{}, io.EOF
		}
	}
}

func (s *stream) assemble() {
	for i := 0; i < len(s.building); i++ {
		b := s.building[i]
		if b == nil {
			continue
		}
		var entry struct {
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			Thinking  string          `json:"thinking"`
			Signature string          `json:"signature"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
		}
		entry.Type = b.kind
		switch b.kind {
		case "text":
			entry.Text = b.text.String()
		case "thinking":
			entry.Thinking = b.text.String()
			entry.Signature = b.signature
		case "tool_use":
			entry.ID, entry.Name = b.id, b.name
			entry.Input = providers.ToolArguments(b.input.String())
		}
		s.final.Content = append(s.final.Content, entry)
	}
}

func (s *stream) Response() agentloop.Response { return translate(s.final, s.model) }
func (s *stream) Close() error                 { return s.reader.Close() }
