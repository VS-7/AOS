package google

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// Every call a turn makes is named apart from every other. The adapter used to
// name a call by its position in the answer, so each step that began with
// Read produced "Read-1": the stored answer held three calls of that name, and
// the chat drew all three results into the last one.
func TestCallsInDifferentStepsAreNamedApart(t *testing.T) {
	answer := func() agentloop.Response {
		var g generated
		if err := json.Unmarshal([]byte(`{"candidates":[{"content":{"parts":[
			{"functionCall":{"name":"Read","args":{}}},
			{"functionCall":{"name":"Read","args":{}}}]}}]}`), &g); err != nil {
			t.Fatal(err)
		}
		return translate(g, "gemini-3-pro")
	}
	seen := map[string]bool{}
	for range 3 {
		for _, c := range answer().ToolCalls {
			if c.ID == "" || seen[c.ID] {
				t.Fatalf("the id %q names more than one call", c.ID)
			}
			seen[c.ID] = true
		}
	}
}

// The results of one step are one user turn with a part per call — Gemini
// refuses two turns of one part each for a turn that made two calls — and a
// turn that says something ends the step.
func TestAStepsResultsAreOneTurn(t *testing.T) {
	got := contents([]agentloop.Message{
		{Role: agentloop.RoleUser, Text: "read both"},
		{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{{ID: "a", Name: "Read"}, {ID: "b", Name: "Read"}}},
		{Role: agentloop.RoleTool, CallID: "a", Name: "Read", Result: json.RawMessage(`"1"`)},
		{Role: agentloop.RoleTool, CallID: "b", Name: "Read", Result: json.RawMessage(`"2"`)},
		{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{{ID: "c", Name: "Read"}}},
		{Role: agentloop.RoleTool, CallID: "c", Name: "Read", Result: json.RawMessage(`"3"`)},
		{Role: agentloop.RoleUser, Text: "and now?"},
	})
	var shape []int
	for _, c := range got {
		parts, _ := c["parts"].([]map[string]any)
		shape = append(shape, len(parts))
	}
	want := []int{1, 2, 2, 1, 1, 1}
	if len(shape) != len(want) {
		t.Fatalf("turns and their parts = %v, want %v", shape, want)
	}
	for i := range want {
		if shape[i] != want[i] {
			t.Fatalf("turns and their parts = %v, want %v", shape, want)
		}
	}
}
