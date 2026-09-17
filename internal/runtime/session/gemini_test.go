package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/google"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// strictGemini stands in for the Gemini API where it is strict about tool
// turns, and refuses a request the way it does:
//
//   - a functionResponse turn must come right after a functionCall turn, with
//     one response part for every call part of that turn;
//   - a functionCall turn must come right after a user turn or a function
//     response turn.
//
// Asked after the last question, it calls Read twice per step until it has
// made `steps` steps, then answers in text.
type strictGemini struct {
	steps int

	mu       sync.Mutex
	requests int
	refused  []string
}

type geminiContent struct {
	Role  string                       `json:"role"`
	Parts []map[string]json.RawMessage `json:"parts"`
}

func (g *strictGemini) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Contents []geminiContent `json:"contents"`
	}
	raw, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(raw, &body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	g.mu.Lock()
	g.requests++
	problem := geminiRefusal(body.Contents)
	if problem != "" {
		g.refused = append(g.refused, problem)
	}
	g.mu.Unlock()
	if problem != "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"error":{"code":400,"message":%q,"status":"INVALID_ARGUMENT"}}`, problem)
		return
	}

	// Steps since the question. Pruning drops the early ones, so the count
	// rides in the arguments of the newest call, which a prune always keeps.
	made := 0
	for _, c := range body.Contents {
		if c.Role == "user" && countParts(c, "functionResponse") == 0 {
			made = 0
		}
		for _, p := range c.Parts {
			var call struct {
				Args struct {
					Step int `json:"step"`
				} `json:"args"`
			}
			if raw, ok := p["functionCall"]; ok && json.Unmarshal(raw, &call) == nil {
				made = call.Args.Step + 1
			}
		}
	}
	answer := `{"candidates":[{"content":{"role":"model","parts":[{"text":"done"}]},"finishReason":"STOP"}]}`
	if made < g.steps {
		answer = fmt.Sprintf(`{"candidates":[{"content":{"role":"model","parts":[`+
			`{"functionCall":{"name":"Read","args":{"step":%d}},"thoughtSignature":"sig-%d"},`+
			`{"functionCall":{"name":"Read","args":{"step":%d,"second":true}}}]},"finishReason":"STOP"}]}`, made, made, made)
	}
	if strings.Contains(r.URL.String(), "alt=sse") {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, "data: "+answer+"\n\n")
		return
	}
	w.Header().Set("content-type", "application/json")
	_, _ = io.WriteString(w, answer)
}

func countParts(c geminiContent, kind string) int {
	n := 0
	for _, p := range c.Parts {
		if _, ok := p[kind]; ok {
			n++
		}
	}
	return n
}

func geminiRefusal(contents []geminiContent) string {
	for i, c := range contents {
		responses, calls := countParts(c, "functionResponse"), countParts(c, "functionCall")
		var prev geminiContent
		if i > 0 {
			prev = contents[i-1]
		}
		if responses > 0 {
			if prev.Role != "model" || countParts(prev, "functionCall") == 0 {
				return "Please ensure that function response turn comes immediately after a function call turn."
			}
			if responses != countParts(prev, "functionCall") {
				return "Please ensure that the number of function response parts is equal to the number of function call parts of the function call turn."
			}
		}
		if calls > 0 && (i == 0 || prev.Role != "user") {
			return "Please ensure that function call turn comes immediately after a user turn or after a function response turn."
		}
	}
	return ""
}

func (g *strictGemini) refusals() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.refused...)
}

// replayToGemini sends a stored conversation plus a new question through the
// real google adapter to backend, pruned the way the next turn's loop prunes
// it.
func replayToGemini(t *testing.T, backend *httptest.Server, stored *chat.Chat) error {
	t.Helper()
	messages := append(transcript(stored, speaker{self: stored.Agent}), agentloop.Message{Role: agentloop.RoleUser, Text: "and now?"})
	state := &agentloop.State{Model: "gemini-3-pro", Messages: agentloop.Prune(messages, agentloop.DefaultPolicy())}
	_, err := google.New("google", providers.Config{APIKey: "k", BaseURL: backend.URL}, nil).
		Generate(context.Background(), state.Request())
	return err
}

// A multi-step Google turn, long enough to compact, against a backend as strict
// as Gemini: every request of the turn is accepted, the answer is stored with
// every call beside its result, and the next turn of the conversation is
// accepted too. Two calls a step is the parallel case, whose results Gemini
// takes as one turn with one part per call.
func TestAGoogleTurnIsAcceptedStepAfterStepAndAgainNextTurn(t *testing.T) {
	const steps = 12
	gemini := &strictGemini{steps: steps}
	backend := httptest.NewServer(gemini)
	t.Cleanup(backend.Close)

	big := strings.Repeat("x", 4_000)
	loop := agentloop.New(agentloop.Deps{
		Provider: google.New("google", providers.Config{APIKey: "k", BaseURL: backend.URL}, nil),
		Tools: toolexec.NewRegistry().Add(toolexec.Func{
			Definition: toolexec.Spec{Name: "Read", Description: "reads"},
			Fn:         func(context.Context, json.RawMessage) (any, error) { return big, nil },
		}),
		Compact: &agentloop.Compactor{Threshold: 40_000, Policy: agentloop.DefaultPolicy()},
		Clock:   &clockx.Stepping{At: at, Step: time.Second},
		Log:     discardLogger(),
	})
	result, err := loop.Run(context.Background(), &agentloop.State{
		Model:    "gemini-3-pro",
		Messages: []agentloop.Message{{Role: agentloop.RoleUser, Text: "build a library API"}},
	})
	if err != nil {
		t.Fatalf("the turn failed: %v (refused: %v)", err, gemini.refusals())
	}
	if result.Compactions == 0 {
		t.Fatal("the turn never compacted, so it proves nothing")
	}

	parts := answerParts(result, nil)
	calls, results := 0, 0
	for _, p := range parts {
		switch p.Type {
		case chat.PartToolCall:
			calls++
		case chat.PartToolResult:
			results++
		}
	}
	if calls != 2*steps || results != 2*steps {
		t.Errorf("stored %d calls and %d results, want %d of each", calls, results, 2*steps)
	}

	stored := &chat.Chat{Messages: []chat.Message{
		{Role: chat.RoleUser, Parts: []chat.Part{{Type: chat.PartText, Text: "build a library API"}}, CreatedAt: at},
		{Role: chat.RoleAssistant, Parts: parts, CreatedAt: at.Add(time.Minute)},
	}}
	if err := replayToGemini(t, backend, stored); err != nil {
		t.Fatalf("the next turn was refused: %v", err)
	}
}

// A conversation already stored is sent in a shape Gemini accepts, keeping
// every exchange that is whole: one stored with Gemini's positional ids —
// "Read-1" once per step — and one stored broken before calls were kept beside
// their results, like the Luara DM with 22 calls and 39 results on one answer.
func TestAStoredConversationIsSentToGeminiWithEveryWholeExchange(t *testing.T) {
	question := chat.Message{Role: chat.RoleUser, Parts: []chat.Part{{Type: chat.PartText, Text: "build a library API"}}, CreatedAt: at}
	repeated := []chat.Part{{Type: chat.PartText, Text: "the API is ready"}}
	for i := range 3 {
		repeated = append(repeated, chat.Part{Type: chat.PartToolCall, ToolName: "Read", ToolCallID: "Read-1", Input: json.RawMessage(`{"n":` + strconv.Itoa(i) + `}`)})
	}
	for range 3 {
		repeated = append(repeated, chat.Part{Type: chat.PartToolResult, ToolName: "Read", ToolCallID: "Read-1", Output: json.RawMessage(`"ok"`)})
	}
	orphans := []chat.Part{{Type: chat.PartText, Text: "the project and the task are ready"}}
	for i := 17; i < 39; i++ {
		orphans = append(orphans, chat.Part{Type: chat.PartToolCall, ToolName: "Read", ToolCallID: "call_" + strconv.Itoa(i), Input: json.RawMessage(`{}`)})
	}
	for i := range 39 {
		orphans = append(orphans, chat.Part{Type: chat.PartToolResult, ToolName: "Read", ToolCallID: "call_" + strconv.Itoa(i), Output: json.RawMessage(`"ok"`)})
	}

	for _, tc := range []struct {
		name  string
		parts []chat.Part
		whole int
	}{
		{name: "positional ids repeated across steps", parts: repeated, whole: 3},
		{name: "results stored without their calls", parts: orphans, whole: 22},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sentToGemini(t, &chat.Chat{Messages: []chat.Message{
				question,
				{Role: chat.RoleAssistant, Parts: tc.parts, CreatedAt: at.Add(time.Minute)},
			}}, tc.whole)
		})
	}
}

func sentToGemini(t *testing.T, stored *chat.Chat, whole int) {
	t.Helper()

	var sent struct {
		Contents []geminiContent `json:"contents"`
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &sent)
		if problem := geminiRefusal(sent.Contents); problem != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":{"code":400,"message":%q}}`, problem)
			return
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`)
	}))
	t.Cleanup(backend.Close)

	if err := replayToGemini(t, backend, stored); err != nil {
		t.Fatalf("the stored conversation was refused: %v", err)
	}
	calls, responses := 0, 0
	for _, c := range sent.Contents {
		calls += countParts(c, "functionCall")
		responses += countParts(c, "functionResponse")
	}
	if calls != whole || responses != whole {
		t.Errorf("sent %d calls and %d responses, want the %d of each that were whole", calls, responses, whole)
	}
}

// answerParts stores a call with the result it produced, one to one. It used
// to skip every call whose id it had already stored, so three steps that each
// called "Read-1" were stored as one call and three results.
func TestAnAnswerStoresEveryCallWhenIDsRepeat(t *testing.T) {
	result := &agentloop.Result{
		Calls: []agentloop.ToolCall{
			{ID: "Read-1", Name: "Read", Input: json.RawMessage(`{"n":1}`)},
			{ID: "Read-1", Name: "Read", Input: json.RawMessage(`{"n":2}`)},
			{ID: "", Name: "Glob", Input: json.RawMessage(`{}`)},
		},
		ToolCalls: []agentloop.ToolResult{
			{CallID: "Read-1", Name: "Read", Output: json.RawMessage(`"a"`)},
			{CallID: "Read-1", Name: "Read", Output: json.RawMessage(`"b"`)},
			{CallID: "", Name: "Glob", Output: json.RawMessage(`[]`)},
		},
	}
	var inputs, outputs []string
	for _, p := range answerParts(result, nil) {
		switch p.Type {
		case chat.PartToolCall:
			inputs = append(inputs, string(p.Input))
		case chat.PartToolResult:
			outputs = append(outputs, string(p.Output))
		}
	}
	if strings.Join(inputs, " ") != `{"n":1} {"n":2} {}` || strings.Join(outputs, " ") != `"a" "b" []` {
		t.Errorf("stored calls %v and results %v, want every call with its own result", inputs, outputs)
	}
}
