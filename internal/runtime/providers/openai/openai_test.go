package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
)

// serveSSE stands up an endpoint that replays one recorded event stream.
func serveSSE(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func answerOf(t *testing.T, body string) agentloop.Response {
	t.Helper()
	server := serveSSE(t, body)
	p := New("openai", providers.Config{APIKey: "k", BaseURL: server.URL}, nil)

	stream, err := p.Stream(context.Background(), agentloop.Request{
		Model:    "gpt-5.4",
		Messages: []agentloop.Message{{Role: agentloop.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	for {
		if _, err := stream.Recv(); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("Recv: %v", err)
		}
	}
	return stream.Response()
}

// The Codex backend's shape: the content arrives only through the per-item
// events, and `response.completed` carries an empty `output`.
//
// This one is worth pinning because of how it failed. An answer parsed as
// empty is not a blank reply — agentloop.Result falls back to
// lastAssistantText(), so the turn is stored with the *previous* turn's
// answer. It looks like a reply, it is a completed run, and it is somebody
// else's sentence.
const codexStream = `event: response.output_item.done
data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"JABUTICABA"}]}}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","model":"gpt-5.4","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}

`

func TestAnswerIsAssembledWhenTheCompletedEventCarriesNoOutput(t *testing.T) {
	got := answerOf(t, codexStream)
	if got.Message.Text != "JABUTICABA" {
		t.Fatalf("text = %q, want the answer assembled from the per-item events", got.Message.Text)
	}
	if got.Usage.Total != 12 {
		t.Errorf("usage = %d, want it still read off the completed event", got.Usage.Total)
	}
}

func TestToolCallIsAssembledFromThePerItemEvents(t *testing.T) {
	got := answerOf(t, `event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_1","name":"tasks_list","arguments":"{}"}}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","output":[]}}

`)
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "tasks_list" {
		t.Fatalf("tool calls = %#v, want the one the stream reported", got.ToolCalls)
	}
}

// The public API repeats the whole answer in the completed event. That has to
// keep winning, so a provider that sends both does not get its answer twice.
func TestTheCompletedEventStillWinsWhenItCarriesTheAnswer(t *testing.T) {
	got := answerOf(t, `event: response.output_item.done
data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"from item"}]}}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"from completed"}]}]}}

`)
	if got.Message.Text != "from completed" {
		t.Fatalf("text = %q, want the completed event's own output", got.Message.Text)
	}
}
