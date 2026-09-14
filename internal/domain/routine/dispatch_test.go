package routine

import (
	"context"
	"testing"
	"time"
)

// dispatcher records the firings handed to it, as the queue would hold them.
type dispatcher struct {
	firings []Firing
	busy    map[string]bool
}

func (d *dispatcher) Dispatch(_ context.Context, f Firing) (bool, error) {
	if d.busy[f.Routine] {
		return false, nil
	}
	d.firings = append(d.firings, f)
	return true, nil
}

// A scheduled routine's run is a whole turn, and the tick that found it due
// used to take that turn itself: every other due routine, in this workspace and
// the next, and both retention passes waited for it, and the ticks that passed
// meanwhile were dropped. The tick now hands the firing on and returns, and
// whoever runs it later fires it as a scheduled run.
func TestADueRoutineIsHandedOnRatherThanRunInTheTick(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{
		Name:     "Every minute",
		Triggers: []TriggerInput{{Type: Scheduled, Cron: "* * * * *"}},
	})
	d := &dispatcher{}

	got, err := h.svc.DispatchScheduled(asAgent("atlas"), start.Add(15*time.Minute), d)
	if err != nil {
		t.Fatal(err)
	}
	if n := h.executor.count(); n != 0 {
		t.Fatalf("the tick ran the routine %d times itself", n)
	}
	want := Firing{Agent: "atlas", Routine: out.Routine.ID, Cron: "* * * * *"}
	if len(d.firings) != 1 || d.firings[0] != want {
		t.Fatalf("handed on %+v, want [%+v]", d.firings, want)
	}
	if len(got.Fired) != 1 || got.Fired[0] != out.Routine.ID {
		t.Fatalf("fired = %v", got.Fired)
	}

	run, err := h.svc.Fire(asAgent("atlas"), d.firings[0].Input())
	if err != nil {
		t.Fatal(err)
	}
	if run.Trigger != Scheduled || run.Payload["cron"] != "* * * * *" || h.executor.count() != 1 {
		t.Fatalf("the handed-on firing ran as %+v (%d executions), want one scheduled run", run, h.executor.count())
	}
}

// A routine whose last firing is still waiting or running is not handed on a
// second time: a turn longer than the routine's interval does not pile up
// copies of itself behind it.
func TestADueRoutineStillRunningIsNotHandedOnAgain(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{
		Name:     "Every minute",
		Triggers: []TriggerInput{{Type: Scheduled, Cron: "* * * * *"}},
	})
	d := &dispatcher{busy: map[string]bool{out.Routine.ID: true}}

	got, err := h.svc.DispatchScheduled(asAgent("atlas"), start.Add(15*time.Minute), d)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.firings) != 0 || len(got.Fired) != 0 {
		t.Fatalf("handed on %v and reported %v fired while the last firing was still running", d.firings, got.Fired)
	}
	if len(got.Running) != 1 || got.Running[0] != out.Routine.ID {
		t.Fatalf("running = %v, want the routine reported as still running", got.Running)
	}
}

// cancelling is a runtime whose turn is cut short the way a daemon shutting
// down cuts it: the context goes while the turn runs.
type cancelling struct{ cancel context.CancelFunc }

func (c cancelling) Execute(ctx context.Context, _ Execution) (Outcome, error) {
	c.cancel()
	<-ctx.Done()
	return Outcome{}, ctx.Err()
}

// The worker stops a scheduled run when the daemon shuts down, and fire closed
// the run record and marked the routine fired on that same cancelled context:
// both writes failed, the run read as running for good, and the routine did not
// know it had fired. On instance 41 the run cut short at 21:33:11 still said
// running after the daemon came back.
func TestARunCutShortIsStillClosedAndRecordedOnTheRoutine(t *testing.T) {
	ctx, cancel := context.WithCancel(asAgent("atlas"))
	defer cancel()
	h := newHarness(t, func(d *Deps) { d.Executor = cancelling{cancel: cancel} })
	out := h.create(t, CreateInput{
		Name:     "Cut short",
		Triggers: []TriggerInput{{Type: Scheduled, Cron: "* * * * *"}},
	})

	if _, err := h.svc.Fire(ctx, Firing{Agent: "atlas", Routine: out.Routine.ID, Cron: "* * * * *"}.Input()); err == nil {
		t.Fatal("a run whose context went reported success")
	}
	runs, err := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs.Runs) != 1 || runs.Runs[0].Status == RunRunning || runs.Runs[0].EndedAt == nil {
		t.Fatalf("runs = %+v, want the one run closed", runs.Runs)
	}
	view, err := h.svc.Get(asAgent("atlas"), GetInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if view.LastFiredAt == nil {
		t.Fatal("the routine does not know it fired")
	}
}
