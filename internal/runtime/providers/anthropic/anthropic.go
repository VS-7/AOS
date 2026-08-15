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

// thinkingBudget maps the reasoning levels onto the token budget this provider
// takes instead of an effort word.
var thinkingBudget = map[agentloop.ReasoningLevel]int{
	agentloop.ReasoningLow:    2048,
	agentloop.ReasoningMedium: 8192,
	agentloop.ReasoningHigh:   24576,
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
	if budget, ok := thinkingBudget[req.Reasoning]; ok {
		out["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
		// The budget has to fit inside the answer, and a budget larger than
		// the ceiling is rejected by the provider rather than clamped.
		if budget >= DefaultMaxTokens {
			out["max_tokens"] = budget + 4096
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
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking string          `json:"thinking"`
		ID       string          `json:"id"`
		Name     string          `json:"name"`
		Input    json.RawMessage `json:"input"`
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

	for _, c := range m.Content {
		switch c.Type {
		case "text":
			text.WriteString(c.Text)
		case "thinking":
			thinking.WriteString(c.Thinking)
		case "tool_use":
			calls = append(calls, agentloop.ToolCall{ID: c.ID, Name: c.Name, Input: c.Input})
		}
	}
	msg.Text = text.String()
	msg.Reasoning = thinking.String()
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
	kind  string
	id    string
	name  string
	text  strings.Builder
	input strings.Builder
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
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
			ID       string          `json:"id"`
			Name     string          `json:"name"`
			Input    json.RawMessage `json:"input"`
		}
		entry.Type = b.kind
		switch b.kind {
		case "text":
			entry.Text = b.text.String()
		case "thinking":
			entry.Thinking = b.text.String()
		case "tool_use":
			entry.ID, entry.Name = b.id, b.name
			entry.Input = providers.ToolArguments(b.input.String())
		}
		s.final.Content = append(s.final.Content, entry)
	}
}

func (s *stream) Response() agentloop.Response { return translate(s.final, s.model) }
func (s *stream) Close() error                 { return s.reader.Close() }
