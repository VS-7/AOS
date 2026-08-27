package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// The two shapes this provider's reasoning parameter takes, and which model
// gets which. Sending the wrong one is not a degradation — the provider
// answers 400 and the turn never happens.
func TestReasoningTakesTheShapeTheModelAccepts(t *testing.T) {
	for _, tc := range []struct {
		model    string
		adaptive bool
	}{
		{"claude-opus-5", true},
		{"claude-sonnet-5", true},
		{"claude-opus-4-8", true},
		{"claude-sonnet-4-6", true},
		{"anthropic.claude-opus-5", true},  // the Bedrock spelling
		{"claude-opus-4-6@20260101", true}, // a dated snapshot
		{"claude-haiku-4-5", false},        // older: still takes a budget
		{"claude-3-5-sonnet-20241022", false},
	} {
		out := body(agentloop.Request{Model: tc.model, Reasoning: agentloop.ReasoningMedium}, false)
		thinking, ok := out["thinking"].(map[string]any)
		if !ok {
			t.Fatalf("%s: no thinking parameter at all: %#v", tc.model, out["thinking"])
		}
		if tc.adaptive {
			if thinking["type"] != "adaptive" {
				t.Errorf("%s: thinking.type = %v, want adaptive", tc.model, thinking["type"])
			}
			if _, has := thinking["budget_tokens"]; has {
				t.Errorf("%s: budget_tokens is rejected by this model with a 400", tc.model)
			}
			cfg, ok := out["output_config"].(map[string]any)
			if !ok || cfg["effort"] != "medium" {
				t.Errorf("%s: output_config = %#v, want effort medium", tc.model, out["output_config"])
			}
			continue
		}
		if thinking["type"] != "enabled" || thinking["budget_tokens"] == nil {
			t.Errorf("%s: thinking = %#v, want an enabled budget", tc.model, thinking)
		}
		if _, has := out["output_config"]; has {
			t.Errorf("%s: effort is not a parameter this model takes", tc.model)
		}
	}

	// Reasoning off means no thinking parameter of either kind.
	out := body(agentloop.Request{Model: "claude-opus-5", Reasoning: agentloop.ReasoningNone}, false)
	if _, has := out["thinking"]; has {
		t.Errorf("reasoning none still asked for thinking: %#v", out["thinking"])
	}
}

// A tool-using turn has to be replayed with the thinking block that asked for
// the tool, signature intact. Without it the provider refuses the next call,
// so the whole loop stops at the second step.
func TestAToolTurnIsReplayedWithItsSignedThinkingBlock(t *testing.T) {
	out := messages([]agentloop.Message{
		{Role: agentloop.RoleUser, Text: "what is in the file?"},
		{
			Role:      agentloop.RoleAssistant,
			Reasoning: "I should read it",
			Encrypted: "sig-abc",
			ToolCalls: []agentloop.ToolCall{{ID: "t1", Name: "fs_read", Input: json.RawMessage(`{"path":"a"}`)}},
		},
		{Role: agentloop.RoleTool, CallID: "t1", Result: json.RawMessage(`"contents"`)},
	})

	if len(out) != 3 {
		t.Fatalf("messages = %d, want user, assistant, tool result", len(out))
	}
	content, ok := out[1]["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("assistant content = %#v", out[1]["content"])
	}
	first := content[0]
	if first["type"] != "thinking" {
		t.Fatalf("the thinking block is not first: %#v", content)
	}
	if first["signature"] != "sig-abc" || first["thinking"] != "I should read it" {
		t.Errorf("the thinking block was not replayed unchanged: %#v", first)
	}
	if content[len(content)-1]["type"] != "tool_use" {
		t.Errorf("the tool call did not survive: %#v", content)
	}

	// An assistant turn with no signature carries no thinking block: an
	// unsigned one is refused, and a turn that never thought has none.
	plain := messages([]agentloop.Message{{Role: agentloop.RoleAssistant, Text: "hello"}})
	blocks := plain[0]["content"].([]map[string]any)
	if blocks[0]["type"] != "text" {
		t.Errorf("an unsigned turn invented a thinking block: %#v", blocks)
	}
}

// The signature only exists if it is read off the response in the first place.
func TestTheSignatureIsCarriedOffTheResponse(t *testing.T) {
	var m message
	raw := `{"content":[
		{"type":"thinking","thinking":"pondering","signature":"sig-xyz"},
		{"type":"text","text":"done"},
		{"type":"tool_use","id":"t1","name":"fs_read","input":{}}],
		"stop_reason":"tool_use","model":"claude-opus-5"}`
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	got := translate(m, "claude-opus-5")
	if got.Message.Encrypted != "sig-xyz" {
		t.Errorf("Encrypted = %q, want the thinking block's signature", got.Message.Encrypted)
	}
	if got.Message.Reasoning != "pondering" || got.Message.Text != "done" {
		t.Errorf("message = %+v", got.Message)
	}
	if got.StopReason != agentloop.StopToolCalls || len(got.ToolCalls) != 1 {
		t.Errorf("stop = %v calls = %d", got.StopReason, len(got.ToolCalls))
	}
}

// Tools and instructions still travel, which is the rest of the request this
// change touches.
func TestTheRequestStillCarriesToolsAndInstructions(t *testing.T) {
	out := body(agentloop.Request{
		Model:        "claude-opus-5",
		Instructions: "be brief",
		Tools:        []toolexec.Spec{{Name: "fs_read", Description: "read a file"}},
	}, true)
	if out["system"] != "be brief" {
		t.Errorf("system = %v", out["system"])
	}
	if out["stream"] != true {
		t.Errorf("stream = %v", out["stream"])
	}
	tools, ok := out["tools"].([]map[string]any)
	if !ok || len(tools) != 1 || tools[0]["name"] != "fs_read" {
		t.Errorf("tools = %#v", out["tools"])
	}
}
