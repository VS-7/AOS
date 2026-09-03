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

	// Active is who is working, by conversation.
	//
	// It travels beside the roster rather than on the record because it is
	// not part of one: a Chat is stored as JSON, so a field on it would be
	// persisted, and this is a fact about right now.
	//
	// It exists because the interface's "Atlas is working…" was realtime and
	// nothing else — the indicator was seeded empty on every load, so
	// reloading the window during a turn made the agent look idle, and a
	// window opened while a turn was already running never showed it at all.
	Active map[string][]string `json:"active,omitempty" jsonschema:"Agents with a turn in flight, by conversation id."`
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

// UpdateInput renames a conversation, or changes who can read it.
//
// Only what is named is changed. A rename must not silently reopen a private
// conversation, and opening one must not rename it — which is why these are
// two optional fields rather than a whole Chat the caller sends back.
type UpdateInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Conversation to change." validate:"required,notblank"`

	Title      string     `json:"title,omitempty" jsonschema:"New name. Leave empty to keep the current one."`
	Visibility Visibility `json:"visibility,omitempty" jsonschema:"private restricts to the participants; workspace opens it to every member. Leave empty to keep the current one."`

	command.Reasoning
}

// ClearInput empties a conversation's transcript, keeping the conversation.
//
// It is not a variant of Update. Update changes what a conversation is called
// and who can read it; this throws away what was said in it. Folding the second
// into the first would mean a rename could drop a transcript by sending one
// field too many, which is exactly what the interface was doing before this
// existed.
type ClearInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Conversation to empty." validate:"required,notblank"`

	command.Reasoning
}

// ClearOutput reports how much was thrown away.
type ClearOutput struct {
	Chat    string `json:"chat" jsonschema:"Identifier of the conversation."`
	Removed int    `json:"removed" jsonschema:"How many messages were removed."`
}

// DeleteInput removes a conversation and its transcript.
type DeleteInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Conversation to remove." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	Chat    string `json:"chat" jsonschema:"Identifier of the conversation."`
	Deleted bool   `json:"deleted" jsonschema:"True when it was removed."`
}

// SendInput appends a message and dispatches the answer.
type SendInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Conversation to write to." validate:"required,notblank"`
	Text string `json:"text" jsonschema:"What to say. Address a specific agent with @slug." validate:"required,notblank"`

	// Agent overrides the routing. It exists because a caller sometimes knows
	// exactly who should answer and should not have to encode that as a mention
	// inside prose a person will read.
	Agent string `json:"agent,omitempty" jsonschema:"Answer with this agent regardless of what the text mentions."`

	// Context is what was attached to the message rather than typed into it:
	// the workspace instructions the composer pulls in, the content of a file
	// somebody referenced.
	//
	// It exists because there was nowhere else to put it. The interface
	// prepended each of these to Text, so a message whose first part was an
	// attached instruction had the whole of the user's own words swallowed by
	// it — and the renderer, which hides a text part beginning with
	// "[system-reminder]:", then hid the entire message the moment the daemon
	// confirmed it. What a person wrote and what was attached for the model to
	// read are two things, and they are stored as two things.
	Context []string `json:"context,omitempty" jsonschema:"Material attached to the message for the agent to read, kept apart from what the person typed."`

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

// StopInput names the conversation whose turn to end.
type StopInput struct {
	Chat string `json:"chat" cli:"arg" jsonschema:"Conversation whose running turn should stop." validate:"required,notblank"`

	command.Reasoning
}

// StopOutput says whether there was a turn to stop.
//
// `false` is an answer, not a failure: a person presses the button when they
// see an agent working, and the turn may finish on its own before the call
// lands.
type StopOutput struct {
	Chat    string `json:"chat" jsonschema:"The conversation."`
	Stopped bool   `json:"stopped" jsonschema:"True when a turn was running and has been asked to stop."`
}

// ReactInput toggles one reaction on one message.
//
// It carries no actor: who is reacting comes from the ambient identity, and a
// caller that could name it could react as somebody else.
type ReactInput struct {
	Chat    string `json:"chat" cli:"arg" jsonschema:"Conversation the message belongs to." validate:"required,notblank"`
	Message string `json:"message" jsonschema:"Message to react to." validate:"required,notblank"`
	Value   string `json:"value" jsonschema:"The reaction, usually an emoji. Sending the same one again removes it." validate:"required,notblank"`

	command.Reasoning
}

// MarkRunInput records that a turn has begun on the message that asked for it.
//
// The run itself is written when the turn *ends* (Reply), which left nothing
// on the record between "sent" and "answered": the interface could only know
// an agent was working from a realtime event it might have missed, or been
// opened after.
type MarkRunInput struct {
	Chat    string
	Message string
	AgentID string
	JobID   string
}
