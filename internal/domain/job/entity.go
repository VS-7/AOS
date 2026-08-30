// Package job is the durable work queue: the one place in this system with a
// database behind it.
//
// Everything else is files, because everything else is domain state a person
// might want to read, edit and commit (ADR-0004). A queue is not that. It is
// ephemeral operational state, and the one thing it needs that a filesystem
// cannot give is an atomic claim: two workers asking for work at the same
// instant must not get the same job. That is what ADR-0008 buys, and it is the
// whole reason for the exception.
package job

import (
	"encoding/json"
	"time"
)

// Status is where a job sits.
type Status string

const (
	Pending   Status = "pending"
	Claimed   Status = "claimed"
	Succeeded Status = "succeeded"
	Failed    Status = "failed"

	// Dead is a job that failed its last attempt. It stays in the table rather
	// than being deleted: "it ran and failed four times" and "it never arrived"
	// are different problems, and a queue that forgets cannot tell them apart.
	Dead Status = "dead"
)

// Statuses lists them all.
var Statuses = []Status{Pending, Claimed, Succeeded, Failed, Dead}

// EnumValues publishes the job lifecycle to every schema.
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

// The four queues, inherited from the original.
//
// They are separate so that a slow batch of one kind cannot starve another: a
// worker pool draining chat does not stop because the task queue is full.
const (
	QueueChat      = "chat"
	QueueTask      = "task"
	QueueRoutine   = "routine"
	QueueWorkspace = "workspace"
)

// Queues lists the four.
var Queues = []string{QueueChat, QueueTask, QueueRoutine, QueueWorkspace}

// Job is one unit of deferred work.
type Job struct {
	ID    string `json:"id"`
	Queue string `json:"queue"`

	// Kind names the handler. The queue routes by it, so a queue can carry more
	// than one kind of work without the worker having to guess.
	Kind string `json:"kind"`

	Payload json.RawMessage `json:"payload,omitempty"`

	// Workspace scopes the job. The tick fans out per workspace, and a worker
	// running one installation must not pick up another's work.
	Workspace string `json:"workspace,omitempty"`

	Status   Status `json:"status"`
	Attempts int    `json:"attempts"`
	MaxTries int    `json:"maxTries"`

	// RunAt is when the job becomes eligible. A retry sets it forward; that is
	// the whole of the backoff.
	RunAt time.Time `json:"runAt"`

	// LeaseUntil is when a claim expires. A worker that dies stops
	// heartbeating, the lease lapses, and RecoverStale hands the job back.
	LeaseUntil *time.Time `json:"leaseUntil,omitempty"`

	// ClaimedBy names the worker holding it, for the operator who wants to know
	// which process is stuck.
	ClaimedBy string `json:"claimedBy,omitempty"`

	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

// The queue's operating parameters, inherited from the original and made
// configurable here. The original's own comment records the intention it never
// got to: "Isolated into env config later."
const (
	// DefaultLease is the original's lockDuration.
	DefaultLease = 10 * time.Minute

	// DefaultHeartbeat is how often a worker renews its lease. It has to be
	// comfortably shorter than the lease, or a slow tick reaps live work.
	DefaultHeartbeat = 30 * time.Second

	// DefaultConcurrency is the original's global worker count.
	DefaultConcurrency = 20

	// DefaultTick is the original's `*/15 * * * *`.
	DefaultTick = 15 * time.Minute

	// DefaultMaxTries is how many times a job is attempted before it is dead.
	DefaultMaxTries = 3
)

// Backoff is how long to wait before the nth retry.
//
// Exponential from ten seconds, capped at ten minutes. The cap matters more
// than the curve: an unbounded backoff turns a transient provider outage into a
// job that retries next week.
func Backoff(attempt int) time.Duration {
	const base = 10 * time.Second
	const cap = 10 * time.Minute
	if attempt < 1 {
		attempt = 1
	}
	wait := base
	for range attempt - 1 {
		wait *= 2
		if wait >= cap {
			return cap
		}
	}
	return wait
}

// Terminal reports whether a job will not run again.
func (j Job) Terminal() bool { return j.Status == Succeeded || j.Status == Dead }

// Exhausted reports whether this attempt was the last one allowed.
func (j Job) Exhausted() bool {
	limit := j.MaxTries
	if limit <= 0 {
		limit = DefaultMaxTries
	}
	return j.Attempts >= limit
}
