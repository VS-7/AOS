package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/sqlitequeue"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/runtime/worker"
)

type clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func openQueue(t *testing.T) (*sqlitequeue.Queue, *clock) {
	t.Helper()
	c := &clock{at: start}
	q, err := sqlitequeue.Open(sqlitequeue.Options{
		Path:  filepath.Join(t.TempDir(), "jobs.sqlite"),
		Clock: c.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	return q, c
}

// waitFor polls a condition rather than sleeping a fixed amount, so the test is
// fast when the pool is fast and does not flake when the machine is loaded.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func run(t *testing.T, p *worker.Pool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if err := p.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		if err := p.Stop(stopCtx); err != nil {
			t.Errorf("stopping the pool: %v", err)
		}
	})
}

// TestThePoolDrainsWhatIsQueued.
func TestThePoolDrainsWhatIsQueued(t *testing.T) {
	q, _ := openQueue(t)
	var handled atomic.Int64

	p := worker.New(worker.Deps{
		Queue:       q,
		Concurrency: 4,
		Idle:        time.Millisecond,
		TickRate:    time.Hour,
		Handlers: map[string]job.Handler{
			"turn": job.HandlerFunc(func(_ context.Context, j job.Job) (json.RawMessage, error) {
				handled.Add(1)
				return json.RawMessage(`{"ok":true}`), nil
			}),
		},
	})
	run(t, p)

	for i := range 20 {
		if _, err := q.Enqueue(context.Background(), job.Job{
			ID: fmt.Sprintf("j-%02d", i), Queue: job.QueueChat, Kind: "turn",
		}); err != nil {
			t.Fatal(err)
		}
	}

	waitFor(t, "every job to be handled", func() bool { return handled.Load() == 20 })
	waitFor(t, "every job to be marked done", func() bool {
		done, err := q.List(context.Background(), job.Filter{Status: job.Succeeded})
		return err == nil && len(done) == 20
	})
}

// TestAJobThatFailsIsRetriedAndThenDies, and the failure is on the record.
func TestAJobThatFailsIsRetriedAndThenDies(t *testing.T) {
	q, c := openQueue(t)
	var attempts atomic.Int64

	p := worker.New(worker.Deps{
		Queue:       q,
		Concurrency: 1,
		Idle:        time.Millisecond,
		TickRate:    time.Hour,
		Handlers: map[string]job.Handler{
			"flaky": job.HandlerFunc(func(context.Context, job.Job) (json.RawMessage, error) {
				attempts.Add(1)
				return nil, errors.New("the provider timed out")
			}),
		},
	})
	run(t, p)

	if _, err := q.Enqueue(context.Background(), job.Job{
		ID: "j-1", Queue: job.QueueChat, Kind: "flaky", MaxTries: 2,
	}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the first attempt", func() bool { return attempts.Load() >= 1 })
	// The retry sits behind a backoff, and the delay is computed when Fail
	// lands rather than when the handler returned. So the clock is moved on
	// every poll instead of once: advancing before Fail writes would only push
	// the retry further out.
	waitFor(t, "the second attempt", func() bool {
		c.advance(time.Minute)
		return attempts.Load() >= 2
	})

	waitFor(t, "the job to die", func() bool {
		got, err := q.Get(context.Background(), "j-1")
		return err == nil && got != nil && got.Status == job.Dead
	})
	got, err := q.Get(context.Background(), "j-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Error == "" {
		t.Fatal("a dead job records nothing about why")
	}
}

// TestAHandlerThatPanicsCostsThatJobAndNothingElse. A daemon that dies because
// one handler panicked takes every other job with it.
func TestAHandlerThatPanicsCostsThatJobAndNothingElse(t *testing.T) {
	q, _ := openQueue(t)
	var good atomic.Int64

	p := worker.New(worker.Deps{
		Queue:       q,
		Concurrency: 2,
		Idle:        time.Millisecond,
		TickRate:    time.Hour,
		Handlers: map[string]job.Handler{
			"explodes": job.HandlerFunc(func(context.Context, job.Job) (json.RawMessage, error) {
				panic("nil map write")
			}),
			"fine": job.HandlerFunc(func(context.Context, job.Job) (json.RawMessage, error) {
				good.Add(1)
				return nil, nil
			}),
		},
	})
	run(t, p)

	if _, err := q.Enqueue(context.Background(), job.Job{
		ID: "j-bad", Queue: job.QueueChat, Kind: "explodes", MaxTries: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Enqueue(context.Background(), job.Job{
		ID: "j-good", Queue: job.QueueChat, Kind: "fine",
	}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the healthy job to run", func() bool { return good.Load() == 1 })
	waitFor(t, "the panicking job to be recorded dead", func() bool {
		got, err := q.Get(context.Background(), "j-bad")
		return err == nil && got != nil && got.Status == job.Dead
	})
}

// TestAJobWithNoHandlerFailsWithAMessageThatNamesTheKind, which is what
// somebody debugging a version mismatch needs.
func TestAJobWithNoHandlerFailsWithAMessageThatNamesTheKind(t *testing.T) {
	q, _ := openQueue(t)
	p := worker.New(worker.Deps{
		Queue: q, Concurrency: 1, Idle: time.Millisecond, TickRate: time.Hour,
	})
	run(t, p)

	if _, err := q.Enqueue(context.Background(), job.Job{
		ID: "j-1", Queue: job.QueueChat, Kind: "from-the-future", MaxTries: 1,
	}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the job to be recorded dead", func() bool {
		got, err := q.Get(context.Background(), "j-1")
		return err == nil && got != nil && got.Status == job.Dead
	})
	got, _ := q.Get(context.Background(), "j-1")
	if got.Error == "" || !contains(got.Error, "from-the-future") {
		t.Fatalf("error = %q — it does not name the kind", got.Error)
	}
}

// TestTheTickRunsRecoveryBeforeTheRegisteredWork. Work handed back by a dead
// worker should be eligible before the tick creates more.
func TestTheTickRunsRecoveryBeforeTheRegisteredWork(t *testing.T) {
	q, c := openQueue(t)

	// A job left claimed by a worker that is gone.
	if _, err := q.Enqueue(context.Background(), job.Job{
		ID: "j-orphan", Queue: job.QueueChat, Kind: "turn",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Claim(context.Background(), nil, "w-dead", time.Minute); err != nil {
		t.Fatal(err)
	}
	c.advance(2 * time.Minute)

	var ticked atomic.Int64
	var pendingAtTick atomic.Int64
	p := worker.New(worker.Deps{
		Queue: q, Idle: time.Hour, TickRate: time.Hour,
		// The drain slots are pointed at a queue nothing is on, so the recovered
		// job is still pending when the tick counts it. Otherwise a worker
		// claims it first and the test measures the race rather than the order.
		Queues: []string{job.QueueTask},
		Ticks: []worker.Tick{{
			Name: "count",
			Run: func(ctx context.Context, _ time.Time) error {
				pending, err := q.List(ctx, job.Filter{Status: job.Pending})
				if err != nil {
					return err
				}
				pendingAtTick.Store(int64(len(pending)))
				ticked.Add(1)
				return nil
			},
		}},
		Handlers: map[string]job.Handler{
			"turn": job.HandlerFunc(func(context.Context, job.Job) (json.RawMessage, error) {
				// Never claimed: the pool below has no drain slots.
				return nil, nil
			}),
		},
	})
	run(t, p)

	waitFor(t, "the first tick", func() bool { return ticked.Load() >= 1 })
	if pendingAtTick.Load() != 1 {
		t.Fatalf("the tick saw %d pending jobs; recovery did not run first", pendingAtTick.Load())
	}
}

// TestAPeriodicTaskThatBlowsUpDoesNotStopTheTick.
func TestAPeriodicTaskThatBlowsUpDoesNotStopTheTick(t *testing.T) {
	q, _ := openQueue(t)
	var second atomic.Int64

	p := worker.New(worker.Deps{
		Queue: q, Idle: time.Hour, TickRate: time.Hour, Queues: []string{job.QueueTask},
		Ticks: []worker.Tick{
			{Name: "explodes", Run: func(context.Context, time.Time) error { panic("boom") }},
			{Name: "fine", Run: func(context.Context, time.Time) error { second.Add(1); return nil }},
		},
	})
	run(t, p)

	waitFor(t, "the task after the panicking one", func() bool { return second.Load() >= 1 })
}

// TestAPoolWithNoQueueRefusesToStart, rather than looking healthy and doing
// nothing.
func TestAPoolWithNoQueueRefusesToStart(t *testing.T) {
	p := worker.New(worker.Deps{})
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("a pool with nothing to drain started")
	}
}

// TestStartingTwiceIsRefused.
func TestStartingTwiceIsRefused(t *testing.T) {
	q, _ := openQueue(t)
	p := worker.New(worker.Deps{Queue: q, Concurrency: 1, Idle: time.Hour, TickRate: time.Hour})
	run(t, p)

	if err := p.Start(context.Background()); err == nil {
		t.Fatal("the pool started a second time over itself")
	}
}

// TestStoppingAPoolThatNeverStartedIsFine, because a shutdown path should not
// have to know whether the thing it is shutting down ever ran.
func TestStoppingAPoolThatNeverStartedIsFine(t *testing.T) {
	p := worker.New(worker.Deps{})
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestEachWorkerIdentifiesItself, so an operator can find the process holding a
// job that is stuck.
func TestEachWorkerIdentifiesItself(t *testing.T) {
	q, _ := openQueue(t)
	p := worker.New(worker.Deps{Queue: q, Concurrency: 1, Idle: time.Millisecond, TickRate: time.Hour})
	if p.Name() == "" {
		t.Fatal("the worker has no name")
	}

	other := worker.New(worker.Deps{Queue: q})
	if other.Name() == p.Name() {
		t.Fatal("two workers in one process share a name")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
