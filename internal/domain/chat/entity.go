// Package chat is the conversation: the front door to execution rather than a
// dead-end text box.
//
// A chat is the one core entity stored as JSON instead of Markdown, and the
// reason is structural: a conversation has no prose body with metadata above
// it — the messages *are* the structure. The same shape is reused for the
// transcript of an autonomous run, which is why a task run and a chat are the
// same thing in two places.
package chat

import (
	"encoding/json"
	"strings"
	"time"
)

// Kind is the surface a conversation belongs to.
type Kind string

const (
	KindDM       Kind = "dm"       // one human and one counterpart
	KindChannel  Kind = "channel"  // a room in the workspace
	KindTask     Kind = "task"     // a thread attached to a task
	KindRun      Kind = "run"      // the transcript of an autonomous execution
	KindExternal Kind = "external" // bound to a messenger outside the system
)

// Kinds lists every surface.
var Kinds = []Kind{KindDM, KindChannel, KindTask, KindRun, KindExternal}

// Valid reports whether k is one of them.
func (k Kind) Valid() bool {
	for _, known := range Kinds {
		if k == known {
			return true
		}
	}
	return false
}

// Visibility is who may read a conversation.
type Visibility string

const (
	// VisibilityPrivate means only the participants.
	VisibilityPrivate Visibility = "private"
	// VisibilityWorkspace means anyone in the workspace.
	VisibilityWorkspace Visibility = "workspace"
)

// ActorType distinguishes the two kinds of participant.
type ActorType string

const (
	ActorUser  ActorType = "user"
	ActorAgent ActorType = "agent"
)

// Role is the conversational role of a message, as the model sees it.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// PartType is the kind of one piece of a message.
type PartType string

const (
	PartText       PartType = "text"
	PartReasoning  PartType = "reasoning"
	PartToolCall   PartType = "tool-call"
	PartToolResult PartType = "tool-result"
	PartFile       PartType = "file"
)

// Status is where one execution attempt stands.
type Status string

const (
	StatusPending     Status = "pending"
	StatusRunning     Status = "running"
	StatusCompleted   Status = "completed"
	StatusError       Status = "error"
	StatusInterrupted Status = "interrupted"
)

// Chat is one conversation, stored whole at .aos/chats/{id}.chat.json.
type Chat struct {
	ID string `json:"id" collection:"path" jsonschema:"Identifier of the conversation."`

	// Key is the stable lookup key of a private direct message, so that
	// find-or-create does not have to scan every conversation to discover that
	// two people already have one.
	Key string `json:"key,omitempty" jsonschema:"Stable lookup key for a private direct message."`

	Title      string     `json:"title" jsonschema:"What the conversation is called."`
	Kind       Kind       `json:"kind" jsonschema:"One of: dm, channel, task, run, external."`
	Visibility Visibility `json:"visibility" jsonschema:"private restricts to the participants; workspace opens it to every member."`

	Participants []Participant `json:"participants" jsonschema:"Who is in the conversation. Empty on a workspace-visible channel means everyone."`

	Task    string       `json:"task,omitempty" jsonschema:"Task this conversation belongs to, when it is a task thread."`
	Routine string       `json:"routine,omitempty" jsonschema:"Routine this conversation belongs to, when it is a run."`
	Agent   string       `json:"agent,omitempty" jsonschema:"Agent that owns this conversation, for run transcripts."`
	Channel *ChannelMeta `json:"channel,omitempty" jsonschema:"External messenger this conversation is bound to."`

	Messages []Message `json:"messages" jsonschema:"The conversation, in order."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When the conversation started."`
	UpdatedAt time.Time `json:"updatedAt" jsonschema:"When it last changed."`
}

// Participant is one member of a conversation.
type Participant struct {
	Type     ActorType `json:"type" jsonschema:"user or agent."`
	ID       string    `json:"id" jsonschema:"User identifier or agent slug."`
	Role     string    `json:"role,omitempty" jsonschema:"member or admin. Defaults to member."`
	JoinedAt time.Time `json:"joinedAt" jsonschema:"When they joined."`
}

// ChannelMeta binds a conversation to a messenger outside the system.
type ChannelMeta struct {
	Provider string `json:"provider" jsonschema:"Messaging provider slug. Example: \"telegram\"."`
	ChatID   string `json:"chatId" jsonschema:"Conversation identifier on the provider."`
	UserID   string `json:"userId,omitempty" jsonschema:"User identifier on the provider."`
}

// Message is one turn.
type Message struct {
	ID     string  `json:"id" jsonschema:"Identifier of the message."`
	Role   Role    `json:"role" jsonschema:"One of: system, user, assistant, tool."`
	Author *Author `json:"author,omitempty" jsonschema:"Who wrote it."`
	Parts  []Part  `json:"parts" jsonschema:"The content, in order."`

	Reactions []Reaction `json:"reactions,omitempty" jsonschema:"Reactions applied to this message."`

	// Runs records every attempt to answer this message: status, token usage
	// and error. Cost and failure are audited per message rather than per
	// conversation, because that is the granularity at which a person asks
	// "why did this get expensive".
	Runs []Run `json:"runs,omitempty" jsonschema:"Execution attempts triggered by this message."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When the message was written."`
}

// Author is who wrote a message.
type Author struct {
	Type ActorType `json:"type" jsonschema:"user or agent."`
	ID   string    `json:"id" jsonschema:"User identifier or agent slug."`
}

// Part is one piece of a message: text, reasoning, a tool call, its result, or
// a file.
//
// The original relaxes the persisted schema to `any` because the SDK types it
// carries cannot be expressed in JSON Schema. That constraint does not exist
// here, so the type is kept on both sides — one flat struct with a
// discriminator, which round-trips without a custom unmarshaler and can be
// validated on read.
type Part struct {
	Type PartType `json:"type" jsonschema:"One of: text, reasoning, tool-call, tool-result, file."`

	Text string `json:"text,omitempty" jsonschema:"The text, for a text or reasoning part."`

	ToolName   string          `json:"toolName,omitempty" jsonschema:"Tool invoked, for a tool-call or tool-result part."`
	ToolCallID string          `json:"toolCallId,omitempty" jsonschema:"Identifier pairing a call with its result."`
	Input      json.RawMessage `json:"input,omitempty" jsonschema:"Arguments the tool was called with."`
	Output     json.RawMessage `json:"output,omitempty" jsonschema:"What the tool returned."`

	MediaType string `json:"mediaType,omitempty" jsonschema:"MIME type, for a file part."`
	URI       string `json:"uri,omitempty" jsonschema:"Where the file is."`
}

// Reaction is an emoji applied to a message.
type Reaction struct {
	Actor     string    `json:"actor" jsonschema:"Who reacted."`
	Value     string    `json:"value" jsonschema:"The reaction, usually an emoji."`
	Timestamp time.Time `json:"timestamp" jsonschema:"When."`
}

// Run is one attempt to answer a message.
type Run struct {
	AgentID     string     `json:"agentId" jsonschema:"Agent that was asked to answer."`
	JobID       string     `json:"jobId,omitempty" jsonschema:"Queue job that carried the attempt."`
	Status      Status     `json:"status" jsonschema:"pending, running, completed, error or interrupted."`
	Usage       TokenUsage `json:"usage" jsonschema:"Tokens consumed by the attempt."`
	Error       *RunError  `json:"error,omitempty" jsonschema:"Why it failed, when it did."`
	StartedAt   time.Time  `json:"startedAt" jsonschema:"When the attempt began."`
	CompletedAt *time.Time `json:"completedAt,omitempty" jsonschema:"When it reached a terminal state."`
}

// TokenUsage is what one attempt cost.
//
// CostUSD is an addition: the original records tokens and stops there, which
// means the person can see the volume and not the bill.
type TokenUsage struct {
	Input     int     `json:"input" jsonschema:"Prompt tokens."`
	Output    int     `json:"output" jsonschema:"Completion tokens."`
	Reasoning int     `json:"reasoning,omitempty" jsonschema:"Reasoning tokens, when the model reports them separately."`
	Cached    int     `json:"cached,omitempty" jsonschema:"Prompt tokens served from cache."`
	Total     int     `json:"total" jsonschema:"Total tokens."`
	CostUSD   float64 `json:"costUsd,omitempty" jsonschema:"Estimated cost in US dollars."`
}

// RunError is a failure, in the form a caller can act on.
type RunError struct {
	Code    string `json:"code" jsonschema:"Stable error code."`
	Message string `json:"message" jsonschema:"What went wrong."`
}

// AgentParticipants returns the agents in the conversation.
func (c Chat) AgentParticipants() []string {
	var out []string
	for _, p := range c.Participants {
		if p.Type == ActorAgent {
			out = append(out, p.ID)
		}
	}
	return out
}

// HasParticipant reports whether an actor is in the conversation.
func (c Chat) HasParticipant(kind ActorType, id string) bool {
	for _, p := range c.Participants {
		if p.Type == kind && p.ID == id {
			return true
		}
	}
	return false
}

// Text concatenates the text parts of a message, which is what the routing
// looks at for mentions.
func (m Message) Text() string {
	var b strings.Builder
	for _, p := range m.Parts {
		if p.Type == PartText && p.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// TotalUsage sums the token usage of every attempt on this message.
func (m Message) TotalUsage() TokenUsage {
	var total TokenUsage
	for _, r := range m.Runs {
		total.Input += r.Usage.Input
		total.Output += r.Usage.Output
		total.Reasoning += r.Usage.Reasoning
		total.Cached += r.Usage.Cached
		total.Total += r.Usage.Total
		total.CostUSD += r.Usage.CostUSD
	}
	return total
}
