package task

import "github.com/OWNER/aos/internal/core/command"

// ListInput selects tasks.
type ListInput struct {
	Status   Status `json:"status,omitempty" cli:"flag" jsonschema:"Only tasks in this status."`
	Type     string `json:"type,omitempty" cli:"flag" jsonschema:"Only tasks of this workspace type. Example: \"bug\"."`
	Assigned string `json:"assigned,omitempty" cli:"flag" jsonschema:"Only tasks owned by this agent or user."`
	Project  string `json:"project,omitempty" cli:"flag" jsonschema:"Only tasks in this project."`
	Goal     string `json:"goal,omitempty" cli:"flag" jsonschema:"Only tasks serving this goal."`

	// Query and Priority are what the task list's own search box and
	// priority filter send. They had no field here, so the daemon's decoder
	// dropped them and answered the unfiltered list: typing in the search
	// box changed nothing, and neither did picking a priority.
	Query    string   `json:"query,omitempty" cli:"flag" jsonschema:"Substring match over name and summary."`
	Priority Priority `json:"priority,omitempty" cli:"flag" jsonschema:"Only tasks at this priority."`

	Limit  int `json:"limit,omitempty" cli:"flag" jsonschema:"How many to return."`
	Offset int `json:"offset,omitempty" cli:"flag" jsonschema:"How many to skip."`

	command.Reasoning
}

// ListOutput is a page of tasks with their projections.
type ListOutput struct {
	Tasks []View `json:"tasks" jsonschema:"The tasks, newest first, each with its resolved assignee and plan progress."`
	Total int    `json:"total" jsonschema:"How many matched before the page was cut."`
}

// GetInput names one task.
type GetInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the task." validate:"required,notblank"`

	command.Reasoning
}

// CreateInput records a new task.
type CreateInput struct {
	Name string `json:"name" cli:"arg" jsonschema:"What the work is. Example: \"Denial patterns never match a command line\"." validate:"required,notblank"`

	Type     string   `json:"type,omitempty" cli:"flag" jsonschema:"One of the workspace's task types. Example: \"bug\"."`
	Status   Status   `json:"status,omitempty" cli:"flag" jsonschema:"Where it starts: suggestion, backlog, planning or todo. Defaults to backlog."`
	Priority Priority `json:"priority,omitempty" cli:"flag" jsonschema:"One of: no_priority, urgent, high, medium, low."`
	Assigned string   `json:"assigned,omitempty" cli:"flag" jsonschema:"Agent slug or user identifier. Only an agent is dispatched autonomously."`
	Summary  string   `json:"summary,omitempty" cli:"flag" jsonschema:"One-paragraph statement of the work."`
	DueAt    string   `json:"dueAt,omitempty" cli:"flag" jsonschema:"RFC3339 instant it is due."`

	Project   string   `json:"project,omitempty" cli:"flag" jsonschema:"Project this belongs to."`
	Goal      string   `json:"goal,omitempty" cli:"flag" jsonschema:"Goal this serves."`
	DependsOn []string `json:"dependsOn,omitempty" cli:"flag" jsonschema:"Tasks that must finish before this one can start."`

	Worktree bool   `json:"worktree,omitempty" cli:"flag" jsonschema:"Run this task in an isolated Git checkout."`
	Base     string `json:"base,omitempty" cli:"flag" jsonschema:"Branch the checkout is cut from."`

	Content string `json:"content,omitempty" jsonschema:"The description and plan, in Markdown."`

	command.Reasoning
}

// UpdateInput changes a task's description.
//
// Status is present and always rejected, which is the original's rule made
// mechanical: "use set_status for lifecycle moves; never change status via
// update".
type UpdateInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the task." validate:"required,notblank"`

	Name      *string   `json:"name,omitempty" cli:"flag" jsonschema:"New name. The slug follows it."`
	Type      *string   `json:"type,omitempty" cli:"flag" jsonschema:"New workspace task type."`
	Assigned  *string   `json:"assigned,omitempty" cli:"flag" jsonschema:"New owner. Empty unassigns."`
	Priority  *Priority `json:"priority,omitempty" cli:"flag" jsonschema:"New priority."`
	Summary   *string   `json:"summary,omitempty" cli:"flag" jsonschema:"New summary."`
	DueAt     *string   `json:"dueAt,omitempty" cli:"flag" jsonschema:"New due date, RFC3339. Empty clears it."`
	Project   *string   `json:"project,omitempty" cli:"flag" jsonschema:"New project."`
	Goal      *string   `json:"goal,omitempty" cli:"flag" jsonschema:"New goal."`
	DependsOn *[]string `json:"dependsOn,omitempty" cli:"flag" jsonschema:"New dependency list. Replaces the old one whole."`
	Chat      *string   `json:"chat,omitempty" cli:"flag" jsonschema:"Conversation this task's execution runs in."`
	Content   *string   `json:"content,omitempty" jsonschema:"New description and plan, in Markdown."`

	Status Status `json:"status,omitempty" cli:"flag" jsonschema:"Not writable here. Use set-status, which validates the move and runs the guards."`

	command.Reasoning
}

// SetStatusInput moves a task.
type SetStatusInput struct {
	ID     string `json:"id" cli:"arg" jsonschema:"Identifier of the task." validate:"required,notblank"`
	Status Status `json:"status" cli:"arg" jsonschema:"One of: suggestion, backlog, planning, todo, in_progress, stopped, in_review, finished." validate:"required,notblank"`

	// Reason is recorded on the checkpoint when stopping. A run that stops with
	// no reason is one nobody can decide whether to resume.
	Reason string `json:"reason,omitempty" cli:"flag" jsonschema:"Why. Recorded on the checkpoint when stopping."`

	command.Reasoning
}

// SetStatusOutput reports the move.
type SetStatusOutput struct {
	Task *View  `json:"task" jsonschema:"The task after the move."`
	From Status `json:"from" jsonschema:"Where it was."`
	To   Status `json:"to" jsonschema:"Where it is now."`
}

// DeleteInput removes a task and everything under it.
type DeleteInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the task." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput names what went.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"The task that was removed, with its todos, comments and runs."`
}

// BranchInput cuts the isolated checkout a task executes in.
type BranchInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the task." validate:"required,notblank"`

	Branch string `json:"branch,omitempty" cli:"flag" jsonschema:"Branch name. Generated from the workspace prefix and the task slug when left out."`
	Base   string `json:"base,omitempty" cli:"flag" jsonschema:"Branch to cut from."`

	command.Reasoning
}
