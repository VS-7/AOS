// Package todo is the execution plan inside a task.
//
// A todo is a step, not a reminder. What makes it load-bearing is that a task
// cannot reach in_review while any of its todos is still pending: the plan is
// what the review guard counts, so a plan nobody wrote is a task nobody can
// finish honestly.
package todo

import "time"

// Status is where one step sits.
type Status string

const (
	Pending    Status = "pending"
	InProgress Status = "in_progress"
	Blocked    Status = "blocked"
	Finished   Status = "finished"
	Skipped    Status = "skipped"
)

// Statuses lists them in lifecycle order.
var Statuses = []Status{Pending, InProgress, Blocked, Finished, Skipped}

// EnumValues publishes the accepted statuses. "done" is the value a model
// reaches for first and the one this list shows is not here (defect #8).
func (s Status) EnumValues() []string {
	out := make([]string, 0, len(Statuses))
	for _, v := range Statuses {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether s is one of the five.
func (s Status) Valid() bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// Settled reports whether this step is done with, one way or another. It is
// what CountPending counts the absence of.
func (s Status) Settled() bool { return s == Finished || s == Skipped }

// transitions is the lifecycle graph. Anything not listed is refused.
//
// Every non-terminal state can reach Skipped, because discovering that a step
// was unnecessary can happen at any point. Finished can go back to Pending: a
// step that turns out not to have worked is reopened, not deleted.
var transitions = map[Status][]Status{
	Pending:    {InProgress, Blocked, Finished, Skipped},
	InProgress: {Blocked, Finished, Pending, Skipped},
	Blocked:    {InProgress, Pending, Skipped},
	Finished:   {Pending, InProgress},
	Skipped:    {Pending},
}

// CanMoveTo reports whether the lifecycle allows this move.
func (s Status) CanMoveTo(next Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// NextStates lists where this status can go, for the error message that says
// what was possible instead.
func (s Status) NextStates() []string {
	out := make([]string, 0, len(transitions[s]))
	for _, next := range transitions[s] {
		out = append(out, string(next))
	}
	return out
}

// Todo is one step of a task's plan.
type Todo struct {
	// TaskID and ID come from the path: a todo lives at
	// .aos/tasks/{taskId}/todos/{id}.todo.md.
	TaskID string `yaml:"-" json:"taskId" collection:"path" jsonschema:"Identifier of the task this step belongs to."`
	ID     string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this step."`

	Title  string `yaml:"title" json:"title" jsonschema:"What this step is. Example: \"Reproduce the failure in a test\"."`
	Status Status `yaml:"status" json:"status" jsonschema:"One of: pending, in_progress, blocked, finished, skipped."`
	Order  int    `yaml:"order" json:"order" jsonschema:"Position in the plan. Steps are listed by it."`

	// Evidence is what was actually verified. The master prompt demands
	// evidence of validation before a task can be reviewed; without a field for
	// it the evidence lives only in prose and cannot be queried.
	Evidence string `yaml:"evidence,omitempty" json:"evidence,omitempty" jsonschema:"What you verified, concretely. Example: \"go test ./internal/domain/task passes, 24 cases\"."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When the step was added."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it last changed."`

	Content string `yaml:"-" json:"content,omitempty" collection:"content" jsonschema:"Notes on this step, in Markdown."`
}

// Progress is the count a task's checkpoint carries.
type Progress struct {
	Completed int `json:"completed" jsonschema:"Steps that are finished or deliberately skipped."`
	Total     int `json:"total" jsonschema:"Steps in the plan."`
}

// Pending reports how many steps are still open.
func (p Progress) Pending() int { return p.Total - p.Completed }
