// Package task is the unit of work: an execution contract with a lifecycle,
// an owner and a review, not a reminder.
//
// Two rules make it that rather than a list item. Status moves only through
// SetStatus, which validates the move and runs the guards; and a task cannot
// reach in_review while any step of its plan is still open. The original states
// the second in the prompt and hopes; here it is enforced.
package task

import "time"

// Status is where a task sits in its lifecycle.
type Status string

const (
	Suggestion Status = "suggestion"
	Backlog    Status = "backlog"
	Planning   Status = "planning"
	Todo       Status = "todo"
	InProgress Status = "in_progress"
	Stopped    Status = "stopped"
	InReview   Status = "in_review"
	Finished   Status = "finished"
)

// Statuses lists the eight in lifecycle order.
var Statuses = []Status{
	Suggestion, Backlog, Planning, Todo, InProgress, Stopped, InReview, Finished,
}

// EnumValues publishes the task lifecycle to every schema.
func (s Status) EnumValues() []string {
	out := make([]string, 0, len(Statuses))
	for _, v := range Statuses {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether s is one of the eight.
func (s Status) Valid() bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// Priority is how urgent the work is.
type Priority string

const (
	NoPriority Priority = "no_priority"
	Urgent     Priority = "urgent"
	High       Priority = "high"
	Medium     Priority = "medium"
	Low        Priority = "low"
)

// Priorities lists them from most to least urgent, after the unset one.
var Priorities = []Priority{NoPriority, Urgent, High, Medium, Low}

// EnumValues publishes the priorities to every schema.
func (p Priority) EnumValues() []string {
	out := make([]string, 0, len(Priorities))
	for _, v := range Priorities {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether p is one of the five.
func (p Priority) Valid() bool {
	for _, known := range Priorities {
		if p == known {
			return true
		}
	}
	return false
}

// Task is one unit of work.
type Task struct {
	// ID comes from the path: a task lives at .aos/tasks/{id}/TASK.md, and
	// deleting it removes the directory with its todos, comments and runs.
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this task."`

	Name string `yaml:"name" json:"name" jsonschema:"What the work is. Example: \"Denial patterns never match a command line\"."`
	Slug string `yaml:"slug" json:"slug" jsonschema:"URL-safe form of the name, used in branch names."`
	Type string `yaml:"type" json:"type" jsonschema:"One of the workspace's task types. Example: \"bug\"."`

	// Assigned is an agent slug or a user identifier. Which of the two it is
	// decides whether this task is dispatched autonomously, and that is why the
	// resolution is a projection rather than a stored field: a stale copy of
	// "this is an agent" would send work to nobody.
	Assigned string     `yaml:"assigned,omitempty" json:"assigned,omitempty" jsonschema:"Agent slug or user identifier this task belongs to."`
	DueAt    *time.Time `yaml:"dueAt,omitempty" json:"dueAt,omitempty" jsonschema:"When it is due."`
	Priority Priority   `yaml:"priority" json:"priority" jsonschema:"One of: no_priority, urgent, high, medium, low."`
	Summary  string     `yaml:"summary,omitempty" json:"summary,omitempty" jsonschema:"One-paragraph statement of the work."`

	Status     Status      `yaml:"status" json:"status" jsonschema:"Lifecycle status. Moved with set-status, never written directly."`
	Checkpoint *Checkpoint `yaml:"checkpoint,omitempty" json:"checkpoint,omitempty" jsonschema:"Where an interrupted run stopped."`

	Template string   `yaml:"template,omitempty" json:"template,omitempty" jsonschema:"Template the description was generated from."`
	Worktree Worktree `yaml:"worktree" json:"worktree" jsonschema:"Git isolation policy for this task."`
	Chat     string   `yaml:"chat,omitempty" json:"chat,omitempty" jsonschema:"Conversation this task's execution runs in."`

	Project   string   `yaml:"project,omitempty" json:"project,omitempty" jsonschema:"Project this belongs to."`
	Goal      string   `yaml:"goal,omitempty" json:"goal,omitempty" jsonschema:"Goal this serves."`
	DependsOn []string `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty" jsonschema:"Tasks that must finish before this one starts."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When the task was created."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it last changed."`

	Content string `yaml:"-" json:"content,omitempty" collection:"content" jsonschema:"The description and plan, in Markdown."`
}

// Worktree is the Git isolation policy of one task.
//
// When enabled, the task executes on its own branch in its own checkout, and
// the sandbox root becomes that path — so an agent working on a task cannot
// touch the main working tree.
type Worktree struct {
	Enabled bool   `yaml:"enabled" json:"enabled" jsonschema:"Whether this task runs in an isolated checkout."`
	Base    string `yaml:"base,omitempty" json:"base,omitempty" jsonschema:"Branch it is cut from. Defaults to the current one."`
	Branch  string `yaml:"branch,omitempty" json:"branch,omitempty" jsonschema:"Branch name. Generated from the workspace prefix and the task slug."`
	Path    string `yaml:"path,omitempty" json:"path,omitempty" jsonschema:"Absolute path of the checkout, once it exists."`
}

// Checkpoint is where an interrupted run stopped, and what it takes to resume
// from exactly there rather than from the beginning.
type Checkpoint struct {
	ChatID         string    `yaml:"chatId,omitempty" json:"chatId,omitempty" jsonschema:"The conversation the run was using."`
	JobID          string    `yaml:"jobId,omitempty" json:"jobId,omitempty" jsonschema:"The queue job that was running it."`
	PendingTodoIDs []string  `yaml:"pendingTodoIds,omitempty" json:"pendingTodoIds,omitempty" jsonschema:"Steps that were still open."`
	Progress       Progress  `yaml:"progress" json:"progress" jsonschema:"How much of the plan was done."`
	StoppedAt      time.Time `yaml:"stoppedAt" json:"stoppedAt" jsonschema:"When it stopped."`
	Reason         string    `yaml:"reason,omitempty" json:"reason,omitempty" jsonschema:"Why it stopped."`
}

// Progress mirrors the todo aggregate's count. It is copied into the checkpoint
// rather than referenced so that a resumed run can report where it left off
// even if the plan has since changed.
type Progress struct {
	Completed int `yaml:"completed" json:"completed" jsonschema:"Steps finished or deliberately skipped."`
	Total     int `yaml:"total" json:"total" jsonschema:"Steps in the plan."`
}

// ResolvedAssignee is a read-only projection, never persisted.
//
// Type drives execution policy: only an agent receives autonomous dispatch. A
// task owned by a person is a task the system tracks and does not execute.
type ResolvedAssignee struct {
	ID   string `json:"id" jsonschema:"Identifier of the assignee."`
	Type string `json:"type" jsonschema:"agent, user or unknown."`
	Name string `json:"name,omitempty" jsonschema:"Display name."`
	Role string `json:"role,omitempty" jsonschema:"Role, for an agent."`
}

// The three kinds of assignee.
const (
	AssigneeAgent   = "agent"
	AssigneeUser    = "user"
	AssigneeUnknown = "unknown"
)

// Dispatchable reports whether this task may be executed autonomously. An
// unknown assignee is not: the system does not guess who work belongs to.
func (r ResolvedAssignee) Dispatchable() bool { return r.Type == AssigneeAgent }

// View is a task with the projections a reader needs but the file does not hold.
type View struct {
	Task

	Assignee ResolvedAssignee `json:"assignee" jsonschema:"Who owns the task, resolved."`
	Progress Progress         `json:"progress" jsonschema:"How much of the plan is done, now."`

	// Blocked lists the dependencies that are not finished. It is empty for a
	// task that can start, which is the question a caller is actually asking.
	Blocked []string `json:"blocked,omitempty" jsonschema:"Dependencies that are not finished yet."`
}
