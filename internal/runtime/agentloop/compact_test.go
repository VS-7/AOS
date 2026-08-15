package agentloop_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// history builds a conversation long enough to be pruned: a user message, then
// n rounds of an assistant asking for a tool and reading its answer.
func history(rounds int) []agentloop.Message {
	out := []agentloop.Message{{Role: agentloop.RoleUser, Text: "audit the repository"}}
	for i := range rounds {
		id := "c" + itoa(i)
		out = append(out,
			agentloop.Message{
				Role: agentloop.RoleAssistant,
				Text: "looking at file " + itoa(i),
				// Reasoning is the largest thing in a real history and the
				// first thing a prune removes.
				Reasoning: strings.Repeat("thinking about this at length. ", 20),
				ToolCalls: []agentloop.ToolCall{{ID: id, Name: "Read", Input: json.RawMessage(`{"file_path":"x"}`)}},
			},
			agentloop.Message{
				Role: agentloop.RoleTool, CallID: id, Name: "Read",
				Result: json.RawMessage(`{"content":"` + strings.Repeat("a", 100) + `"}`),
			},
		)
	}
	return out
}

// TestPruningKeepsWhatTheModelStillNeeds: the reasoning goes, the recent tool
// traffic stays, and the request the agent is answering is never removed.
func TestPruningKeepsWhatTheModelStillNeeds(t *testing.T) {
	messages := history(20)
	pruned := agentloop.Prune(messages, agentloop.DefaultPolicy())

	if len(pruned) >= len(messages) {
		t.Fatalf("nothing was pruned: %d messages in, %d out", len(messages), len(pruned))
	}

	var reasoning int
	for _, m := range pruned {
		if m.Reasoning != "" {
			reasoning++
		}
	}
	if reasoning != 0 {
		t.Errorf("%d messages kept their reasoning", reasoning)
	}

	// The tool traffic of the last fifteen messages survives, which is what
	// the model is actually reasoning about right now.
	var recentTools int
	for _, m := range pruned[max(0, len(pruned)-agentloop.KeepToolCalls):] {
		if len(m.ToolCalls) > 0 || m.Role == agentloop.RoleTool {
			recentTools++
		}
	}
	if recentTools == 0 {
		t.Error("the recent tool calls were pruned along with the old ones")
	}

	if pruned[0].Role != agentloop.RoleUser || pruned[0].Text != "audit the repository" {
		t.Fatalf("the request being answered was pruned: %+v", pruned[0])
	}
}

// TestTheLastUserMessageSurvivesEveryPolicy. An agent that pruned away the
// thing it was asked to do would answer a question nobody asked.
func TestTheLastUserMessageSurvivesEveryPolicy(t *testing.T) {
	messages := append(history(20),
		agentloop.Message{Role: agentloop.RoleUser, Text: "now summarise what you found"})

	pruned := agentloop.Prune(messages, agentloop.Policy{
		Reasoning: true, ToolCalls: true, EmptyMessages: true,
	})
	last := pruned[len(pruned)-1]
	if last.Role != agentloop.RoleUser || last.Text != "now summarise what you found" {
		t.Fatalf("the last user message is %+v", last)
	}
}

// TestAnEmptyPolicyChangesNothing, so a workspace can turn compaction off and
// get exactly its history back.
func TestAnEmptyPolicyChangesNothing(t *testing.T) {
	messages := history(5)
	if got := agentloop.Prune(messages, agentloop.Policy{}); len(got) != len(messages) {
		t.Fatalf("a policy that removes nothing removed %d messages", len(messages)-len(got))
	}
	if got := agentloop.Prune(nil, agentloop.DefaultPolicy()); got != nil {
		t.Fatalf("pruning nothing produced %v", got)
	}
}

// TestTheThresholdIsTheOriginalsNumber.
func TestTheThresholdIsTheOriginalsNumber(t *testing.T) {
	c := agentloop.NewCompactor()
	if c.Threshold != agentloop.ThresholdChars || agentloop.ThresholdChars != 100_000 {
		t.Fatalf("threshold = %d", c.Threshold)
	}
	if c.ShouldCompact(99_999) {
		t.Error("a history under the threshold was compacted")
	}
	if !c.ShouldCompact(100_001) {
		t.Error("a history over the threshold was not compacted")
	}
	// A compactor built by hand with no threshold falls back rather than
	// compacting on every single call.
	empty := &agentloop.Compactor{}
	if empty.ShouldCompact(10) {
		t.Error("a zero threshold compacted a ten-character history")
	}
}

// TestPreCompactFiresBeforeTheHistoryIsPruned, which is the only order that
// lets an extension say what must survive.
func TestPreCompactFiresBeforeTheHistoryIsPruned(t *testing.T) {
	var sawMessages int
	b, _ := bus(t, event.FuncHandler{
		Name: "keeper", Events: []event.Type{event.PreCompact},
		Fn: func(context.Context, event.Event) (event.Outcome, error) {
			sawMessages++
			return event.Outcome{AdditionalContext: "the audit found three problems; keep that"}, nil
		},
	})

	s := state()
	s.Messages = history(400) // comfortably past the threshold

	p := &fake.Provider{Script: []fake.Step{{Text: "here is the summary"}}}
	l := agentloop.New(agentloop.Deps{
		Provider: p, Tools: toolexec.NewRegistry(),
		Hooks: &agentloop.EventHooks{Bus: b},
		Clock: clockx.Fixed{At: refTime}, Log: quiet(),
	})

	res, err := l.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if sawMessages != 1 {
		t.Fatalf("PreCompact fired %d times", sawMessages)
	}
	if res.Compactions != 1 {
		t.Fatalf("Compactions = %d", res.Compactions)
	}
	// The context the hook injected reached the model, which is the whole
	// point of firing before rather than after.
	var found bool
	for _, m := range p.Requests()[0].Messages {
		if strings.Contains(m.Text, "three problems") {
			found = true
		}
	}
	if !found {
		t.Error("what the hook asked to keep did not reach the model")
	}
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
