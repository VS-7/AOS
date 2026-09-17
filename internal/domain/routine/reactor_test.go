package routine

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
)

// queueing is a Reactor over an in-memory queue: it takes every firing, as a
// daemon whose worker drains the queue does, and runs nothing until drained.
// Each firing goes through JSON, the way a job's payload does.
type queueing struct {
	mu       sync.Mutex
	waiting  [][]byte
	declines bool
	fails    error
}

func (q *queueing) React(_ context.Context, f Firing) (bool, error) {
	if q.fails != nil {
		return false, q.fails
	}
	if q.declines {
		return false, nil
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.waiting = append(q.waiting, raw)
	return true, nil
}

func (q *queueing) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.waiting)
}

// drain takes the firings off the queue until nothing is left, the way the
// worker's slots would, and fails the test if the queue never empties.
func (q *queueing) drain(t *testing.T, svc *Service) {
	t.Helper()
	for taken := 0; ; taken++ {
		if taken > deliveryCeiling {
			t.Fatalf("the queue never emptied after %d firings", taken)
		}
		q.mu.Lock()
		if len(q.waiting) == 0 {
			q.mu.Unlock()
			return
		}
		raw := q.waiting[0]
		q.waiting = q.waiting[1:]
		q.mu.Unlock()

		var f Firing
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		// As the system, the way the worker takes a job: nobody's identity.
		if _, err := svc.Take(context.Background(), f); err != nil {
			t.Fatalf("taking %s: %v", raw, err)
		}
	}
}

func withReactor(h *harness, r Reactor) {
	h.svc.reactor = r
}

// TestAnActivityHandsItsFiringsOnInsteadOfRunningThem. A firing is a whole
// turn, and it ran inside the mutation that published the activity: a person
// moving a task waited for every routine the move set off.
func TestAnActivityHandsItsFiringsOnInsteadOfRunningThem(t *testing.T) {
	h := newHarness(t)
	queue := &queueing{}
	withReactor(h, queue)
	review := []TriggerInput{{Type: Activity, Namespace: "task", Event: "status_changed"}}
	h.create(t, CreateInput{Name: "Check the evidence", Triggers: review})
	h.create(t, CreateInput{Name: "Tidy the plan", Triggers: review})

	h.svc.OnActivity(asAgent("atlas"), "task", "status_changed", map[string]any{"to": "in_review"})

	if got := h.executor.count(); got != 0 {
		t.Fatalf("the publication ran %d turns before returning", got)
	}
	if got := queue.size(); got != 2 {
		t.Fatalf("%d firings were handed on, want one per routine", got)
	}

	queue.drain(t, h.svc)
	if got := h.executor.count(); got != 2 {
		t.Fatalf("the queue ran %d turns, want 2", got)
	}
	for _, call := range h.executor.calls {
		if call.Trigger != Activity || call.Payload["namespace"] != "task" || call.Payload["event"] != "status_changed" {
			t.Fatalf("a queued firing ran as %s with %v, want the activity it reacted to", call.Trigger, call.Payload)
		}
		data, _ := call.Payload["data"].(map[string]any)
		if data["to"] != "in_review" {
			t.Fatalf("the activity's data was lost on the way: %v", call.Payload)
		}
	}
	// The run is the routine's own agent's, whoever took the job.
	for _, actor := range h.executor.actors {
		if actor != "atlas" {
			t.Fatalf("a queued firing ran as %q", actor)
		}
	}
}

// TestRunNowDoesNotWaitForTheRoutinesThatHearIt. Run now on a routine others
// listen for waited for every listener's turn. Handed on, it returns with its
// own run, and the listeners still react one level deep once the queue runs.
func TestRunNowDoesNotWaitForTheRoutinesThatHearIt(t *testing.T) {
	for _, listeners := range []int{1, 2, 3} {
		t.Run(strconv.Itoa(listeners)+" listeners", func(t *testing.T) {
			h := newHarness(t)
			withPublishing(h)
			queue := &queueing{}
			withReactor(h, queue)
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
			if got := h.executor.count(); got != 1 {
				t.Fatalf("Run now waited for %d turns, want only its own", got)
			}
			queue.drain(t, h.svc)
			if got, want := h.executor.count(), 1+listeners; got != want {
				t.Fatalf("one firing with %d listeners set off %d runs through the queue, want %d",
					listeners, got, want)
			}
		})
	}
}

// TestACycleThroughTheQueueStopsWhereItWouldRepeat. The guards on what one
// outside event sets off read the chain on the context, and a firing taken off
// a queue starts on a fresh one. Unless the chain travels with the firing, two
// routines whose runs each move a task set each other off again, one job at a
// time, for as long as the worker runs.
func TestACycleThroughTheQueueStopsWhereItWouldRepeat(t *testing.T) {
	h := newHarness(t)
	exec := &reacting{svc: h.svc}
	h.svc.executor = exec
	queue := &queueing{}
	withReactor(h, queue)
	review := []TriggerInput{{Type: Activity, Namespace: "task", Event: "status_changed"}}
	h.create(t, CreateInput{Name: "Check the evidence", Triggers: review})
	h.create(t, CreateInput{Name: "Tidy the plan", Triggers: review})

	h.svc.OnActivity(asAgent("atlas"), "task", "status_changed", map[string]any{"to": "in_review"})
	queue.drain(t, h.svc)

	// The same four runs as inline delivery: each routine, and the other once.
	if got := exec.count(); got != 4 {
		t.Fatalf("one task change set off %d runs through the queue", got)
	}
}

// TestAFiringNothingWouldRunIsTakenWhereTheActivityWasPublished. A process
// that drains no queue — a command run on its own — or a queue that refuses
// the job must not lose the reaction: it runs inline, as before.
func TestAFiringNothingWouldRunIsTakenWhereTheActivityWasPublished(t *testing.T) {
	for name, queue := range map[string]*queueing{
		"no worker":     {declines: true},
		"queue refused": {fails: errors.New("database is locked")},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withReactor(h, queue)
			h.create(t, CreateInput{
				Name:     "Check the evidence",
				Triggers: []TriggerInput{{Type: Activity, Namespace: "task", Event: "status_changed"}},
			})

			h.svc.OnActivity(asAgent("atlas"), "task", "status_changed", map[string]any{"to": "in_review"})

			if got := h.executor.count(); got != 1 {
				t.Fatalf("the reaction ran %d times, want once, here", got)
			}
		})
	}
}
