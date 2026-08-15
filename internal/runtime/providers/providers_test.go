package providers_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/anthropic"
	"github.com/OWNER/aos/internal/runtime/providers/compat"
	"github.com/OWNER/aos/internal/runtime/providers/google"
	"github.com/OWNER/aos/internal/runtime/providers/openai"
	"github.com/OWNER/aos/internal/runtime/providers/providertest"
)

// TestEveryProviderObeysTheContract. Eight registrations, four wire formats,
// one loop that knows about none of them.
func TestEveryProviderObeysTheContract(t *testing.T) {
	cases := []providertest.Case{
		{
			Name: "openai",
			Build: func(base string) agentloop.LLMProvider {
				return openai.New("openai", providers.Config{BaseURL: base, APIKey: "k"}, nil)
			},
			Answer: providertest.Exchange{
				Body: `{"model":"test-model","status":"completed","output":[
					{"type":"reasoning","encrypted_content":"gAAAAA","summary":[]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"the readme says hello"}]}],
					"usage":{"input_tokens":120,"output_tokens":9,"total_tokens":129,
					"output_tokens_details":{"reasoning_tokens":4}}}`,
				Stream: sse(
					`{"type":"response.output_text.delta","delta":"the readme "}`,
					`{"type":"response.output_text.delta","delta":"says hello"}`,
					`{"type":"response.completed","response":{"model":"test-model","status":"completed","output":[
						{"type":"message","role":"assistant","content":[{"type":"output_text","text":"the readme says hello"}]}],
						"usage":{"input_tokens":120,"output_tokens":9,"total_tokens":129}}}`,
				),
			},
			ToolCall: providertest.Exchange{
				Body: `{"model":"test-model","status":"completed","output":[
					{"type":"function_call","call_id":"fc_1","name":"Read","arguments":"{\"path\":\"README.md\"}"}],
					"usage":{"input_tokens":100,"output_tokens":12,"total_tokens":112}}`,
			},
			WantToolName: "Read", WantToolArgs: `{"path":"README.md"}`,
		},
		{
			Name: "anthropic",
			Build: func(base string) agentloop.LLMProvider {
				return anthropic.New(providers.Config{BaseURL: base, APIKey: "k"})
			},
			Answer: providertest.Exchange{
				Body: `{"model":"test-model","stop_reason":"end_turn","content":[
					{"type":"thinking","thinking":"checking"},
					{"type":"text","text":"the readme says hello"}],
					"usage":{"input_tokens":120,"output_tokens":9}}`,
				Stream: sse(
					`{"type":"message_start","message":{"model":"test-model","usage":{"input_tokens":120}}}`,
					`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
					`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"the readme "}}`,
					`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"says hello"}}`,
					`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}`,
					`{"type":"message_stop"}`,
				),
			},
			ToolCall: providertest.Exchange{
				Body: `{"model":"test-model","stop_reason":"tool_use","content":[
					{"type":"tool_use","id":"toolu_1","name":"Read","input":{"path":"README.md"}}],
					"usage":{"input_tokens":100,"output_tokens":12}}`,
			},
			WantToolName: "Read", WantToolArgs: `{"path":"README.md"}`,
		},
		{
			Name: "openrouter",
			Build: func(base string) agentloop.LLMProvider {
				return compat.New("openrouter", providers.Config{BaseURL: base, APIKey: "k"}, compat.OpenRouterBaseURL)
			},
			Answer: providertest.Exchange{
				Body: `{"model":"test-model","choices":[{"finish_reason":"stop","message":
					{"content":"the readme says hello","reasoning":"checking"}}],
					"usage":{"prompt_tokens":120,"completion_tokens":9,"total_tokens":129}}`,
				Stream: sse(
					`{"model":"test-model","choices":[{"delta":{"content":"the readme "}}]}`,
					`{"model":"test-model","choices":[{"delta":{"content":"says hello"},"finish_reason":"stop"}]}`,
					`{"model":"test-model","choices":[],"usage":{"prompt_tokens":120,"completion_tokens":9,"total_tokens":129}}`,
					`[DONE]`,
				),
			},
			ToolCall: providertest.Exchange{
				Body: `{"model":"test-model","choices":[{"finish_reason":"tool_calls","message":{"content":"",
					"tool_calls":[{"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"path\":\"README.md\"}"}}]}}],
					"usage":{"prompt_tokens":100,"completion_tokens":12,"total_tokens":112}}`,
			},
			WantToolName: "Read", WantToolArgs: `{"path":"README.md"}`,
		},
		{
			Name: "google",
			Build: func(base string) agentloop.LLMProvider {
				return google.New("google", providers.Config{BaseURL: base, APIKey: "k"}, nil)
			},
			Answer: providertest.Exchange{
				Body: `{"modelVersion":"test-model","candidates":[{"finishReason":"STOP","content":{"parts":[
					{"text":"checking","thought":true},
					{"text":"the readme says hello"}]}}],
					"usageMetadata":{"promptTokenCount":120,"candidatesTokenCount":9,"totalTokenCount":129,"thoughtsTokenCount":4}}`,
				Stream: sse(
					`{"modelVersion":"test-model","candidates":[{"content":{"parts":[{"text":"the readme "}]}}]}`,
					`{"modelVersion":"test-model","candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"says hello"}]}}],
						"usageMetadata":{"promptTokenCount":120,"candidatesTokenCount":9,"totalTokenCount":129}}`,
				),
			},
			ToolCall: providertest.Exchange{
				Body: `{"modelVersion":"test-model","candidates":[{"finishReason":"STOP","content":{"parts":[
					{"functionCall":{"name":"Read","args":{"path":"README.md"}}}]}}],
					"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":12,"totalTokenCount":112}}`,
			},
			WantToolName: "Read", WantToolArgs: `{"path":"README.md"}`,
		},
	}

	for _, c := range cases {
		providertest.Run(t, c)
	}
}

// TestEveryProviderInTheSpecificationIsRegistered. Registration happens from an
// init function, so this also proves that importing an adapter is all it takes.
func TestEveryProviderInTheSpecificationIsRegistered(t *testing.T) {
	want := []string{
		"anthropic", "codex", "crof", "gemini-cli", "google", "openai", "opencode", "openrouter",
	}
	got := providers.Names()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("registered = %v, want %v", got, want)
	}
	for _, id := range want {
		if _, err := providers.Build(id, providers.Config{Home: t.TempDir()}); err != nil {
			t.Errorf("%s could not be built: %v", id, err)
		}
	}
}

// TestAProviderNobodyRegisteredSaysWhatIsAvailable.
func TestAProviderNobodyRegisteredSaysWhatIsAvailable(t *testing.T) {
	_, err := providers.Build("invented", providers.Config{})
	if err == nil {
		t.Fatal("an unknown provider was built")
	}
	if !strings.Contains(err.Error(), "PROVIDER_UNKNOWN") {
		t.Fatalf("err = %v", err)
	}
}

// TestTheTwoWaysACapabilityCanBeUnavailableAreDistinct. "You did not configure
// a voice model" and "the provider you configured cannot speak" send a person
// to two different places.
func TestTheTwoWaysACapabilityCanBeUnavailableAreDistinct(t *testing.T) {
	_, err := providers.Capability[agentloop.SpeechProvider](
		providers.Slot{Name: "voice"}, providers.Config{})
	if err == nil || !strings.Contains(err.Error(), "CAPABILITY_MODEL_MISSING") {
		t.Fatalf("an unconfigured slot gave %v", err)
	}

	_, err = providers.Capability[agentloop.SpeechProvider](
		providers.Slot{Name: "voice", Provider: "anthropic", Model: "claude-sonnet-5"},
		providers.Config{})
	if err == nil || !strings.Contains(err.Error(), "CAPABILITY_NOT_SUPPORTED") {
		t.Fatalf("a provider without the capability gave %v", err)
	}

	// The one that is configured and supported resolves.
	if _, err := providers.Capability[agentloop.LLMProvider](
		providers.Slot{Name: "default", Provider: "openai", Model: "gpt-5"},
		providers.Config{APIKey: "k"}); err != nil {
		t.Fatalf("a working slot did not resolve: %v", err)
	}
}

// TestOpenCodeRoutesTheFreeModelsElsewhere, which is the one routing rule any
// of the compatible providers has.
func TestOpenCodeRoutesTheFreeModelsElsewhere(t *testing.T) {
	p, err := providers.Build("opencode", providers.Config{})
	if err != nil {
		t.Fatal(err)
	}
	impl, ok := p.(*compat.Provider)
	if !ok {
		t.Fatalf("opencode is a %T", p)
	}
	if got := impl.EndpointFor("some-model"); got != compat.OpenCodeBaseURL {
		t.Errorf("an ordinary model goes to %s", got)
	}
	if got := impl.EndpointFor("some-model-free"); got != compat.OpenCodeFreeBaseURL {
		t.Errorf("a free model goes to %s", got)
	}

	// An installation that overrode the endpoint keeps all its traffic there.
	overridden, err := providers.Build("opencode", providers.Config{BaseURL: "https://proxy.internal/v1"})
	if err != nil {
		t.Fatal(err)
	}
	if got := overridden.(*compat.Provider).EndpointFor("some-model-free"); got != "https://proxy.internal/v1" {
		t.Errorf("an overridden endpoint still split its traffic: %s", got)
	}
}

// sse renders frames the way a provider does.
//
// Each frame is compacted first: a newline inside a data line ends it, so a
// recorded frame written across several lines for readability would arrive
// truncated — which is a property of the format, and a thing worth getting
// right in the fixture rather than working around in the reader.
func sse(frames ...string) string {
	var b strings.Builder
	for _, f := range frames {
		b.WriteString("data: " + compactJSON(f) + "\n\n")
	}
	return b.String()
}

func compactJSON(s string) string {
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(s)); err != nil {
		return s // [DONE] and anything else that is not JSON
	}
	return out.String()
}
