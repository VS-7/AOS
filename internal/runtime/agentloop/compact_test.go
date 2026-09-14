package agentloop_test

import (
	"strconv"
	"strings"
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

// TestARequestNeverCarriesAToolMessageWithoutItsPair is the repair for
// conversations stored before persistence recorded calls beside results.
//
// A stored answer could hold the results of a turn's earliest tool calls and
// not the calls themselves — compaction had pruned the calls out of the
// transcript persistence read them from. Replayed, those results reach the
// provider as function_call_output items with no function_call, the request is
// refused, and so is every later one: the conversation could never be answered
// again. Whatever the history holds, what is sent must pair.
func TestARequestNeverCarriesAToolMessageWithoutItsPair(t *testing.T) {
	s := &agentloop.State{Messages: []agentloop.Message{
		{Role: agentloop.RoleUser, Text: "build the API"},
		// A result whose call is gone: the stored shape of the defect.
		{Role: agentloop.RoleTool, CallID: "orphan", Name: "Read", Result: []byte(`"lost"`)},
		{Role: agentloop.RoleAssistant, Text: "reading", ToolCalls: []agentloop.ToolCall{
			{ID: "kept", Name: "Read"},
			// A call nothing answered, which the provider refuses just as firmly.
			{ID: "unanswered", Name: "Glob"},
		}},
		{Role: agentloop.RoleTool, CallID: "kept", Name: "Read", Result: []byte(`"ok"`)},
		// A result that answers a call made only after it.
		{Role: agentloop.RoleTool, CallID: "late", Name: "Read", Result: []byte(`"early"`)},
		{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{{ID: "late", Name: "Read"}}},
		{Role: agentloop.RoleUser, Text: "and now?"},
	}}

	sent := s.Request().Messages
	offered := map[string]bool{}
	answered := map[string]bool{}
	for _, m := range sent {
		if m.Role == agentloop.RoleTool {
			if !offered[m.CallID] {
				t.Errorf("result %s was sent with no call before it", m.CallID)
			}
			answered[m.CallID] = true
		}
		for _, c := range m.ToolCalls {
			offered[c.ID] = true
		}
	}
	for id := range offered {
		if !answered[id] {
			t.Errorf("call %s was sent with no result after it", id)
		}
	}
	if !offered["kept"] || !answered["kept"] {
		t.Error("the exchange that was whole did not survive the repair")
	}
	if last := sent[len(sent)-1]; last.Role != agentloop.RoleUser || last.Text != "and now?" {
		t.Errorf("last message = %+v, want the question", last)
	}
	// The stored history is the record; only what is sent is repaired.
	if len(s.Messages) != 7 || len(s.Messages[2].ToolCalls) != 2 {
		t.Error("building the request rewrote the state's own history")
	}
}

// unpaired reports every way messages break the rule each provider enforces
// most strictly: the results of an assistant message's calls come right after
// it, one per call, before anything else is said. Gemini refuses a
// functionResponse turn that does not follow its functionCall turn with as
// many parts, and Anthropic a tool_result not in the very next message — so a
// check by id alone passes histories those providers refuse.
func unpaired(messages []agentloop.Message) []string {
	var problems []string
	for i := 0; i < len(messages); i++ {
		m := messages[i]
		if m.Role == agentloop.RoleTool {
			problems = append(problems, "result "+m.CallID+" at "+strconv.Itoa(i)+" follows no call")
			continue
		}
		if len(m.ToolCalls) == 0 {
			continue
		}
		open := make([]string, 0, len(m.ToolCalls))
		for _, c := range m.ToolCalls {
			open = append(open, c.ID)
		}
		for i+1 < len(messages) && messages[i+1].Role == agentloop.RoleTool {
			i++
			matched := false
			for k, id := range open {
				if id == messages[i].CallID {
					open = append(open[:k], open[k+1:]...)
					matched = true
					break
				}
			}
			if !matched {
				problems = append(problems, "result "+messages[i].CallID+" at "+strconv.Itoa(i)+" answers no call of its step")
			}
		}
		for _, id := range open {
			problems = append(problems, "call "+id+" is not answered in its step")
		}
	}
	return problems
}

// shape draws a history one letter a message — u(ser), a(ssistant),
// C(alling), R(esult) — so a test failure shows what was sent.
func shape(messages []agentloop.Message) string {
	var b strings.Builder
	for _, m := range messages {
		switch {
		case m.Role == agentloop.RoleTool:
			b.WriteByte('R')
		case len(m.ToolCalls) > 0:
			b.WriteByte('C')
		default:
			b.WriteByte(m.Role[0])
		}
	}
	return b.String()
}

func calling(text string, ids ...string) agentloop.Message {
	m := agentloop.Message{Role: agentloop.RoleAssistant, Text: text}
	for _, id := range ids {
		m.ToolCalls = append(m.ToolCalls, agentloop.ToolCall{ID: id, Name: "Read", Input: []byte(`{}`)})
	}
	return m
}

func answering(id string) agentloop.Message {
	return agentloop.Message{Role: agentloop.RoleTool, CallID: id, Name: "Read", Result: []byte(`"ok"`)}
}

func user(text string) agentloop.Message {
	return agentloop.Message{Role: agentloop.RoleUser, Text: text}
}

// Pairing by id alone threw away every step after the first on the providers
// that do not mint unique call ids. Google names a call "<tool>-<position in
// the answer>", and Antigravity does the same when its API gives none, so the
// first Read of every step is "Read-1": keeping only the first occurrence of an
// id sent one call for three results, and Gemini refused the third request of
// every tool-using turn. A call and its result belong to the same step, and
// that is what pairs them.
func TestPairingKeepsEveryStepWhateverTheCallsAreNamed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    []agentloop.Message
		shape string
	}{
		{
			name: "unique ids, as OpenAI and Codex mint them",
			in: []agentloop.Message{user("go"), calling("", "call_a", "call_b"), answering("call_a"), answering("call_b"),
				calling("", "call_c"), answering("call_c"), user("next")},
			shape: "uCRRCRu",
		},
		{
			name: "positional ids repeated across steps, as Google names them",
			in: []agentloop.Message{user("go"), calling("", "Read-1"), answering("Read-1"), calling("", "Read-1", "Read-2"),
				answering("Read-1"), answering("Read-2"), calling("", "Read-1"), answering("Read-1"), user("next")},
			shape: "uCRCRRCRu",
		},
		{
			name: "a stored answer: every step's calls on one message, then every result",
			in: []agentloop.Message{user("go"), calling("done", "Read-1", "Read-1", "Read-1"),
				answering("Read-1"), answering("Read-1"), answering("Read-1"), user("next")},
			shape: "uCRRRu",
		},
		{
			name: "no ids at all",
			in: []agentloop.Message{user("go"), calling("", "", ""), answering(""), answering(""),
				calling("", ""), answering(""), user("next")},
			shape: "uCRRCRu",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := agentloop.PairToolMessages(tc.in)
			if s := shape(got); s != tc.shape {
				t.Errorf("sent %s, want %s", s, tc.shape)
			}
			if problems := unpaired(got); len(problems) > 0 {
				t.Errorf("sent a history a strict provider refuses: %v", problems)
			}
			if calls := len(got[1].ToolCalls); calls != len(tc.in[1].ToolCalls) {
				t.Errorf("the first step kept %d of its %d calls", calls, len(tc.in[1].ToolCalls))
			}
		})
	}
}

// A history already stored broken is repaired step by step: what a step's
// calls did not get an answer for, and what answers no call of its step, are
// not sent — whatever the ids happen to be.
func TestPairingRepairsABrokenHistoryStepByStep(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    []agentloop.Message
		shape string
		calls []int // calls kept on each C, in order
	}{
		{
			// What answerParts stored before calls and results were paired
			// one to one: the repeated id was kept once, its results three
			// times.
			name:  "one call stored for three results with its id",
			in:    []agentloop.Message{user("go"), calling("done", "Read-1"), answering("Read-1"), answering("Read-1"), answering("Read-1"), user("next")},
			shape: "uCRu",
			calls: []int{1},
		},
		{
			// A result is not an answer to a call the model made before
			// somebody spoke: no provider reads it as one.
			name:  "a result after the user spoke",
			in:    []agentloop.Message{user("go"), calling("", "x"), user("wait"), answering("x"), user("next")},
			shape: "uuu",
		},
		{
			// Answers stored before the fix repeated every call the
			// conversation had made; the repeat has no result of its own.
			name: "a call repeated from an earlier answer",
			in: []agentloop.Message{user("go"), calling("read it", "call_x"), answering("call_x"), user("more"),
				calling("again", "call_x", "call_y"), answering("call_y"), user("next")},
			shape: "uCRuCRu",
			calls: []int{1, 1},
		},
		{
			name:  "a result that answers a call made only after it",
			in:    []agentloop.Message{user("go"), answering("late"), calling("", "late"), user("next")},
			shape: "uu",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := agentloop.PairToolMessages(tc.in)
			if s := shape(got); s != tc.shape {
				t.Errorf("sent %s, want %s", s, tc.shape)
			}
			if problems := unpaired(got); len(problems) > 0 {
				t.Errorf("sent a history a strict provider refuses: %v", problems)
			}
			var kept []int
			for _, m := range got {
				if len(m.ToolCalls) > 0 {
					kept = append(kept, len(m.ToolCalls))
				}
			}
			if strconv.Quote(fmtInts(kept)) != strconv.Quote(fmtInts(tc.calls)) {
				t.Errorf("calls kept per step = %v, want %v", kept, tc.calls)
			}
		})
	}
}

func fmtInts(v []int) string {
	out := make([]string, 0, len(v))
	for _, n := range v {
		out = append(out, strconv.Itoa(n))
	}
	return strings.Join(out, ",")
}

// The window a prune keeps widened back to the earliest message offering any
// id a kept result answers. With Google's positional ids that is the first
// step of the turn, whatever its length: nothing was ever pruned, and a long
// tool-using turn on Gemini grew until the context ran out. The window now
// starts on a step's boundary and nowhere earlier.
func TestPruneDropsOldStepsWhenEveryCallHasTheSameID(t *testing.T) {
	full := []agentloop.Message{user("build the API")}
	for range 40 {
		full = append(full, calling("", "Read-1"), answering("Read-1"))
	}
	pruned := agentloop.Prune(full, agentloop.DefaultPolicy())
	if problems := unpaired(pruned); len(problems) > 0 {
		t.Fatalf("the prune left a history a strict provider refuses: %v", problems)
	}
	results := 0
	for _, m := range pruned {
		if m.Role == agentloop.RoleTool {
			results++
		}
	}
	if results == 0 || results > agentloop.KeepToolCalls {
		t.Fatalf("the prune kept %d of 40 results, want the recent ones within %d messages", results, agentloop.KeepToolCalls)
	}
}
