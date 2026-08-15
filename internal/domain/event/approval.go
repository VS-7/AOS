package event

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Broker is the approval channel: the thing that makes `ask` mean ask.
//
// It holds the requests that are waiting and the goroutines that are blocked on
// them. Whoever can answer — the desktop over the event socket, a person at a
// terminal, an operator over HTTP — settles a request by id, and the blocked
// call returns with the decision.
//
// Fail-closed is the invariant: every path out of Request that is not a human
// saying yes returns Approved false. There is no timeout branch, no
// cancellation branch and no shutdown branch that approves.
type Broker struct {
	clock    Clock
	ids      IDs
	notifier Notifier
	deadline time.Duration

	mu      sync.Mutex
	waiting map[string]*pending
	closed  bool
}

type pending struct {
	req    ApprovalRequest
	answer chan ApprovalResult
	once   sync.Once
}

// BrokerDeps is what the broker is built from.
type BrokerDeps struct {
	Clock    Clock
	IDs      IDs
	Notifier Notifier

	// Deadline is how long a request waits. Zero means the default of two
	// minutes, which is the original's own number for an interactive prompt.
	Deadline time.Duration
}

// NewBroker builds an approval channel with nobody waiting on it.
func NewBroker(d BrokerDeps) *Broker {
	deadline := d.Deadline
	if deadline == 0 {
		deadline = DefaultApprovalDeadline
	}
	return &Broker{
		clock:    d.Clock,
		ids:      d.IDs,
		notifier: d.Notifier,
		deadline: deadline,
		waiting:  map[string]*pending{},
	}
}

// RequestApproval publishes a request and blocks until it is settled.
func (b *Broker) RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResult, error) {
	if req.ID == "" && b.ids != nil {
		req.ID = b.ids.New()
	}
	if req.Risk == "" {
		req.Risk = RiskMedium
	}
	deadline := req.Deadline
	if deadline == 0 {
		deadline = b.deadline
	}
	now := b.now()
	req.CreatedAt = now
	req.ExpiresAt = now.Add(deadline)

	p := &pending{req: req, answer: make(chan ApprovalResult, 1)}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ApprovalResult{Reason: "the approval channel is shutting down"}, nil
	}
	b.waiting[req.ID] = p
	b.mu.Unlock()

	defer b.forget(req.ID)

	if b.notifier != nil {
		b.notifier.ApprovalRequested(ctx, req)
	}

	timer := time.NewTimer(deadline)
	defer timer.Stop()

	var res ApprovalResult
	select {
	case res = <-p.answer:
	case <-timer.C:
		res = ApprovalResult{Approved: false, Reason: "nobody answered within " + deadline.String()}
	case <-ctx.Done():
		res = ApprovalResult{Approved: false, Reason: "the turn was cancelled while waiting for approval"}
	}

	if b.notifier != nil {
		b.notifier.ApprovalSettled(ctx, req, res)
	}
	return res, nil
}

// Pending lists the requests still waiting, oldest first.
func (b *Broker) Pending() []ApprovalRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]ApprovalRequest, 0, len(b.waiting))
	for _, p := range b.waiting {
		out = append(out, p.req)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// Decide settles a pending request. It reports whether the id was waiting: an
// answer to a request that already timed out must not look like it landed.
func (b *Broker) Decide(id string, res ApprovalResult) bool {
	b.mu.Lock()
	p, ok := b.waiting[id]
	b.mu.Unlock()
	if !ok {
		return false
	}
	// Once, because two people clicking at the same moment is a race the
	// interface cannot prevent and the broker can.
	delivered := false
	p.once.Do(func() {
		p.answer <- res
		delivered = true
	})
	return delivered
}

// Close releases everyone waiting, denying each. A daemon shutting down with a
// blocked approval must not leave a goroutine holding a turn forever.
func (b *Broker) Close() {
	b.mu.Lock()
	b.closed = true
	all := make([]*pending, 0, len(b.waiting))
	for _, p := range b.waiting {
		all = append(all, p)
	}
	b.mu.Unlock()

	for _, p := range all {
		p.once.Do(func() {
			p.answer <- ApprovalResult{Approved: false, Reason: "the daemon is shutting down"}
		})
	}
}

func (b *Broker) forget(id string) {
	b.mu.Lock()
	delete(b.waiting, id)
	b.mu.Unlock()
}

func (b *Broker) now() time.Time {
	if b.clock == nil {
		return time.Time{}
	}
	return b.clock.Now()
}

// NoopApprover is the answer in a context where there is nobody to ask: a
// routine at three in the morning, an autonomous task, an MCP client with no
// interface.
//
// It denies immediately rather than after the deadline. Waiting two minutes for
// a human who is definitionally not there is the cost with none of the benefit,
// and ADR-0007 says so explicitly.
type NoopApprover struct{}

// RequestApproval denies, with a reason that says why rather than pretending a
// hook decided. The distinction matters to whoever reads the transcript: the
// original's message is indistinguishable from a policy denial.
func (NoopApprover) RequestApproval(_ context.Context, _ ApprovalRequest) (ApprovalResult, error) {
	return ApprovalResult{
		Approved: false,
		Reason:   "no approval channel is available in this run mode, so the call could not be confirmed",
	}, nil
}
