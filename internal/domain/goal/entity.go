// Package goal holds the strategic outcomes daily work aligns to. A goal
// exists to answer one question before an agent starts something significant:
// does this serve anything, or is it technically correct and strategically
// useless work? See docs/04 - Domínio/Goal (Go).md for the design.
package goal

import "time"

// Status is where a goal sits in its own lifecycle.
type Status string

const (
	StatusActive    Status = "active"
	StatusAchieved  Status = "achieved"
	StatusAbandoned Status = "abandoned"
	StatusPaused    Status = "paused"
)

// statuses lists every member of the union, in declaration order.
var statuses = []Status{StatusActive, StatusAchieved, StatusAbandoned, StatusPaused}

// EnumValues publishes the accepted statuses to every schema.
func (s Status) EnumValues() []string {
	out := make([]string, 0, len(statuses))
	for _, v := range statuses {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether s is one of the four declared lifecycle states.
func (s Status) Valid() bool {
	for _, known := range statuses {
		if s == known {
			return true
		}
	}
	return false
}

// Priority is how urgent a goal is.
//
// It exists because the interface has always edited it — a priority column on
// the list, a dropdown on the row, a field on the form — and Go had no such
// field, so `goals_update {priority}` answered 200 with the priority silently
// dropped and the screen toasted "Priority updated to High" over a record
// that had not changed. The five values are the ones the interface offers,
// and the same set a task carries.
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

// Goal is a strategic outcome, checkable rather than aspirational once it
// carries a Measure. .aos/goals/{id}.goal.md, one file per goal, exactly like
// every other native collection record.
type Goal struct {
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this goal, used to address it in goals_get/update/delete."`

	Title       string `yaml:"title"                 json:"title" jsonschema:"What this goal is."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"One line summarising the outcome."`
	Status      Status `yaml:"status"                json:"status" jsonschema:"One of: active, achieved, abandoned, paused."`

	Priority Priority `yaml:"priority,omitempty" json:"priority,omitempty" jsonschema:"How urgent this goal is: no_priority, urgent, high, medium or low."`

	// Project scopes the goal under a Project (Go), when it has one — a goal
	// need not belong to a project.
	Project string `yaml:"project,omitempty" json:"project,omitempty" jsonschema:"Project this goal belongs to, if any."`

	DueAt *time.Time `yaml:"dueAt,omitempty" json:"dueAt,omitempty" jsonschema:"When this goal is due, if it has a deadline."`

	// Skill names the skill that installed this goal, when it did — see
	// docs/04 - Domínio/Skill (Go).md's metadata.goals. Cleared alongside the
	// skill on uninstall.
	Skill string `yaml:"skill,omitempty" json:"skill,omitempty" jsonschema:"Skill that installed this goal, if any."`

	// Measure makes the outcome checkable instead of aspirational — a goal
	// without a verifiable criterion gives the agent no way to tell whether
	// it contributed. Addition over the original, which has no equivalent
	// field.
	Measure string `yaml:"measure,omitempty" json:"measure,omitempty" jsonschema:"How to tell this goal was actually served, not just attempted."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When this goal was created."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`

	Content string `yaml:"-" json:"content" collection:"content" jsonschema:"Markdown body, below the frontmatter."`
}
