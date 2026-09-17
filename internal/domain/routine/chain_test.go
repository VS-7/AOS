package routine

import (
	"context"
	"strconv"
	"sync"
	"testing"
)

// publishing is the notifier the daemon wires: a firing is published as the
// activity routine.fired, and the activity log delivers it back to OnActivity
// inline, on the same context. It stops delivering after a ceiling so a test
// of the loop fails instead of recursing until the stack runs out.
type publishing struct {
	mu        sync.Mutex
	svc       *Service
	delivered int
}

const deliveryCeiling = 50

func (p *publishing) RoutineFired(ctx context.Context, r *Routine, run *Run) {
	p.mu.Lock()
	p.delivered++
	over := p.delivered > deliveryCeiling
	p.mu.Unlock()
	if over {
		return
	}
	p.svc.OnActivity(ctx, "routine", "fired", map[string]any{
		"routine": r.ID, "agent": r.Agent, "run": run.ID, "status": string(run.Status),
	})
}

func withPublishing(h *harness) *publishing {
	p := &publishing{svc: h.svc}
	h.svc.notifier = p
	return p
}

// TestARoutineReactingToRoutineFiringsDoesNotFireItselfForever is the loop the
// trigger catalogue offered: "a routine ran" fired the routine that listened
// for it, whose own firing was a routine running, and so on for as long as the
// daemon was up — a paid model turn per iteration.
func TestARoutineReactingToRoutineFiringsDoesNotFireItselfForever(t *testing.T) {
	h := newHarness(t)
	notifier := withPublishing(h)

	h.create(t, CreateInput{
		Name:     "Tell me when anything ran",
		Triggers: []TriggerInput{{Type: Activity, Namespace: "routine", Event: "fired"}},
	})
	other := h.create(t, CreateInput{Name: "The nightly sweep"})

	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: other.Routine.ID}); err != nil {
		t.Fatal(err)
	}

	// The sweep ran, and the listener heard about it once. The listener's own
	// firing is a routine running too, and it must not hear itself.
	if got := h.executor.count(); got != 2 {
		t.Fatalf("one firing set off %d runs (%d deliveries)", got, notifier.delivered)
	}
}

// TestFiringTheListenerByHandDoesNotMakeItHearItself.
func TestFiringTheListenerByHandDoesNotMakeItHearItself(t *testing.T) {
	h := newHarness(t)
	withPublishing(h)

	listener := h.create(t, CreateInput{
		Name:     "Tell me when anything ran",
		Triggers: []TriggerInput{{Type: Activity, Namespace: "routine", Event: "fired"}},
	})
	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: listener.Routine.ID}); err != nil {
		t.Fatal(err)
	}
	if got := h.executor.count(); got != 1 {
		t.Fatalf("firing the listener by hand ran it %d times", got)
	}
}

// reacting is an executor whose run publishes the activity its routine reacts
// to, the way a routine that moves a task publishes task.status_changed.
type reacting struct {
	executor
	svc *Service
}

func (e *reacting) Execute(ctx context.Context, req Execution) (Outcome, error) {
	out, err := e.executor.Execute(ctx, req)
	if e.count() <= deliveryCeiling {
		e.svc.OnActivity(ctx, "task", "status_changed", map[string]any{"to": "in_review"})
	}
	return out, err
}

// TestACycleThroughAnotherNamespaceStopsWhereItWouldRepeat. A routine whose
// run changes what it reacts to, or two routines that feed each other, are the
// same loop without routine.fired in it: nothing a routine does while it runs
// can fire that routine again.
func TestACycleThroughAnotherNamespaceStopsWhereItWouldRepeat(t *testing.T) {
	h := newHarness(t)
	exec := &reacting{svc: h.svc}
	h.svc.executor = exec

	reviewTrigger := []TriggerInput{{Type: Activity, Namespace: "task", Event: "status_changed"}}
	h.create(t, CreateInput{Name: "Check the evidence", Triggers: reviewTrigger})
	h.create(t, CreateInput{Name: "Tidy the plan", Triggers: reviewTrigger})

	// A person moves a task: both routines react. Each one's run moves a task
	// again, which fires the other — once — and then nothing that has not
	// already run in that chain is left to fire.
	h.svc.OnActivity(asAgent("atlas"), "task", "status_changed", map[string]any{"to": "in_review"})

	if got := exec.count(); got != 4 {
		t.Fatalf("one task change set off %d runs", got)
	}
}

// TestAChainOfDistinctRoutinesIsBounded. Refusing repeats bounds a chain by the
// number of routines, and a workspace with many that react to one another
// would still fan out; the chain stops at MaxChain firings regardless.
func TestAChainOfDistinctRoutinesIsBounded(t *testing.T) {
	h := newHarness(t)
	exec := &reacting{svc: h.svc}
	h.svc.executor = exec

	h.create(t, CreateInput{Name: "React", Triggers: []TriggerInput{{Type: Activity, Namespace: "task", Event: "status_changed"}}})

	ctx := asAgent("atlas")
	for i := 0; i < MaxChain; i++ {
		ctx = withFiring(ctx, &Routine{Agent: "atlas", ID: "outer-" + string(rune('a'+i))}, "")
	}
	h.svc.OnActivity(ctx, "task", "status_changed", map[string]any{"to": "in_review"})
	if got := exec.count(); got != 0 {
		t.Fatalf("an activity %d firings deep still fired %d routines", MaxChain, got)
	}
}

// TestACancelledCallerStopsTheReaction. The desktop bridge's context was never
// cancelled, so this is not what stopped the loop — but a caller that has gone
// away must not keep starting runs on its behalf.
func TestACancelledCallerStopsTheReaction(t *testing.T) {
	h := newHarness(t)
	h.create(t, CreateInput{Name: "React", Triggers: []TriggerInput{{Type: Activity, Namespace: "task"}}})

	ctx, cancel := context.WithCancel(asAgent("atlas"))
	cancel()
	h.svc.OnActivity(ctx, "task", "created", nil)

	if got := h.executor.count(); got != 0 {
		t.Fatalf("a cancelled caller started %d runs", got)
	}
}

// TestRoutineFiringsAreReactedToOneLevelDeep. Every routine listening for
// routine.fired heard the firing that started a chain, and then each one's own
// firing too, all but the routines already in that branch: k listeners made
// k + k(k-1) + k(k-1)(k-2) runs of one Run now — four for two, fifteen for
// three, a hundred and fifty-six paid turns for six. Hearing that a routine
// ran is one level of reaction; a reaction's own firing sets nothing else off.
func TestRoutineFiringsAreReactedToOneLevelDeep(t *testing.T) {
	for _, listeners := range []int{2, 3} {
		t.Run(strconv.Itoa(listeners)+" listeners", func(t *testing.T) {
			h := newHarness(t)
			withPublishing(h)
			for i := range listeners {
				h.create(t, CreateInput{
					Name:     "Tell me when anything ran " + strconv.Itoa(i),
					Triggers: []TriggerInput{{Type: Activity, Namespace: "routine", Event: "fired"}},
				})
			}
			sweep := h.create(t, CreateInput{Name: "The nightly sweep"})

			if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: sweep.Routine.ID}); err != nil {
				t.Fatal(err)
			}
			if got, want := h.executor.count(), 1+listeners; got != want {
				t.Fatalf("one firing with %d listeners set off %d runs, want %d: the sweep and each listener once",
					listeners, got, want)
			}
		})
	}
}

// TestAFiringAnotherActivitySetOffIsStillHeard. The bound is on reacting to a
// reaction to a routine firing, not on a routine that ran because a task moved:
// "tell me when anything ran" still hears that one.
func TestAFiringAnotherActivitySetOffIsStillHeard(t *testing.T) {
	h := newHarness(t)
	withPublishing(h)
	h.create(t, CreateInput{
		Name:     "Tell me when anything ran",
		Triggers: []TriggerInput{{Type: Activity, Namespace: "routine", Event: "fired"}},
	})
	h.create(t, CreateInput{
		Name:     "Check the evidence",
		Triggers: []TriggerInput{{Type: Activity, Namespace: "task", Event: "status_changed"}},
	})

	h.svc.OnActivity(asAgent("atlas"), "task", "status_changed", map[string]any{"to": "in_review"})

	if got := h.executor.count(); got != 2 {
		t.Fatalf("a task change set off %d runs, want the checker and the listener that heard it", got)
	}
}
