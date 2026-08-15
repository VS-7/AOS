package chat

import "github.com/OWNER/aos/internal/core/command"

// ListInput selects conversations.
type ListInput struct {
	Query   string `json:"query,omitempty" jsonschema:"Filter by title, identifier, task or routine."`
	Kind    Kind   `json:"kind,omitempty" jsonschema:"Narrow to one surface: dm, channel, task, run or external."`
	Task    string `json:"task,omitempty" jsonschema:"Only conversations attached to this task."`
	Routine string `json:"routine,omitempty" jsonschema:"Only conversations attached to this routine."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number to return. Defaults to 50."`

	command.Reasoning
}

// ListOutput is the roster of conversations.
//
// The messages are stripped: a listing answers "what conversations are there",
// and returning every transcript to answer it would be the most expensive call
// in the system.
type ListOutput struct {
	Chats []Chat `json:"chats" jsonschema:"The conversations found, most recently updated first, without their messages."`
	Total int    `json:"total" jsonschema:"How many matched."`
}

// GetInput reads one conversation in full.
type GetInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Identifier of the conversation." validate:"required,notblank"`

	command.Reasoning
}

// CreateInput opens a conversation.
type CreateInput struct {
	Title        string        `json:"title" jsonschema:"What to call it." validate:"required,notblank"`
	Kind         Kind          `json:"kind,omitempty" jsonschema:"One of: dm, channel, task, run, external. Defaults to channel."`
	Visibility   Visibility    `json:"visibility,omitempty" jsonschema:"private or workspace. Defaults to workspace."`
	Participants []Participant `json:"participants,omitempty" jsonschema:"Who is in it. Empty on a workspace-visible channel means everyone."`
	Task         string        `json:"task,omitempty" jsonschema:"Task this conversation belongs to."`
	Routine      string        `json:"routine,omitempty" jsonschema:"Routine this conversation belongs to."`
	Agent        string        `json:"agent,omitempty" jsonschema:"Agent that owns this conversation, for run transcripts."`
	Channel      *ChannelMeta  `json:"channel,omitempty" jsonschema:"External messenger to bind it to."`

	command.Reasoning
}

// SendInput appends a message and dispatches the answer.
type SendInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Conversation to write to." validate:"required,notblank"`
	Text string `json:"text" jsonschema:"What to say. Address a specific agent with @slug." validate:"required,notblank"`

	// Agent overrides the routing. It exists because a caller sometimes knows
	// exactly who should answer and should not have to encode that as a mention
	// inside prose a person will read.
	Agent string `json:"agent,omitempty" jsonschema:"Answer with this agent regardless of what the text mentions."`

	command.Reasoning
}

// SendOutput reports what was persisted and who was asked to answer.
type SendOutput struct {
	Message Message `json:"message" jsonschema:"The message as it was stored."`
	Target  Target  `json:"target" jsonschema:"Which agent will answer, and how that was decided."`
	JobID   string  `json:"jobId,omitempty" jsonschema:"Identifier of the queued turn, when the runtime accepted it."`

	// Dispatched is false when the message was stored but no answer was
	// started. That is a real state — there may be no agent to answer, or the
	// runtime may be unavailable — and a caller that assumed otherwise would
	// wait for a reply that is not coming.
	Dispatched bool `json:"dispatched" jsonschema:"True when a turn was handed to the agent runtime."`
}
