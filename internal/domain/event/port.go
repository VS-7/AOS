package event

import (
	"context"
	"time"
)

// Handler is one hook. It is an interface rather than a function so that a
// handler can say which events it wants without the bus calling it for the
// eight it does not.
type Handler interface {
	ID() string
	Handles() []Type
	Handle(ctx context.Context, e Event) (Outcome, error)
}

// Log is the append-only audit sink.
//
// One method, and deliberately so: there is no Update and no Delete anywhere in
// this system, and the way to keep it that way is for the port not to have
// them. Retention removes whole files, which is the adapter's business.
type Log interface {
	Append(ctx context.Context, r Record) error
}

// Approver is the human in the loop.
//
// The original degrades `ask` to `deny` (defect #8). This port is what makes
// the honest answer possible; ADR-0007 is the argument for why the honest
// answer is worth the complexity.
type Approver interface {
	// RequestApproval blocks until a human decides, the deadline elapses, or
	// the context is cancelled. It MUST NOT return Approved on a timeout: the
	// absence of an answer is not an answer.
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResult, error)
}

// Notifier is how a pending approval reaches whoever can answer it. It is a
// port because the desktop hears it over a socket, the CLI over a channel in
// the same process, and a test over neither.
type Notifier interface {
	ApprovalRequested(ctx context.Context, req ApprovalRequest)
	ApprovalSettled(ctx context.Context, req ApprovalRequest, res ApprovalResult)
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out the identifier of a record or of a pending request.
type IDs interface{ New() string }
