package routine

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist a routine.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Routine, error)
	List(ctx context.Context, q collections.Query) ([]Routine, error)
	Create(ctx context.Context, v *Routine) error
	Update(ctx context.Context, v *Routine, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Runs persists the audit record of each firing.
type Runs interface {
	Get(ctx context.Context, key collections.Key) (*Run, error)
	List(ctx context.Context, q collections.Query) ([]Run, error)
	Create(ctx context.Context, v *Run) error
	Update(ctx context.Context, v *Run, expect collections.Version) error
}

// Executor runs a routine's prompt as the agent that owns it.
//
// It is a port because the runtime is one layer out: the domain decides that a
// routine is due and what it may do, and the runtime decides how a turn is
// taken. Returning the conversation is what ties the audit record to the
// transcript.
type Executor interface {
	Execute(ctx context.Context, req Execution) (Outcome, error)
}

// Execution is one firing handed to the runtime.
type Execution struct {
	Agent   string
	Routine string
	RunID   string
	Trigger TriggerType
	Payload map[string]any

	// Prompt is the routine's body: what the agent is to do.
	Prompt string

	// Scope is the filter the runtime applies to the tool registry. A routine
	// that may not create tasks does not see tasks_create at all, which is
	// better than being refused after choosing it.
	Scope Scope
}

// Outcome is what a firing produced.
type Outcome struct {
	ChatID string
	Usage  Usage
}

// Dispatcher hands a scheduled firing on to be run later, off the tick that
// found it due.
//
// A run is a whole turn. The tick used to take it inline, and every other due
// routine — in this workspace and in the next the tick visits — waited for it,
// and the ticks that passed meanwhile were lost.
type Dispatcher interface {
	// Dispatch hands one firing on. It reports false, having handed nothing
	// on, when an earlier firing of the same routine is still waiting or
	// running, so a turn longer than the routine's interval does not pile up
	// copies of itself.
	Dispatch(ctx context.Context, f Firing) (bool, error)
}

// Firing is one scheduled firing handed on. Whoever runs it fires it with
// Input, which records it as the scheduled run it is.
type Firing struct {
	Agent   string `json:"agent"`
	Routine string `json:"routine"`
	Cron    string `json:"cron"`
}

// Input is how the firing is run: Fire, as a scheduled trigger carrying its
// cron, and never forced — a routine disabled while its firing waited records
// a skipped run instead.
func (f Firing) Input() FireInput {
	return FireInput{
		ID: f.Routine, Agent: f.Agent, Trigger: Scheduled,
		Payload: map[string]any{"cron": f.Cron},
	}
}

// Tokens mints and verifies webhook secrets.
//
// It is a port so the hashing choice lives in an adapter: the domain's rule is
// only that the file holds a hash and the token is shown once.
type Tokens interface {
	New() (token, hash string, err error)
	Verify(token, hash string) bool
}

// Directory answers whether an agent exists, so a routine cannot be created
// under one that does not.
type Directory interface {
	IsAgent(ctx context.Context, id string) bool
}

// Notifier publishes what happened to a routine.
type Notifier interface {
	RoutineFired(ctx context.Context, r *Routine, run *Run)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new routine or run.
type IDs interface{ New() string }
