package agentloop

import (
	"encoding/json"
	"time"

	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// State is one turn in progress.
//
// It is passed to every hook by pointer, which is how a hook injects context:
// it appends to Pending, and the next model call carries it. That is the same
// mechanism the original uses and the reason the hook interface takes a state
// rather than a copy of the messages.
type State struct {
	SessionID string
	AgentID   string
	Workspace string

	// Instructions is the assembled context document. A hook may extend it
	// through Pending rather than by rewriting it: the document declares the
	// authority of each of its blocks, and text spliced into it would inherit
	// an authority nobody granted.
	Instructions string

	Messages []Message
	Tools    []toolexec.Spec

	Model     string
	Reasoning ReasoningLevel

	// Pending is context a hook asked to add, waiting to be attached to the
	// next model call.
	Pending []string

	// Steps counts model calls made in this turn.
	Steps int

	// Usage accumulates across the turn.
	Usage Usage

	// Compactions counts how many times the history was pruned, which is what
	// makes "the agent forgot" answerable after the fact.
	Compactions int

	// Started is when the turn began.
	Started time.Time
}

// Append adds a message.
func (s *State) Append(m Message) { s.Messages = append(s.Messages, m) }

// AppendContext queues text for the next model call.
func (s *State) AppendContext(text string) {
	if text == "" {
		return
	}
	s.Pending = append(s.Pending, text)
}

// AppendToolResults turns results into the tool messages the model reads next.
func (s *State) AppendToolResults(results []ToolResult, now time.Time) {
	for _, r := range results {
		payload := r.Output
		switch {
		case r.Denied:
			payload = mustJSON(map[string]any{
				"denied": true,
				"reason": r.Error,
			})
		case r.Error != "":
			payload = mustJSON(map[string]any{
				"error": r.Error,
			})
		}
		s.Append(Message{
			Role: RoleTool, CallID: r.CallID, Name: r.Name,
			Result: payload, At: now,
		})
	}
}

// Request builds the next model call.
//
// Pending context is drained into a user message rather than into the
// instructions, and the difference is the trust model: the context document
// declares the authority of every block it contains, and text appended to a
// turn is a turn, not policy.
func (s *State) Request() Request {
	// The slice is copied rather than shared. Appending the pending context to
	// s.Messages directly would write into its spare capacity, and the next
	// s.Append would then overwrite the message the provider was handed — a
	// provider that retries, or that reads the request after returning, would
	// see a conversation that changed underneath it.
	messages := make([]Message, 0, len(s.Messages)+len(s.Pending))
	messages = append(messages, s.Messages...)
	for _, text := range s.Pending {
		messages = append(messages, Message{Role: RoleUser, Text: text})
	}
	return Request{
		Model:        s.Model,
		Instructions: s.Instructions,
		Messages:     messages,
		Tools:        s.Tools,
		Reasoning:    s.Reasoning,
	}
}

// DrainPending clears the queued context after a call consumed it.
func (s *State) DrainPending() { s.Pending = nil }

// Chars is the size of the history, which is what the compactor decides on.
func (s *State) Chars() int {
	var n int
	for _, m := range s.Messages {
		n += len(m.Text) + len(m.Reasoning) + len(m.Encrypted) + len(m.Result)
		for _, c := range m.ToolCalls {
			n += len(c.Name) + len(c.Input)
		}
	}
	return n
}

// Result is what a finished turn produced.
type Result struct {
	// Text is the answer, which is the text of the last assistant message.
	Text string

	Messages    []Message
	Usage       Usage
	Steps       int
	Compactions int
	StopReason  string

	// ToolCalls is every call made in the turn, in order, with what it
	// returned. It is what an activity feed and a task comment are written
	// from, and what makes a run auditable.
	ToolCalls []ToolResult

	Duration time.Duration
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		// The only values passed here are maps of strings and booleans built
		// two lines above the call.
		return json.RawMessage(`{"error":"the result could not be encoded"}`)
	}
	return raw
}
