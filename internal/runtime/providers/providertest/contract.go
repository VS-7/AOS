// Package providertest is the executable contract every model provider obeys.
//
// The adapters differ in almost everything — a tool result is a message here, a
// part of a user turn there, a functionResponse somewhere else — and the loop
// is written against none of that. This suite is what says the translation
// works: the same conversation, the same assertions, eight wire formats.
//
// It runs against a recorded exchange rather than the real endpoint. That is a
// real limit and worth stating: it proves the adapter reads what the provider
// documents, not that the provider still sends it.
package providertest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// Exchange is one recorded call: what the provider answers, and what the
// adapter is expected to make of it.
type Exchange struct {
	// Body is the JSON the provider returns to a non-streaming call.
	Body string
	// Stream is the event stream it returns to a streaming one.
	Stream string
}

// Case is one provider under test.
type Case struct {
	Name string

	// Build makes the adapter against the test server's base URL.
	Build func(baseURL string) agentloop.LLMProvider

	// Answer is the recorded reply to a call with no tools: a plain sentence.
	Answer Exchange

	// ToolCall is the recorded reply that asks for a tool.
	ToolCall Exchange

	// WantToolName and WantToolArgs are what the recorded tool call means.
	WantToolName string
	WantToolArgs string
}

// Conversation is the request every case is asked to render: a user turn, an
// assistant that called a tool, and the tool's answer. It exercises all three
// message shapes in one call, which is where the adapters differ most.
func Conversation() agentloop.Request {
	schema, err := jsonschema.For[struct {
		Path string `json:"path" jsonschema:"Which file."`
	}](nil)
	if err != nil {
		panic(err)
	}
	return agentloop.Request{
		Model:        "test-model",
		Instructions: "<context>you are a test</context>",
		Reasoning:    agentloop.ReasoningMedium,
		Tools: []toolexec.Spec{
			{Name: "Read", Description: "Read a file.", InputSchema: schema},
		},
		Messages: []agentloop.Message{
			{Role: agentloop.RoleUser, Text: "what does the readme say"},
			{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{
				{ID: "call-1", Name: "Read", Input: json.RawMessage(`{"path":"README.md"}`)},
			}},
			{Role: agentloop.RoleTool, CallID: "call-1", Name: "Read",
				Result: json.RawMessage(`{"content":"hello"}`)},
		},
	}
}

// Run executes the contract against one provider.
func Run(t *testing.T, c Case) {
	t.Helper()

	t.Run(c.Name+"/an answer comes back whole", func(t *testing.T) {
		server, seen := serve(c.Answer)
		defer server.Close()

		res, err := c.Build(server.URL).Generate(context.Background(), Conversation())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.Message.Text, "hello") {
			t.Fatalf("Text = %q", res.Message.Text)
		}
		if res.StopReason != agentloop.StopEnd {
			t.Errorf("StopReason = %q", res.StopReason)
		}
		if res.Usage.Input == 0 || res.Usage.Output == 0 {
			t.Errorf("the usage was not read: %+v", res.Usage)
		}
		// The whole conversation reached the provider, in whatever shape it
		// takes: the instructions, the tool, and all three messages.
		body := seen()
		for _, fragment := range []string{"you are a test", "Read", "README.md", "hello"} {
			if !strings.Contains(body, fragment) {
				t.Errorf("the request does not carry %q:\n%s", fragment, body)
			}
		}
	})

	t.Run(c.Name+"/a tool call is understood", func(t *testing.T) {
		server, _ := serve(c.ToolCall)
		defer server.Close()

		res, err := c.Build(server.URL).Generate(context.Background(), Conversation())
		if err != nil {
			t.Fatal(err)
		}
		if len(res.ToolCalls) != 1 {
			t.Fatalf("ToolCalls = %+v", res.ToolCalls)
		}
		call := res.ToolCalls[0]
		if call.Name != c.WantToolName {
			t.Errorf("name = %q, want %q", call.Name, c.WantToolName)
		}
		if got := compact(t, call.Input); got != c.WantToolArgs {
			t.Errorf("input = %s, want %s", got, c.WantToolArgs)
		}
		if call.ID == "" {
			t.Error("the call has no id, so its result cannot be paired with it")
		}
		if res.StopReason != agentloop.StopToolCalls {
			t.Errorf("StopReason = %q", res.StopReason)
		}
	})

	if c.Answer.Stream == "" {
		return
	}

	t.Run(c.Name+"/the stream adds up to the answer", func(t *testing.T) {
		server, _ := serve(c.Answer)
		defer server.Close()

		stream, err := c.Build(server.URL).Stream(context.Background(), Conversation())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = stream.Close() }()

		var b strings.Builder
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			b.WriteString(chunk.Text)
		}
		final := stream.Response()
		if !strings.Contains(b.String(), "hello") {
			t.Fatalf("the chunks say %q", b.String())
		}
		// The two must agree. A stream whose pieces differ from the final
		// answer shows the person one thing and records another.
		if strings.TrimSpace(final.Message.Text) != strings.TrimSpace(b.String()) {
			t.Fatalf("the stream said %q and the answer is %q", b.String(), final.Message.Text)
		}
	})

	t.Run(c.Name+"/a refusal is reported with its status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"slow down"}}`))
		}))
		defer server.Close()

		_, err := c.Build(server.URL).Generate(context.Background(), Conversation())
		if err == nil {
			t.Fatal("a 429 was read as an answer")
		}
		if !strings.Contains(err.Error(), "slow down") {
			t.Errorf("the provider's own message was lost: %v", err)
		}
	})
}

// serve answers one recorded exchange and remembers the request body, so the
// suite can assert on what the adapter sent as well as on what it read.
func serve(e Exchange) (*httptest.Server, func() string) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)

		if strings.Contains(r.Header.Get("Accept"), "event-stream") && e.Stream != "" {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(e.Stream))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(e.Body))
	}))
	return server, func() string { return body }
}

func compact(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(out)
}
