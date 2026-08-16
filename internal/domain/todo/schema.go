package todo

import "github.com/OWNER/aos/internal/core/command"

// ListInput reads one task's plan.
type ListInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`

	command.Reasoning
}

// ListOutput is the plan with its completion.
type ListOutput struct {
	Todos    []Todo   `json:"todos" jsonschema:"The steps, in plan order."`
	Total    int      `json:"total" jsonschema:"How many steps the plan has."`
	Progress Progress `json:"progress" jsonschema:"How much of it is done."`
}

// GetInput names one step. Every todo command carries the parent, because a
// step identifier means nothing without the task it belongs to.
type GetInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier of the step." validate:"required,notblank"`

	command.Reasoning
}

// CreateInput adds a step.
type CreateInput struct {
	Task  string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	Title string `json:"title" cli:"arg" jsonschema:"What this step is. Example: \"Reproduce the failure in a test\"." validate:"required,notblank"`

	Order   int    `json:"order,omitempty" cli:"flag" jsonschema:"Position in the plan. Left out, the step goes at the end."`
	Content string `json:"content,omitempty" jsonschema:"Notes on this step, in Markdown."`

	command.Reasoning
}

// UpdateInput changes a step's description.
//
// Status is present and always rejected. A field that silently did nothing
// would be worse: a model would write it, see success, and believe the step had
// moved.
type UpdateInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier of the step." validate:"required,notblank"`

	Title    *string `json:"title,omitempty" cli:"flag" jsonschema:"New title."`
	Order    *int    `json:"order,omitempty" cli:"flag" jsonschema:"New position in the plan."`
	Evidence *string `json:"evidence,omitempty" cli:"flag" jsonschema:"What you verified, concretely."`
	Content  *string `json:"content,omitempty" jsonschema:"New notes, in Markdown."`

	Status Status `json:"status,omitempty" cli:"flag" jsonschema:"Not writable here. Use set-status, which validates the move."`

	command.Reasoning
}

// SetStatusInput moves one step.
type SetStatusInput struct {
	Task   string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID     string `json:"id" cli:"arg" jsonschema:"Identifier of the step." validate:"required,notblank"`
	Status Status `json:"status" cli:"arg" jsonschema:"One of: pending, in_progress, blocked, finished, skipped." validate:"required,notblank"`

	Evidence string `json:"evidence,omitempty" cli:"flag" jsonschema:"What you verified. Record it when finishing a step that was verifiable."`

	command.Reasoning
}

// SetStatusOutput reports the move and, when finishing without evidence, says so.
type SetStatusOutput struct {
	Todo *Todo  `json:"todo" jsonschema:"The step after the move."`
	From Status `json:"from" jsonschema:"Where it was."`
	To   Status `json:"to" jsonschema:"Where it is now."`

	// Warning is advisory, not an error. It is the one place the system tells
	// an agent it did something legal and thin.
	Warning string `json:"warning,omitempty" jsonschema:"Advice about what was just recorded. Not a failure."`
}

// DeleteInput removes a step.
type DeleteInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier of the step." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput names what went.
type DeleteOutput struct {
	ID   string `json:"id" jsonschema:"The step that was removed."`
	Task string `json:"task" jsonschema:"The task it belonged to."`
}
