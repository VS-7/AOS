package job

import (
	"time"

	"github.com/OWNER/aos/internal/core/command"
)

// ListInput selects jobs.
type ListInput struct {
	Queue     string `json:"queue,omitempty" cli:"flag" jsonschema:"One of: chat, task, routine, workspace."`
	Status    Status `json:"status,omitempty" cli:"flag" jsonschema:"One of: pending, claimed, succeeded, failed, dead."`
	Workspace string `json:"workspace,omitempty" cli:"flag" jsonschema:"Only jobs of this workspace."`
	Kind      string `json:"kind,omitempty" cli:"flag" jsonschema:"Only jobs of this handler kind."`
	Limit     int    `json:"limit,omitempty" cli:"flag" jsonschema:"How many to return, newest first."`

	command.Reasoning
}

// ListOutput is the matching jobs.
type ListOutput struct {
	Jobs  []Job `json:"jobs" jsonschema:"The jobs, newest first."`
	Total int   `json:"total" jsonschema:"How many were returned."`
}

// GetInput names one job.
type GetInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the job." validate:"required,notblank"`

	command.Reasoning
}

// StatsInput scopes the count.
type StatsInput struct {
	Workspace string `json:"workspace,omitempty" cli:"flag" jsonschema:"Only jobs of this workspace."`

	command.Reasoning
}

// StatsOutput is the shape of the queue right now.
type StatsOutput struct {
	Total    int            `json:"total" jsonschema:"How many jobs the queue holds."`
	ByStatus map[string]int `json:"byStatus" jsonschema:"How many in each state."`
	ByQueue  map[string]int `json:"byQueue" jsonschema:"How many on each queue."`

	Dead []string `json:"dead,omitempty" jsonschema:"Jobs that failed their last attempt."`

	// Stale is the shape of a real incident: a job still marked claimed whose
	// lease has already lapsed, which means the worker holding it died.
	Stale []string `json:"stale,omitempty" jsonschema:"Jobs whose worker stopped reporting. Recover hands them back."`

	At time.Time `json:"at" jsonschema:"When this was counted."`
}

// RecoverInput takes nothing: the question is always the same one.
type RecoverInput struct {
	command.Reasoning
}

// RecoverOutput says how many were handed back.
type RecoverOutput struct {
	Recovered int `json:"recovered" jsonschema:"How many jobs were returned to the queue."`
}

// PurgeInput drops finished jobs.
type PurgeInput struct {
	OlderThanDays int `json:"olderThanDays,omitempty" cli:"flag" jsonschema:"Override the window, in days. Defaults to 7."`

	command.Reasoning
}

// PurgeOutput says what went.
type PurgeOutput struct {
	Removed   int    `json:"removed" jsonschema:"How many finished jobs were removed."`
	OlderThan string `json:"olderThan" jsonschema:"The window that was applied."`
}
