package job

import (
	"context"
	"encoding/json"
	"time"
)

// Queue is the durable store of deferred work.
//
// Claim is the operation that makes it a queue rather than a table: it must
// hand one job to exactly one caller, atomically, or two workers do the same
// work. Everything else here could be built on files.
//
// Wide port: a job's lifecycle has ten distinct transitions and every one of
// them has to be atomic against the others. Splitting them into a writer and a
// reader would put the claim on one side and the lease on the other, and the
// implementation would immediately need both halves to share a transaction —
// which is the coupling the split was supposed to remove.
type Queue interface {
	Enqueue(ctx context.Context, j Job) (string, error)

	// Claim takes the next eligible job from any of the named queues and holds
	// it for lease. It returns nil when there is nothing to do, which is the
	// common case and not an error.
	Claim(ctx context.Context, queues []string, worker string, lease time.Duration) (*Job, error)

	// Heartbeat extends the lease of a job this worker is still running.
	Heartbeat(ctx context.Context, jobID, worker string, lease time.Duration) error

	Complete(ctx context.Context, jobID string, result json.RawMessage) error

	// Fail records an attempt that did not work. retryIn nil means the queue
	// decides from the attempt count; a job out of attempts becomes dead.
	Fail(ctx context.Context, jobID string, cause error, retryIn *time.Duration) error

	// RecoverStale hands back jobs whose lease lapsed, and reports how many.
	// It is what makes a worker that was killed recoverable rather than a
	// permanent hole in the queue.
	RecoverStale(ctx context.Context) (int, error)

	Get(ctx context.Context, jobID string) (*Job, error)
	List(ctx context.Context, q Filter) ([]Job, error)

	// Purge removes terminal jobs older than the window and reports how many.
	Purge(ctx context.Context, olderThan time.Duration) (int, error)

	Close() error
}

// Filter selects jobs for a listing.
type Filter struct {
	Queue     string
	Status    Status
	Workspace string
	Kind      string
	Limit     int
}

// Handler runs one job.
//
// Returning an error retries it according to the queue's policy. The result is
// stored on the record, which is what lets an operator see what a job actually
// produced rather than only that it finished.
type Handler interface {
	Handle(ctx context.Context, j Job) (json.RawMessage, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, j Job) (json.RawMessage, error)

// Handle runs the function.
func (f HandlerFunc) Handle(ctx context.Context, j Job) (json.RawMessage, error) { return f(ctx, j) }

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new job.
type IDs interface{ New() string }
