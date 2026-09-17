package routine

import (
	"context"
	"strings"
)

// MaxChain is how many routine firings one outside event may set off in a row.
//
// A routine reacts to activities, and a routine's run publishes activities:
// its own routine.fired, and whatever its agent did — a task moved, a comment
// written. Without a bound the reactive half of this package is a loop waiting
// for the trigger that closes it. "A routine ran" was that trigger: a routine
// listening for it heard its own firing and fired again, a paid model turn per
// iteration, for as long as the daemon stayed up.
//
// Refusing to fire a routine that is already in the chain is what stops a
// loop. This bound stops the other shape, a workspace of distinct routines
// that react to one another, from fanning out before it runs out of them.
// Activities of the routine namespace are bound tighter still: see
// Chain.Echoed.
const MaxChain = 4

// routineNamespace is where a routine's own activities are published.
const routineNamespace = "routine"

// Chain is the routine firings that led to where an activity is published.
//
// It lives on the context while a firing runs, and it travels with a firing
// handed on to run later (Firing.Chain): the bound is on what one outside
// event sets off, and a firing taken off a queue is still part of it.
type Chain struct {
	// Firings names each routine firing above this point, outermost first.
	Firings []string `json:"firings,omitempty"`

	// Echoed is whether one of those firings was set off by a routine
	// activity — somebody's routine.fired. Such a chain sets nothing else off
	// through the routine namespace.
	//
	// Excluding the routines already in the chain was not enough there. Every
	// listener hears every other listener's firing, so k routines waiting for
	// routine.fired turned one run into k + k(k-1) + k(k-1)(k-2) more before
	// MaxChain stopped them: four for two listeners, a hundred and fifty-six
	// paid turns for six. Hearing that a routine ran is one level of reaction.
	Echoed bool `json:"echoed,omitempty"`
}

// chainKey carries the Chain of the current context.
type chainKey struct{}

// withFiring records that r is firing, set off by an activity of namespace
// ("" when no activity set it off), for everything its run and its
// publication do on the context this returns.
func withFiring(ctx context.Context, r *Routine, namespace string) context.Context {
	prior := chainOf(ctx)
	next := Chain{
		Firings: make([]string, 0, len(prior.Firings)+1),
		Echoed:  prior.Echoed || strings.EqualFold(namespace, routineNamespace),
	}
	next.Firings = append(next.Firings, prior.Firings...)
	next.Firings = append(next.Firings, chainLink(r))
	return withChain(ctx, next)
}

// withChain restores a chain that was carried off the context it began on.
func withChain(ctx context.Context, c Chain) context.Context {
	return context.WithValue(ctx, chainKey{}, c)
}

// chainOf is the chain of firings already above this context.
func chainOf(ctx context.Context) Chain {
	chain, _ := ctx.Value(chainKey{}).(Chain)
	return chain
}

// stops reports why an activity of namespace published on this chain fires no
// routine at all, or "" when it may.
func (c Chain) stops(namespace string) string {
	switch {
	case len(c.Firings) >= MaxChain:
		return "the chain of routine firings is at its limit"
	case c.Echoed && strings.EqualFold(namespace, routineNamespace):
		return "a routine firing that reacted to another routine's firing sets off no further routine"
	}
	return ""
}

// includes reports whether r is one of the firings that led here.
func (c Chain) includes(r *Routine) bool {
	link := chainLink(r)
	for _, firing := range c.Firings {
		if firing == link {
			return true
		}
	}
	return false
}

// chainLink names a routine by its owner as well as its id: the id alone is
// unique only within one agent's directory.
func chainLink(r *Routine) string { return r.Agent + "/" + r.ID }
