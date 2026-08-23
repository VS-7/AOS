// Package google speaks the Gemini generateContent API.
package google

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/oauthfile"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// DefaultBaseURL is the public endpoint.
const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// thinkingBudget maps the reasoning levels onto the token budget this provider
// takes. Minus one asks the model to decide for itself, which is what "high"
// means here.
var thinkingBudget = map[agentloop.ReasoningLevel]int{
	agentloop.ReasoningNone:   0,
	agentloop.ReasoningLow:    2048,
	agentloop.ReasoningMedium: 8192,
	agentloop.ReasoningHigh:   -1,
}

func init() {
	providers.Register("google", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return New("google", cfg, nil), nil
	})
	// gemini-cli is the same API reached with the OAuth credential the Gemini
	// CLI wrote, which is how somebody uses an allowance they already have.
	providers.Register("gemini-cli", func(cfg providers.Config) (agentloop.LLMProvider, error) {
		return New("gemini-cli", cfg, oauthfile.GeminiCLI(cfg.Home)), nil
	})
}

// Provider is the adapter.
type Provider struct {
	name   string
	client *providers.Client
}

// New builds it.
func New(name string, cfg providers.Config, tokens *oauthfile.Store) *Provider {
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	c := providers.NewClient(name, base, nil)
	c.Auth = func(ctx context.Context, r *http.Request) error {
		if tokens != nil {
			token, err := tokens.Token(ctx)
			if err != nil {
				return err
			}
			r.Header.Set("Authorization", "Bearer "+token)
			return nil
		}
		if cfg.APIKey != "" {
			// The key goes in a header rather than the query string, so it
			// does not end up in an access log somebody forwards.
			r.Header.Set("x-goog-api-key", cfg.APIKey)
		}
		return nil
	}
	return &Provider{name: name, client: c}
}

// Name reports the provider id.
func (p *Provider) Name() string { return p.name }

// Generate makes one call.
func (p *Provider) Generate(ctx context.Context, req agentloop.Request) (agentloop.Response, error) {
	var out generated
	path := "/models/" + req.Model + ":generateContent"
	if err := p.client.PostJSON(ctx, path, body(req), &out); err != nil {
		return agentloop.Response{}, err
	}
	return translate(out, req.Model), nil
}

// Stream makes the same call against the streaming endpoint.
func (p *Provider) Stream(ctx context.Context, req agentloop.Request) (agentloop.Stream, error) {
	path := "/models/" + req.Model + ":streamGenerateContent?alt=sse"
	reader, err := p.client.PostSSE(ctx, path, body(req))
	if err != nil {
		return nil, err
	}
	return &stream{reader: reader, model: req.Model}, nil
}

func body(req agentloop.Request) map[string]any {
	out := map[string]any{"contents": contents(req.Messages)}
	if req.Instructions != "" {
		out["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.Instructions}},
		}
	}
	if len(req.Tools) > 0 {
		out["tools"] = []map[string]any{{"functionDeclarations": toolDefs(req.Tools)}}
	}
	if budget, ok := thinkingBudget[req.Reasoning]; ok {
		out["generationConfig"] = map[string]any{
			"thinkingConfig": map[string]any{"thinkingBudget": budget},
		}
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
			if params := geminiParameters(t.InputSchema); params != nil {
				def["parameters"] = params
			}
		}
		out = append(out, def)
	}
	return out
}

// geminiSchemaKeys is what this API's Schema accepts. It is an OpenAPI 3.0
// subset, not JSON Schema, and it rejects the request outright — 400, whole
// turn lost — on any field it does not know.
//
// This is a whitelist rather than a list of things to strip, deliberately: a
// tool schema is generated from a Go input struct, so a field type nobody
// anticipated can introduce a keyword at any time. Dropping what is not
// recognised degrades one parameter's description; forwarding it fails every
// call to every tool.
var geminiSchemaKeys = map[string]bool{
	"type": true, "format": true, "title": true, "description": true,
	"nullable": true, "enum": true, "items": true, "properties": true,
	"required": true, "minItems": true, "maxItems": true,
	"minLength": true, "maxLength": true, "pattern": true,
	"minimum": true, "maximum": true, "default": true, "example": true,
	"anyOf": true,
}

// geminiParameters renders a tool's input schema in the shape this API takes.
//
// Sending JSON Schema straight through is what the adapter used to do, and it
// made every tool-carrying turn fail: the registry's schemas carry
// `additionalProperties` (no such field in the Schema proto) and union types
// as `"type": ["string", "null"]` (the proto's type is a single enum, not
// repeating). Both are rejected with "Invalid JSON payload received", so the
// agent could never call a tool — which, since the loop always sends its
// toolset, meant it could never answer at all.
func geminiParameters(in *jsonschema.Schema) map[string]any {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	return geminiSchemaOf(doc, definitionsOf(doc), 0)
}

// definitionsOf collects the reusable subschemas a $ref can point at, so that
// resolving one inlines the real thing instead of dropping the parameter.
func definitionsOf(doc map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"$defs", "definitions"} {
		if defs, ok := doc[key].(map[string]any); ok {
			for name, def := range defs {
				out[name] = def
			}
		}
	}
	return out
}

// maxSchemaDepth bounds $ref inlining. A self-referential schema is legal
// JSON Schema and would otherwise expand forever.
const maxSchemaDepth = 12

func geminiSchemaOf(node map[string]any, defs map[string]any, depth int) map[string]any {
	if node == nil || depth > maxSchemaDepth {
		return nil
	}
	if ref, ok := node["$ref"].(string); ok {
		if target := resolveRef(ref, defs); target != nil {
			return geminiSchemaOf(target, defs, depth+1)
		}
		// An unresolvable $ref carries no shape this API can use; an empty
		// object is at least a valid parameter it will accept.
		return map[string]any{"type": "object"}
	}

	out := map[string]any{}
	for key, value := range node {
		if !geminiSchemaKeys[key] {
			continue
		}
		out[key] = value
	}

	// A union type is the single commonest divergence: JSON Schema writes
	// an optional string as `["string", "null"]`, which this API reads as a
	// list where it wants one value.
	if list, ok := out["type"].([]any); ok {
		delete(out, "type")
		for _, item := range list {
			name, _ := item.(string)
			if name == "null" {
				out["nullable"] = true
				continue
			}
			if _, taken := out["type"]; !taken && name != "" {
				out["type"] = name
			}
		}
	}

	// `const` has no counterpart here, but a one-value enum means the same
	// thing and is accepted.
	if value, ok := node["const"]; ok {
		if text, isText := value.(string); isText {
			out["enum"] = []any{text}
			if _, typed := out["type"]; !typed {
				out["type"] = "string"
			}
		}
	}

	// This API's enum is a list of strings against a string type. A
	// numeric or mixed enum would be rejected, and the type alone still
	// describes the parameter.
	if values, ok := out["enum"].([]any); ok {
		for _, value := range values {
			if _, isText := value.(string); !isText {
				delete(out, "enum")
				break
			}
		}
	}

	if properties, ok := node["properties"].(map[string]any); ok {
		converted := map[string]any{}
		for name, child := range properties {
			childNode, isObject := child.(map[string]any)
			if !isObject {
				continue
			}
			if rendered := geminiSchemaOf(childNode, defs, depth+1); rendered != nil {
				converted[name] = rendered
			}
		}
		if len(converted) == 0 {
			delete(out, "properties")
		} else {
			out["properties"] = converted
		}
	}

	if items, ok := node["items"].(map[string]any); ok {
		if rendered := geminiSchemaOf(items, defs, depth+1); rendered != nil {
			out["items"] = rendered
		} else {
			delete(out, "items")
		}
	}

	if branches, ok := node["anyOf"].([]any); ok {
		rendered := make([]any, 0, len(branches))
		for _, branch := range branches {
			branchNode, isObject := branch.(map[string]any)
			if !isObject {
				continue
			}
			if converted := geminiSchemaOf(branchNode, defs, depth+1); converted != nil {
				rendered = append(rendered, converted)
			}
		}
		if len(rendered) == 0 {
			delete(out, "anyOf")
		} else {
			out["anyOf"] = rendered
		}
	}

	// `required` may only name properties that survived; naming a dropped
	// one describes a parameter the model cannot see.
	if names, ok := out["required"].([]any); ok {
		properties, _ := out["properties"].(map[string]any)
		kept := make([]any, 0, len(names))
		for _, name := range names {
			text, _ := name.(string)
			if _, present := properties[text]; present {
				kept = append(kept, text)
			}
		}
		if len(kept) == 0 {
			delete(out, "required")
		} else {
			out["required"] = kept
		}
	}

	return out
}

func resolveRef(ref string, defs map[string]any) map[string]any {
	for _, prefix := range []string{"#/$defs/", "#/definitions/"} {
		if name, found := strings.CutPrefix(ref, prefix); found {
			target, _ := defs[name].(map[string]any)
			return target
		}
	}
	return nil
}

// contents renders the conversation. This API calls the assistant "model" and
// carries a tool result as a functionResponse part on a user turn.
func contents(messages []agentloop.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case agentloop.RoleTool:
			out = append(out, map[string]any{
				"role": "user",
				"parts": []map[string]any{{
					"functionResponse": map[string]any{
						"name":     m.Name,
						"response": responseValue(m.Result),
					},
				}},
			})

		case agentloop.RoleAssistant:
			var parts []map[string]any
			if m.Text != "" {
				parts = append(parts, map[string]any{"text": m.Text})
			}
			for _, c := range m.ToolCalls {
				// The signature rides alongside the call, not inside it —
				// see agentloop.ToolCall.Signature for why it has to come
				// back at all.
				part := map[string]any{
					"functionCall": map[string]any{
						"name": c.Name,
						"args": json.RawMessage(providers.JSONString(c.Input)),
					},
				}
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

type generated struct {
	Candidates []struct {
		Content struct {
			Parts []part `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
		ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

type part struct {
	Text         string `json:"text"`
	Thought      bool   `json:"thought"`
	FunctionCall *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
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
	candidate := g.Candidates[0]

	var text, thinking strings.Builder
	var index int
	for _, p := range candidate.Content.Parts {
		switch {
		case p.FunctionCall != nil:
			// This API does not give a call an id, and the loop pairs a result
			// with its call by one, so the position in the answer becomes it.
			index++
			out.ToolCalls = append(out.ToolCalls, agentloop.ToolCall{
				ID:        p.FunctionCall.Name + "-" + itoa(index),
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

	switch candidate.FinishReason {
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

// stream reads the chunked answer, each frame carrying a whole candidate.
type stream struct {
	reader *providers.EventReader
	model  string
	whole  generated
}

func (s *stream) Recv() (agentloop.Chunk, error) {
	for {
		e, err := s.reader.Next()
		if err != nil {
			return agentloop.Chunk{}, err
		}
		var frame generated
		if err := json.Unmarshal(e.Data, &frame); err != nil {
			continue
		}
		s.merge(frame)
		if len(frame.Candidates) == 0 {
			continue
		}
		var text, thinking strings.Builder
		for _, p := range frame.Candidates[0].Content.Parts {
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

// merge accumulates the frames into one answer, because each carries only its
// own slice of the parts.
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
		s.whole.Candidates = make([]struct {
			Content struct {
				Parts []part `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		}, 1)
	}
	s.whole.Candidates[0].Content.Parts = append(
		s.whole.Candidates[0].Content.Parts, frame.Candidates[0].Content.Parts...)
	if frame.Candidates[0].FinishReason != "" {
		s.whole.Candidates[0].FinishReason = frame.Candidates[0].FinishReason
	}
}

func (s *stream) Response() agentloop.Response {
	// The text arrives in many parts; the answer is their concatenation, which
	// translate already does.
	return translate(s.whole, s.model)
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}
