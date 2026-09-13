package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
	"github.com/OWNER/aos/internal/runtime/providers/openai"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// The conversation that could never be answered again.
//
// A long tool-using turn compacted its working transcript, and the stored
// answer read its tool calls back out of that transcript while taking its
// results from the loop's own list. The earliest calls were pruned, their
// results were not, and every later message in the chat was refused by the
// Responses API with "No tool call found for function call output". These
// tests follow the whole path: the turn, what is stored, what the next turn
// reads, and the body that reaches the provider.

// bodyOfNextTurn replays a stored conversation plus a new question through the
// real openai adapter and returns the input items it sent.
func bodyOfNextTurn(t *testing.T, stored *chat.Chat) []map[string]any {
	t.Helper()
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &sent)
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"output\":[],\"status\":\"completed\"}}\n\n")
	}))
	t.Cleanup(server.Close)

	messages := append(transcript(stored), agentloop.Message{Role: agentloop.RoleUser, Text: "and now?"})
	// Pruned the way the next turn's loop would prune it, so a repair that only
	// survives an uncompacted history does not pass.
	state := &agentloop.State{Model: "gpt-5.5", Messages: agentloop.Prune(messages, agentloop.DefaultPolicy())}

	stream, err := openai.New("openai", providers.Config{APIKey: "k", BaseURL: server.URL}, nil).
		Stream(context.Background(), state.Request())
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := stream.Recv(); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	_ = stream.Close()

	raw, _ := json.Marshal(sent["input"])
	var items []map[string]any
	_ = json.Unmarshal(raw, &items)
	return items
}

// orphansIn counts what the Responses API refuses: an output with no call
// before it, and a call with no output after it.
func orphansIn(items []map[string]any) (outputs, calls int) {
	offered, answered := map[any]bool{}, map[any]bool{}
	for _, item := range items {
		switch item["type"] {
		case "function_call":
			offered[item["call_id"]] = true
		case "function_call_output":
			if !offered[item["call_id"]] {
				outputs++
			}
			answered[item["call_id"]] = true
		}
	}
	for id := range offered {
		if !answered[id] {
			calls++
		}
	}
	return outputs, calls
}

func TestACompactedTurnIsStoredSoTheNextTurnCanBeSent(t *testing.T) {
	const steps = 20
	script := make([]fake.Step, 0, steps+1)
	for i := range steps {
		script = append(script, fake.Step{Calls: []agentloop.ToolCall{
			fake.Call("call_"+strconv.Itoa(i), "Read", map[string]any{"n": i}),
		}})
	}
	script = append(script, fake.Step{Text: "the library API is ready"})

	big := strings.Repeat("x", 9_000)
	registry := toolexec.NewRegistry().Add(toolexec.Func{
		Definition: toolexec.Spec{Name: "Read", Description: "reads"},
		Fn:         func(context.Context, json.RawMessage) (any, error) { return big, nil },
	})
	loop := agentloop.New(agentloop.Deps{
		Provider: &fake.Provider{Script: script},
		Tools:    registry,
		Compact:  &agentloop.Compactor{Threshold: 40_000, Policy: agentloop.DefaultPolicy()},
		Clock:    &clockx.Stepping{At: at, Step: time.Second},
		Log:      discardLogger(),
	})
	result, err := loop.Run(context.Background(), &agentloop.State{
		Messages: []agentloop.Message{{Role: agentloop.RoleUser, Text: "build a library API"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Compactions == 0 {
		t.Fatal("the turn never compacted, so it proves nothing")
	}

	parts := answerParts(result, nil)
	calls, results := map[string]bool{}, 0
	for _, p := range parts {
		switch p.Type {
		case chat.PartToolCall:
			calls[p.ToolCallID] = true
		case chat.PartToolResult:
			results++
			if !calls[p.ToolCallID] {
				t.Errorf("stored result %s has no stored call", p.ToolCallID)
			}
		}
	}
	if len(calls) != steps || results != steps {
		t.Errorf("stored %d calls and %d results, want %d of each", len(calls), results, steps)
	}

	stored := &chat.Chat{Messages: []chat.Message{
		{Role: chat.RoleUser, Parts: []chat.Part{{Type: chat.PartText, Text: "build a library API"}}, CreatedAt: at},
		{Role: chat.RoleAssistant, Parts: parts, CreatedAt: at.Add(time.Minute)},
	}}
	if outputs, unanswered := orphansIn(bodyOfNextTurn(t, stored)); outputs != 0 || unanswered != 0 {
		t.Fatalf("the next turn sent %d outputs with no call and %d calls with no output", outputs, unanswered)
	}
}

// A chat already stored broken — the user's Luara DM holds 22 calls and 39
// results on one message — must be answerable again, not only chats written
// after the fix.
func TestAConversationStoredWithOrphanResultsCanBeAnsweredAgain(t *testing.T) {
	parts := []chat.Part{{Type: chat.PartText, Text: "the project and the task are ready"}}
	for i := range 39 {
		id := "call_" + strconv.Itoa(i)
		if i >= 17 {
			parts = append(parts, chat.Part{Type: chat.PartToolCall, ToolName: "Read", ToolCallID: id, Input: json.RawMessage(`{}`)})
		}
	}
	for i := range 39 {
		parts = append(parts, chat.Part{Type: chat.PartToolResult, ToolName: "Read", ToolCallID: "call_" + strconv.Itoa(i), Output: json.RawMessage(`"ok"`)})
	}
	stored := &chat.Chat{Messages: []chat.Message{
		{Role: chat.RoleUser, Parts: []chat.Part{{Type: chat.PartText, Text: "build a library API"}}, CreatedAt: at},
		{Role: chat.RoleAssistant, Parts: parts, CreatedAt: at.Add(time.Minute)},
	}}

	items := bodyOfNextTurn(t, stored)
	if outputs, unanswered := orphansIn(items); outputs != 0 || unanswered != 0 {
		t.Fatalf("the replay sent %d outputs with no call and %d calls with no output", outputs, unanswered)
	}
	kept := 0
	for _, item := range items {
		if item["type"] == "function_call_output" {
			kept++
		}
	}
	if kept != 22 {
		t.Errorf("the replay kept %d answered exchanges, want the 22 that were whole", kept)
	}
}
