// Package agentloop is the reasoning loop: a model call, the tool calls it
// asked for, and again until it stops asking.
//
// The loop is written here rather than taken from a framework because the five
// points where policy intervenes are the identity of this system (ADR-0005).
// The one that matters most is approval: a framework that does not expose
// rewriting a tool payload before execution cannot implement the hook contract
// at all, and the hook contract is how skills extend the agent.
package agentloop

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// Role is who a message is from.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn in the conversation the model sees.
//
// It is flat rather than a list of parts because every provider's wire format
// is flat in the places that matter, and a parts model would be translated
// twice: once into the provider's shape and once back.
type Message struct {
	Role Role   `json:"role"`
	Text string `json:"text,omitempty"`

	// Reasoning is what the model thought, when the provider returns it in the
	// clear. Encrypted is the opaque blob some providers return instead, which
	// is carried between turns and never read.
	Reasoning string `json:"reasoning,omitempty"`
	Encrypted string `json:"encryptedReasoning,omitempty"`

	// ToolCalls is what an assistant message asked for.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`

	// The three fields of a tool message.
	CallID string          `json:"callId,omitempty"`
	Name   string          `json:"name,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`

	At time.Time `json:"at,omitzero"`
}

// ToolCall is the model asking for a tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`

	// Signature is an opaque token a provider attached to this call and
	// requires back, unchanged, when the conversation continues past it —
	// the same arrangement Message.Encrypted describes for reasoning, and
	// carried for the same reason: it is never read here, only returned.
	//
	// Gemini 3 is why it exists. It rejects a follow-up request whose
	// functionCall parts have lost their thought_signature ("required for
	// tools to work correctly"), which makes every tool-using turn fail on
	// its second model call — the one that reads the tool's result.
	Signature string `json:"signature,omitempty"`
}

// ToolResult is what came back.
//
// Denied is separate from Error because they are different facts about the
// world: a denial is a decision somebody made and the model should reason about
// it, and an error is something that went wrong and the model should probably
// try differently.
type ToolResult struct {
	CallID string          `json:"callId"`
	Name   string          `json:"name"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
	Denied bool            `json:"denied,omitempty"`
}

// ReasoningLevel is how hard the model should think.
type ReasoningLevel string

const (
	ReasoningNone   ReasoningLevel = "none"
	ReasoningLow    ReasoningLevel = "low"
	ReasoningMedium ReasoningLevel = "medium"
	ReasoningHigh   ReasoningLevel = "high"
)

// DefaultReasoning is what an installation that said nothing gets.
const DefaultReasoning = ReasoningMedium

// Request is one call to a model.
type Request struct {
	Model        string
	Instructions string // the assembled context document
	Messages     []Message
	Tools        []toolexec.Spec
	Reasoning    ReasoningLevel

	// Options is the provider-specific escape hatch. Anything that belongs to
	// one provider and has no meaning in the others goes here rather than
	// growing this struct a field nobody else can honour.
	Options map[string]any
}

// Usage is what a call cost.
type Usage struct {
	Input     int     `json:"input"`
	Output    int     `json:"output"`
	Reasoning int     `json:"reasoning,omitempty"`
	Cached    int     `json:"cached,omitempty"`
	Total     int     `json:"total"`
	CostUSD   float64 `json:"costUsd,omitempty"`
}

// Add folds one call's usage into a running total.
func (u *Usage) Add(next Usage) {
	u.Input += next.Input
	u.Output += next.Output
	u.Reasoning += next.Reasoning
	u.Cached += next.Cached
	u.Total += next.Total
	u.CostUSD += next.CostUSD
}

// Stop reasons a provider reports.
const (
	StopEnd       = "end"        // the model finished its answer
	StopToolCalls = "tool_calls" // it asked for tools
	StopLength    = "length"     // it ran out of room
	StopFiltered  = "filtered"   // the provider refused
)

// Response is what a model call returned.
type Response struct {
	Message    Message
	ToolCalls  []ToolCall
	Usage      Usage
	StopReason string
	Model      string
}

// Chunk is one piece of a streamed answer.
type Chunk struct {
	Text      string
	Reasoning string
	Done      bool
}

// Stream is a streamed answer. Recv returns io.EOF when the answer is complete,
// after which Response holds the whole of it.
type Stream interface {
	Recv() (Chunk, error)
	Response() Response
	Close() error
}

// LLMProvider is one model provider.
//
// Two methods because a turn either streams to somebody watching or does not,
// and pretending a non-streaming call is a stream of one chunk costs every
// adapter a wrapper that does nothing.
type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (Stream, error)
}

// Optional capabilities. A provider implements what it supports, and the
// two-step check in the registry distinguishes "you did not configure it" from
// "this provider cannot do it" — the original's distinction, kept because the
// two send a person to different places.
type (
	// SpeechProvider turns text into audio.
	SpeechProvider interface {
		Speech(ctx context.Context, text string, voice string) ([]byte, string, error)
	}
	// ImageProvider generates an image.
	ImageProvider interface {
		Image(ctx context.Context, prompt string) ([]byte, string, error)
	}
	// RealtimeProvider mints a token for a browser to open a live session.
	RealtimeProvider interface {
		RealtimeToken(ctx context.Context) (string, error)
	}
)

// Emitter receives what the model produced as it produces it. It is how a chat
// shows an answer being written rather than appearing.
type Emitter interface {
	Delta(ctx context.Context, c Chunk)
}

// ToolWatcher is an Emitter that also wants to see the tools a turn runs, as
// it runs them.
//
// Separate from Emitter, and optional, for the reason the two-method
// LLMProvider split exists: a turn either has somebody watching it work or it
// does not, and an emitter that only wants the answer should not have to
// implement two methods it ignores. The loop checks for it at construction.
//
// It is what lets an interface show "searching memory" while it happens.
// Without it the only live signal a turn produced was streamed text, so every
// tool call — often the slowest and most interesting part of a turn — was
// invisible until the whole thing finished and the transcript was refetched.
type ToolWatcher interface {
	ToolStarted(ctx context.Context, call ToolCall)
	ToolFinished(ctx context.Context, result ToolResult)
}
