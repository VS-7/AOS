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

	// List reports the checkouts of this repository that exist under root,
	// other than the main working tree, so the prune can see what it has.
	// Whether one is under root is decided once links are resolved, the way
	// git reports a checkout, and each is spelled under root as the caller
	// wrote it — so the paths compare with the ones Branch placed there
	// whatever link the state directory is reached through.
	List(ctx context.Context, root string) ([]string, error)

	// Exists reports whether path is one of this repository's checkouts, is
	// still on disk, and — once links are resolved — sits under root. A task's
	// recorded path is read back from a file that is copied, restored and
	// edited, and it becomes a sandbox root: it is asked about rather than
	// believed.
	Exists(ctx context.Context, root, path string) bool

	// Source reports what a checkout for spec would be cut from, before
	// anything is created or pruned — so a workspace with nothing to cut from
	// is refused with the reason rather than with git's last words.
	Source(ctx context.Context, spec WorktreeSpec) (WorktreeSource, error)

	// WorkspaceIn is the directory the workspace is inside one of its
	// checkouts: the checkout itself when the workspace is its repository's
	// top, the workspace's folder inside it when the workspace is a folder of
	// a project. found is false, and the directory the checkout, when that
	// folder is not in the checkout or leads out of it.
	//
	// It depends on where the workspace sits in its repository and on the
	// checkout, never on the task's branch: it is asked on every turn.
	WorkspaceIn(ctx context.Context, checkout string) (dir string, found bool, err error)
}

// WorktreeSource is the repository a task's checkout would come from.
type WorktreeSource struct {
	// Dir is the workspace directory checkouts are cut from.
	Dir string

	// Toplevel is the top of the working tree Dir belongs to, or "" when it
	// belongs to none.
	Toplevel string

	// Own reports whether that working tree is Dir itself. When it is not, the
	// workspace is a directory inside somebody's repository — a monorepo's
	// subfolder — and a checkout is of that repository, holding the workspace
	// at Subdir.
	Own bool

	// BaseExists reports whether there is something to check out: the branch
	// already exists, or the base (HEAD when none is named) is a commit. A
	// repository nobody has committed to has neither.
	BaseExists bool

	// Subdir is where the workspace sits inside Toplevel when that repository
	// is not its own, as a relative path; "" when it is.
	Subdir string

	// SubdirCommitted reports whether what would be checked out holds Subdir.
	// A folder the enclosing repository never committed is not in a checkout
	// of it, and a task rooted there would find nothing.
	SubdirCommitted bool
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
	BranchPrefix   string
	Limit          int
	DeleteOld      bool
	OnCreateScript string

	// Root is where this workspace places its checkouts: its own directory,
	// never shared with another workspace. Two workspaces that are folders of
	// one repository cut checkouts from that one repository, and a root they
	// shared made each one's checkouts look like the other's leftovers.
	Root string

	// LegacyRoot is the installation-wide directory checkouts were placed in
	// before each workspace had a root of its own. A task's checkout recorded
	// there, named after the task, is still the task's; anything else in it
	// may belong to any workspace, so it is never counted or pruned.
	LegacyRoot string

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
