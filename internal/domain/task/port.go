package task

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository is what this package needs to persist a task.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Task, error)
	List(ctx context.Context, q collections.Query) ([]Task, error)
	Create(ctx context.Context, v *Task) error
	Update(ctx context.Context, v *Task, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Plan is the slice of the todo aggregate the review guard needs.
//
// It is three questions, not the whole service: how much is open, which steps
// they are, and how far along the plan is. A task that could reach further
// could rewrite the plan it is being judged against.
type Plan interface {
	CountPending(ctx context.Context, taskID string) (int, error)
	PendingIDs(ctx context.Context, taskID string) ([]string, error)
	Progress(ctx context.Context, taskID string) (Progress, error)
}

// Directory resolves an assignee to what it actually is.
//
// The answer decides execution policy — only an agent is dispatched — so it is
// asked afresh rather than stored: a task whose assignee was deleted must stop
// being dispatchable, not keep a stale label that says it still is.
type Directory interface {
	Resolve(ctx context.Context, id string) (ResolvedAssignee, error)
}

// Worktrees creates and removes the isolated checkouts a task executes in.
type Worktrees interface {
	// Create cuts a branch and checks it out at its own path. It returns where.
	Create(ctx context.Context, spec WorktreeSpec) (string, error)

	// Remove deletes one checkout. Removing one that is not there is not an
	// error: a prune that fails because somebody already cleaned up by hand is
	// a prune that stops working.
	Remove(ctx context.Context, path string) error

	// List reports the checkouts that exist, so the prune can see what it has.
	List(ctx context.Context) ([]string, error)
}

// WorktreeSpec is what it takes to cut one.
type WorktreeSpec struct {
	TaskID string
	Branch string
	Base   string

	// Path is where the checkout goes. The caller decides, because it lives
	// outside the repository and the domain does not know the layout of the
	// state directory.
	Path string
}

// Setup runs the workspace's onCreateScript inside a fresh checkout.
//
// It is a port rather than an exec call because the script is third-party code
// in most workspaces, and it runs under the assigned agent's sandbox policy —
// which is a divergence from the original, where it runs unrestricted.
type Setup interface {
	Run(ctx context.Context, agentID, dir, script string) error
}

// Policy is what the workspace says about isolation and branch naming.
type Policy interface {
	Worktrees(ctx context.Context) (WorktreePolicy, error)
	TaskTypes(ctx context.Context) ([]string, error)
}

// WorktreePolicy is the workspace's isolation configuration.
type WorktreePolicy struct {
	BranchPrefix       string
	Limit              int
	DeleteOld          bool
	OnCreateScript     string
	Root               string // where checkouts are placed
	DefaultBase        string
	EnabledByDefault   bool
	ScriptTimeoutHint  time.Duration
	ScriptUnderSandbox bool
}

// Notifier publishes what happened to a task.
//
// It is best-effort and cannot fail the mutation: an activity log that is down
// must not stop work from moving.
type Notifier interface {
	TaskChanged(ctx context.Context, event string, t *Task, data map[string]any)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a new task.
type IDs interface{ New() string }
