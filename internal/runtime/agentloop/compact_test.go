package agentloop_test

import (
	"strconv"
	"testing"

	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// conversation builds turns of one assistant message asking for a tool and one
// tool message answering it, which is the shape every working session has.
func conversation(turns int) []agentloop.Message {
	out := []agentloop.Message{{Role: agentloop.RoleUser, Text: "build the API"}}
	for i := 0; i < turns; i++ {
		id := "call_" + strconv.Itoa(i)
		out = append(out,
			agentloop.Message{
				Role:      agentloop.RoleAssistant,
				ToolCalls: []agentloop.ToolCall{{ID: id, Name: "Read", Input: []byte(`{}`)}},
			},
			agentloop.Message{
				Role: agentloop.RoleTool, CallID: id, Name: "Read", Result: []byte(`"ok"`),
			},
		)
	}
	return out
}

// callIDs of every tool call still being offered, and of every tool result
// still being answered with.
func callIDs(messages []agentloop.Message) (calls, results map[string]bool) {
	calls, results = map[string]bool{}, map[string]bool{}
	for _, m := range messages {
		for _, c := range m.ToolCalls {
			calls[c.ID] = true
		}
		if m.Role == agentloop.RoleTool && m.CallID != "" {
			results[m.CallID] = true
		}
	}
	return calls, results
}

// TestPruneNeverOrphansAToolResult is the defect behind AOS_AGENT_PROVIDER_FAILED.
//
// Compaction kept tool calls for the last KeepToolCalls messages and stripped
// them from everything older, on a flat index. But a turn is two messages —
// the assistant message that asks and the tool message that answers — and the
// cut fell between them: the assistant message lost its calls and was dropped
// as empty, while its results, one index later, were inside the window and
// kept.
//
// What went to the provider was a function_call_output with no function_call,
// and the Responses API refuses the whole request:
//
//	No tool call found for function call output with call_id call_…
//
// Every session long enough to compact died there, which is most of what "the
// agent could not do anything" was.
func TestPruneNeverOrphansAToolResult(t *testing.T) {
	// Every length around and well past the window: the boundary lands between
	// a call and its result on every other one.
	for turns := 1; turns <= 40; turns++ {
		pruned := agentloop.Prune(conversation(turns), agentloop.DefaultPolicy())
		calls, results := callIDs(pruned)
		for id := range results {
			if !calls[id] {
				t.Fatalf("turns=%d: result %s survived with no call to answer — "+
					"this is the payload the provider refuses", turns, id)
			}
		}
	}
}

// TestPruneKeepsTheRecentToolCalls: the fix must not be "drop everything".
// Compaction exists to save context, and the newest exchanges are the ones the
// model is still reasoning with.
func TestPruneKeepsTheRecentToolCalls(t *testing.T) {
	pruned := agentloop.Prune(conversation(40), agentloop.DefaultPolicy())
	calls, results := callIDs(pruned)
	if len(calls) == 0 || len(results) == 0 {
		t.Fatalf("compaction removed every exchange: %d calls, %d results", len(calls), len(results))
	}
	// The last turn is always within the window.
	if !calls["call_39"] || !results["call_39"] {
		t.Error("the most recent exchange was pruned")
	}
}

// TestPruneStillDropsOldExchanges: a fix that paired everything by keeping
// everything would defeat the purpose.
func TestPruneStillDropsOldExchanges(t *testing.T) {
	full := conversation(40)
	pruned := agentloop.Prune(full, agentloop.DefaultPolicy())
	if len(pruned) >= len(full) {
		t.Fatalf("nothing was pruned: %d in, %d out", len(full), len(pruned))
	}
	calls, _ := callIDs(pruned)
	if calls["call_0"] {
		t.Error("the oldest exchange survived compaction")
	}
}

// TestPruneKeepsTheLastUserMessage is the rule the function documents: an
// agent that pruned away what it was asked to do would answer a question
// nobody asked.
func TestPruneKeepsTheLastUserMessage(t *testing.T) {
	messages := conversation(40)
	messages = append(messages, agentloop.Message{Role: agentloop.RoleUser, Text: "now deploy it"})
	pruned := agentloop.Prune(messages, agentloop.DefaultPolicy())

	last := pruned[len(pruned)-1]
	if last.Role != agentloop.RoleUser || last.Text != "now deploy it" {
		t.Fatalf("last message = %+v, want the user's own request", last)
	}
}
