package antigravity

import (
	"context"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
)

// The pacing and back-off constants.
//
// These are not tuned against a published rate limit, because Google publishes
// none for this endpoint. They are chosen to keep an agent loop — which calls
// in bursts, once per step, for as many steps as a task takes — looking like a
// person using an editor, which is the traffic this allowance is for.
const (
	// minInterval is the floor between two calls leaving this process. A tool
	// loop that finishes a step in milliseconds would otherwise issue a dozen
	// requests a second, which is the shape that gets an account looked at.
	minInterval = 400 * time.Millisecond

	// firstCooldown is how long a refusal shuts the door. It doubles on each
	// consecutive refusal up to maxCooldown, and one success clears it.
	firstCooldown = 30 * time.Second
	maxCooldown   = 15 * time.Minute
)

// guard is the part of this adapter that protects the account rather than the
// call.
//
// It does two things and deliberately does not do a third. It paces what
// leaves this process, and it stops sending after the service has refused —
// for a window that grows while refusals keep coming. What it does not do is
// retry. That is the decision worth stating plainly: a 429 answered by a retry
// is a client arguing with a rate limiter, and the argument is what turns a
// throttle into a suspension. The turn fails, the person sees why and when it
// lifts, and nothing further is sent in the meantime.
//
// The window is per provider instance, and instances are built per call site
// from the registry, so this is not a process-wide limiter. It does not need
// to be: what it exists to prevent is one runaway loop, and a loop holds one
// provider.
type guard struct {
	mu sync.Mutex

	// now is injected so a test can move time without spending it.
	now func() time.Time
	// sleep is injected for the same reason.
	sleep func(ctx context.Context, d time.Duration) error

	last         time.Time
	blockedUntil time.Time
	blockReason  string
	refusals     int
}

func newGuard(clock func() time.Time) *guard {
	if clock == nil {
		clock = timeNow
	}
	return &guard{now: clock, sleep: sleepContext}
}

// enter blocks until this process may make another call, or fails when the
// door is shut.
func (g *guard) enter(ctx context.Context) error {
	g.mu.Lock()
	now := g.now()
	if now.Before(g.blockedUntil) {
		until, reason := g.blockedUntil, g.blockReason
		g.mu.Unlock()
		return errBackingOff(until.Sub(now), reason)
	}
	wait := time.Duration(0)
	if !g.last.IsZero() {
		if elapsed := now.Sub(g.last); elapsed < minInterval {
			wait = minInterval - elapsed
		}
	}
	// Claim the slot before releasing, so two goroutines entering together
	// space themselves out instead of both measuring against the same past.
	g.last = now.Add(wait)
	g.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	return g.sleep(ctx, wait)
}

// observe records what the service answered.
//
// A refusal that is about load or entitlement shuts the door; anything else —
// a malformed request, a model that does not exist — is this build's fault and
// closing the door would only hide it. A clean answer forgives the history,
// because an allowance that has reset should not still be serving a back-off
// from before it did.
func (g *guard) observe(err error) {
	status := 0
	if err != nil {
		status = apperr.StatusOf(err)
		if e, ok := apperr.As(err); ok {
			if raw, found := e.Issues["status"]; found {
				if code, isInt := raw.(int); isInt {
					status = code
				}
			}
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	switch {
	case err == nil:
		g.refusals = 0
		g.blockedUntil = time.Time{}
		g.blockReason = ""
	case status == apperr.StatusTooManyRequests:
		g.shut("the allowance is exhausted or the service is throttling this account")
	case status == apperr.StatusUnauthorized || status == apperr.StatusForbidden:
		g.shut("the login was refused")
	}
}

// shut extends the back-off. It is called with the lock held.
func (g *guard) shut(reason string) {
	g.refusals++
	cooldown := firstCooldown
	for i := 1; i < g.refusals && cooldown < maxCooldown; i++ {
		cooldown *= 2
	}
	cooldown = min(cooldown, maxCooldown)
	g.blockedUntil = g.now().Add(cooldown)
	g.blockReason = reason
}

// block shuts the door for a stated reason that did not come from an HTTP
// status — an exhausted quota this adapter read for itself, before spending
// anything to discover it.
func (g *guard) block(until time.Time, reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if until.After(g.blockedUntil) {
		g.blockedUntil = until
		g.blockReason = reason
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func errBackingOff(remaining time.Duration, reason string) error {
	return apperr.New("ANTIGRAVITY_BACKING_OFF").
		Causer("antigravity.guard.enter").
		Msgf("not calling Antigravity for another %s: %s", remaining.Round(time.Second), reason).
		Issue("retryAfterSeconds", int(remaining.Round(time.Second)/time.Second)).
		Issue("reason", reason).
		Status(apperr.StatusTooManyRequests).
		CTA(apperr.CallToAction{
			Label: "this build stops sending after the service refuses, rather than retrying into a suspension; wait for the window in the issue, or point this agent at another provider",
		})
}
