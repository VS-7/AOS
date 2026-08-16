package activity

import (
	"context"
	"time"
)

// Log is the append-only store, partitioned by month.
//
// Append is the hot path and is O(1) by construction. Rewrite exists for the
// two operations that genuinely have to edit history — deleting one entry and
// purging a partial month — and is deliberately awkward to reach, because
// rewriting an audit log should never be the easy option.
type Log interface {
	Append(ctx context.Context, a Activity) error

	// Load reads every entry created at or after since, oldest first. A zero
	// time reads everything the log still holds.
	Load(ctx context.Context, since time.Time) ([]Activity, error)

	// Months lists the partitions present, oldest first, as "2026-03".
	Months(ctx context.Context) ([]string, error)

	// Rewrite replaces one month with exactly these entries. Passing none
	// removes the partition.
	Rewrite(ctx context.Context, month string, entries []Activity) error
}

// ReadStore persists the per-actor read overlay.
type ReadStore interface {
	Load(ctx context.Context) (ReadState, error)
	Save(ctx context.Context, s ReadState) error
}

// Sink is a best-effort consumer of a published activity.
//
// Two are wired at the composition root: the realtime hub, so an open desktop
// sees it now, and the routine trigger evaluator, so an automation reacts to
// it. Neither may fail the mutation that produced the activity — a routine that
// throws must not roll back the task whose status changed.
type Sink interface {
	OnActivity(ctx context.Context, a Activity)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new entry.
type IDs interface{ New() string }
