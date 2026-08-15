package app

import (
	"encoding/json"

	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/event"
)

// PendingInput takes nothing: the question is always the same one.
type PendingInput struct {
	command.Reasoning
}

// PendingOutput is what is waiting.
type PendingOutput struct {
	Pending []event.ApprovalRequest `json:"pending" jsonschema:"The tool calls waiting for a decision, oldest first."`
	Total   int                     `json:"total" jsonschema:"How many are waiting."`
}

// DecideInput answers one request.
type DecideInput struct {
	ID       string `json:"id" cli:"arg" jsonschema:"Identifier of the waiting request." validate:"required,notblank"`
	Approved bool   `json:"approved" jsonschema:"True to let the call run, false to refuse it."`
	Reason   string `json:"reason,omitempty" jsonschema:"Why. A refusal without one leaves the agent guessing what to try instead."`

	// UpdatedInput is the correction path: approving a call with a different
	// payload than the one the model proposed.
	UpdatedInput json.RawMessage `json:"updatedInput,omitempty" jsonschema:"A corrected payload to run instead of the one proposed."`

	Remember string `json:"remember,omitempty" jsonschema:"none, session or always. Defaults to none — approving once is not approving forever."`

	command.Reasoning
}

// DecideOutput reports whether the answer landed.
type DecideOutput struct {
	ID string `json:"id" jsonschema:"The request that was answered."`

	// Settled is false when nothing was waiting under that identifier, which
	// is what a request that already timed out looks like from here. It must
	// not read as success: the call it belonged to was denied minutes ago.
	Settled bool `json:"settled" jsonschema:"False when nothing was waiting under this identifier — usually because it already expired."`
}
