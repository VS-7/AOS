package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// The worker was built and never started. wire.go made the pool with the
// routine, activity-retention and job-retention ticks, and nothing outside a
// test called Start: in a real daemon no scheduled routine ever fired, no
// retention ran, and work a dead worker held was never recovered.
//
// Starting only the primary's pool would not have been the fix either. In the
// desktop the person's workspace is usually not the directory the daemon
// opened but a workspace it serves beside it, and the primary's routine tick
// read the primary's routines only. So this builds the daemon the way `aosd
// serve` does — New, then Serve, and nothing else — with the routine in a
// second workspace, and waits for it to fire there.
func TestTheServingDaemonFiresScheduledRoutinesInEveryWorkspace(t *testing.T) {
	home, first, second, third := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome:       home,
			env.KeyServerPort: strconv.Itoa(freePort(t)),
			// A window a day wide, so the pass the worker makes the moment it
			// starts finds an every-minute cron due whatever second this runs
			// at, and no second tick comes while the test is looking.
			env.KeyJobsTick: "24h",
		})),
		WorkspaceRoot: first,
		// Records are stamped an hour ago, so a routine created a moment
		// before the worker's first pass is not younger than the minute that
		// pass finds due: a routine never fires for a slot before it existed.
		Clock: anHourAgo{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	ctx := context.Background()
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: "Second", Path: second}); err != nil {
		t.Fatal(err)
	}
	routineIn := func(id string) string {
		var out struct {
			Routine struct {
				ID string `json:"id"`
			} `json:"routine"`
		}
		raw := invokeIn(inWorkspace(id), t, a, "routines_create", `{
			"_reasoning": "a test is checking that a schedule fires where it was written",
			"name": "Every minute", "agent": "atlas",
			"triggers": [{"type": "scheduled", "cron": "* * * * *"}],
			"content": "Say what time it is."
		}`)
		if err := json.Unmarshal(raw, &out); err != nil || out.Routine.ID == "" {
			t.Fatalf("routines_create answered %s (%v)", raw, err)
		}
		return out.Routine.ID
	}
	runsOf := func(workspaceID, routineID string) int {
		var out struct {
			Total int `json:"total"`
		}
		raw := invokeIn(inWorkspace(workspaceID), t, a, "routines_runs",
			`{"_reasoning":"a test is checking whether the schedule fired","id":"`+routineID+`"}`)
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out.Total
	}
	early := routineIn("second")

	serveCtx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan struct{})
	go func() {
		done <- a.Serve(serveCtx, app.ServeOptions{
			Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
			ShutdownTimeout: 5 * time.Second,
			Ready:           func(string) { close(ready) },
		})
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("the daemon stopped before it served: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon never started serving")
	}

	waitFor(t, "the routine in the second workspace to fire", func() bool {
		return runsOf("second", early) > 0
	})

	// A workspace registered while the daemon runs is served by the next tick,
	// not only the ones there at boot. TickOnce is that next tick, without
	// waiting a day for it.
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: "Third", Path: third}); err != nil {
		t.Fatal(err)
	}
	late := routineIn("third")
	a.Worker.TickOnce(ctx)
	waitFor(t, "the routine in a workspace registered after boot to fire", func() bool {
		return runsOf("third", late) > 0
	})

	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the daemon reported an error on shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon did not shut down")
	}
	// Stopped with the daemon, not left draining the queue behind it: a pool
	// still running refuses a second start.
	restart, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := a.Worker.Start(restart); err != nil {
		t.Fatalf("the worker was still running after the daemon stopped: %v", err)
	}
	_ = a.Worker.Stop(context.Background())
}

// anHourAgo is the wall clock an hour behind.
type anHourAgo struct{}

func (anHourAgo) Now() time.Time { return time.Now().Add(-time.Hour) }

// A scheduled routine's run is a whole turn, and the worker's tick took it
// inline: one long run held up every other workspace's due routines and both
// retention passes, and the ticks that passed meanwhile were dropped. The tick
// now puts each due firing on the queue and returns, the pool's slots run
// them, and a routine whose firing is still waiting is not queued twice.
func TestTheTickQueuesScheduledRunsForThePoolToRun(t *testing.T) {
	home, first, second := t.TempDir(), t.TempDir(), t.TempDir()
	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome:     home,
			env.KeyJobsTick: "24h",
		})),
		WorkspaceRoot: first,
		Clock:         anHourAgo{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	ctx := context.Background()
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{Name: "Second", Path: second}); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Routine struct {
			ID string `json:"id"`
		} `json:"routine"`
	}
	raw := invokeIn(inWorkspace("second"), t, a, "routines_create", `{
		"_reasoning": "a test is checking where a scheduled run is taken",
		"name": "Every minute", "agent": "atlas",
		"triggers": [{"type": "scheduled", "cron": "* * * * *"}],
		"content": "Say what time it is."
	}`)
	if err := json.Unmarshal(raw, &out); err != nil || out.Routine.ID == "" {
		t.Fatalf("routines_create answered %s (%v)", raw, err)
	}
	runs := func() int {
		var got struct {
			Total int `json:"total"`
		}
		raw := invokeIn(inWorkspace("second"), t, a, "routines_runs",
			`{"_reasoning":"a test is checking whether the schedule fired","id":"`+out.Routine.ID+`"}`)
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		return got.Total
	}
	queued := func() []job.Job {
		listed, err := a.Queue.List(ctx, job.Filter{Kind: "aos.routine"})
		if err != nil {
			t.Fatal(err)
		}
		return listed
	}

	// The pool is not draining: whatever the tick does not queue, it ran.
	a.Worker.TickOnce(ctx)
	if n := runs(); n != 0 {
		t.Fatalf("the tick ran the routine itself (%d runs) instead of queueing it", n)
	}
	jobs := queued()
	if len(jobs) != 1 || jobs[0].Workspace != "second" || jobs[0].Queue != job.QueueRoutine || jobs[0].Status != job.Pending {
		t.Fatalf("queued = %+v, want one pending routine job for the second workspace", jobs)
	}
	a.Worker.TickOnce(ctx)
	if jobs := queued(); len(jobs) != 1 {
		t.Fatalf("a firing still waiting was queued again: %d jobs", len(jobs))
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := a.Worker.Start(runCtx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = a.Worker.Stop(stopCtx)
	}()
	waitFor(t, "the pool to run the queued firing", func() bool { return runs() > 0 })
	waitFor(t, "the routine job to finish", func() bool {
		jobs := queued()
		return len(jobs) >= 1 && jobs[len(jobs)-1].EndedAt != nil
	})
}
