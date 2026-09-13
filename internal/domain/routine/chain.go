package routine

import "context"

// MaxChain is how many routine firings one outside event may set off in a row.
//
// A routine reacts to activities, and a routine's run publishes activities:
// its own routine.fired, and whatever its agent did — a task moved, a comment
// written. Delivery is inline, on the firing's context, so without a bound the
// reactive half of this package is a loop waiting for the trigger that closes
// it. "A routine ran" was that trigger: a routine listening for it heard its
// own firing and fired again, a paid model turn per iteration, for as long as
// the daemon stayed up.
//
// Refusing to fire a routine that is already in the chain is what stops a
// loop. This bound stops the other shape, a workspace of distinct routines
// that react to one another, from fanning out before it runs out of them.
const MaxChain = 4

// chainKey carries the routines whose firing led to the current context.
type chainKey struct{}

// withFiring records that r is firing, for everything its run and its
// publication do on the context this returns.
func withFiring(ctx context.Context, r *Routine) context.Context {
	prior := chainOf(ctx)
	next := make([]string, 0, len(prior)+1)
	next = append(next, prior...)
	next = append(next, chainLink(r))
	return context.WithValue(ctx, chainKey{}, next)
}

// chainOf is the routines already firing above this context, outermost first.
func chainOf(ctx context.Context) []string {
	chain, _ := ctx.Value(chainKey{}).([]string)
	return chain
}

// inChain reports whether r is one of the firings that led here.
func inChain(chain []string, r *Routine) bool {
	link := chainLink(r)
	for _, firing := range chain {
		if firing == link {
			return true
		}
	}
	return false
}

// chainLink names a routine by its owner as well as its id: the id alone is
// unique only within one agent's directory.
func chainLink(r *Routine) string { return r.Agent + "/" + r.ID }
