// Package worker drains the job queue and runs the periodic tick.
//
// It is the part of the system that makes autonomy real: without it a task
// assigned to an agent is a row in a file, and a routine with a cron is a
// promise. The shape is the original's — a bounded pool, a lease with a
// heartbeat, a fifteen-minute tick with recovery first — with the parameters
// configurable rather than fixed (ADR-0008).
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/core/safe"
	"github.com/OWNER/aos/internal/domain/job"
)

// Pool drains queues with a bounded number of concurrent handlers.
type Pool struct {
	queue    job.Queue
	handlers map[string]job.Handler
	log      *slog.Logger

	name        string
	queues      []string
	concurrency int
	lease       time.Duration
	heartbeat   time.Duration
	idle        time.Duration

	// ticks are the periodic jobs: the cron evaluation, the stale recovery,
	// whatever else the composition root registers.
	ticks    []Tick
	tickRate time.Duration

	mu      sync.Mutex
	running bool
	stop    context.CancelFunc
	done    chan struct{}
}

// Tick is periodic work that is not queued: something the daemon does on a
// schedule regardless of what is in the queue.
type Tick struct {
	Name string
	Run  func(ctx context.Context, now time.Time) error
}

// Deps is what the pool is built from.
type Deps struct {
	Queue    job.Queue
	Handlers map[string]job.Handler
	Ticks    []Tick
	Log      *slog.Logger

	// Name identifies this worker in a claim. It defaults to the hostname and
	// process id, which is what an operator needs to find the process holding
	// a job that is stuck.
	Name string

	// Queues limits which queues this pool drains. Empty drains all four.
	Queues []string

	Concurrency int
	Lease       time.Duration
	Heartbeat   time.Duration
	TickRate    time.Duration

	// Idle is how long to wait after finding nothing before asking again. It
	// is the polling interval, and it is short because the alternative — a
	// notification channel — buys latency this system does not need.
	Idle time.Duration
}

// New builds a pool.
func New(d Deps) *Pool {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	p := &Pool{
		queue:       d.Queue,
		handlers:    d.Handlers,
		ticks:       d.Ticks,
		log:         log,
		name:        d.Name,
		queues:      d.Queues,
		concurrency: d.Concurrency,
		lease:       d.Lease,
		heartbeat:   d.Heartbeat,
		tickRate:    d.TickRate,
		idle:        d.Idle,
	}
	if p.handlers == nil {
		p.handlers = map[string]job.Handler{}
	}
	if p.name == "" {
		host, _ := os.Hostname()
		p.name = host + ":" + strconv.Itoa(os.Getpid()) + ":" + ids.UUID{}.New()[:8]
	}
	if p.concurrency <= 0 {
		p.concurrency = job.DefaultConcurrency
	}
	if p.lease <= 0 {
		p.lease = job.DefaultLease
	}
	if p.heartbeat <= 0 {
		p.heartbeat = job.DefaultHeartbeat
	}
	if p.tickRate <= 0 {
		p.tickRate = job.DefaultTick
	}
	if p.idle <= 0 {
		p.idle = time.Second
	}
	return p
}

// Name is how this worker identifies itself in a claim.
func (p *Pool) Name() string { return p.name }

// Register adds a handler for a kind of job. It is safe only before Start.
func (p *Pool) Register(kind string, h job.Handler) { p.handlers[kind] = h }

// Start runs the pool until the context is cancelled or Stop is called.
//
// It returns immediately. The daemon owns the lifetime; a caller that wants to
// wait uses Stop, which blocks until every in-flight job has finished or its
// own context has expired.
func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return errors.New("worker: the pool is already running")
	}
	if p.queue == nil {
		p.mu.Unlock()
		return errors.New("worker: a pool with no queue has nothing to drain")
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	p.running, p.stop, p.done = true, cancel, make(chan struct{})
	done := p.done
	p.mu.Unlock()

	var wg sync.WaitGroup
	for i := range p.concurrency {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			p.drain(runCtx, n)
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.tickLoop(runCtx)
	}()

	// The cancellation of the caller's context stops the pool, so a daemon that
	// shuts down does not have to remember to call Stop as well.
	go func() {
		<-ctx.Done()
		cancel()
	}()
	go func() {
		wg.Wait()
		close(done)
	}()
	return nil
}

// Stop cancels the pool and waits for it to finish.
func (p *Pool) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	stop, done := p.stop, p.done
	p.running = false
	p.mu.Unlock()

	stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// The jobs still running keep their lease; when it lapses another
		// worker recovers them. Saying so is better than blocking forever.
		return ctx.Err()
	}
}

// drain is one worker: claim, run, report, repeat.
func (p *Pool) drain(ctx context.Context, n int) {
	log := p.log.With("worker", p.name, "slot", n)
	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := p.queue.Claim(ctx, p.queues, p.name, p.lease)
		if err != nil {
			if ctx.Err() != nil {
				// The pool is shutting down. A cancelled claim is the expected
				// end of this loop, not an incident, and logging it once per
				// slot buries whatever the real reason for the shutdown was.
				return
			}
			// A queue that cannot be read is usually a database that went away.
			// Backing off is right: hammering it produces a log nobody can read
			// and does not fix it.
			log.Error("could not claim a job", "err", err)
			if !sleep(ctx, p.idle) {
				return
			}
			continue
		}
		if claimed == nil {
			if !sleep(ctx, p.idle) {
				return
			}
			continue
		}
		p.run(ctx, log, *claimed)
	}
}

// run executes one job under a heartbeat, and reports the outcome whichever way
// it went.
func (p *Pool) run(ctx context.Context, log *slog.Logger, j job.Job) {
	handler, ok := p.handlers[j.Kind]
	if !ok {
		// A job whose handler this build does not have is not a transient
		// failure and retrying it wastes the attempts. It is failed with a
		// message that names the kind, which is what somebody debugging a
		// version mismatch needs.
		err := errors.New("no handler is registered for " + strconv.Quote(j.Kind))
		p.report(ctx, log, j, nil, err, noRetry())
		return
	}

	beat, stopBeat := context.WithCancel(ctx)
	defer stopBeat()
	go p.beat(beat, log, j.ID)

	var result json.RawMessage
	err := safe.Do(ctx, "worker.handle:"+j.Kind, func(ctx context.Context) error {
		var handleErr error
		result, handleErr = handler.Handle(ctx, j)
		return handleErr
	})
	stopBeat()

	p.report(ctx, log, j, result, err, nil)
}

// beat renews the lease while a job runs. A worker that dies stops beating, the
// lease lapses, and RecoverStale hands the job back.
func (p *Pool) beat(ctx context.Context, log *slog.Logger, jobID string) {
	ticker := time.NewTicker(p.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// The parent context is used, not the beat context: the write must
			// not be cancelled by the very shutdown that makes it matter.
			if err := p.queue.Heartbeat(context.WithoutCancel(ctx), jobID, p.name, p.lease); err != nil {
				log.Warn("could not extend the lease on a running job", "job", jobID, "err", err)
				return
			}
		}
	}
}

// report closes out a job. It writes on a context detached from the run's, so a
// daemon shutting down still records what its last job did.
func (p *Pool) report(ctx context.Context, log *slog.Logger, j job.Job, result json.RawMessage, err error, retryIn *time.Duration) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err == nil {
		if completeErr := p.queue.Complete(writeCtx, j.ID, result); completeErr != nil {
			log.Error("a job finished and could not be marked done", "job", j.ID, "err", completeErr)
		}
		return
	}
	log.Warn("a job failed", "job", j.ID, "kind", j.Kind, "attempt", j.Attempts, "err", err)
	if failErr := p.queue.Fail(writeCtx, j.ID, err, retryIn); failErr != nil {
		log.Error("a job failed and the failure could not be recorded", "job", j.ID, "err", failErr)
	}
}

// tickLoop runs the periodic work: recovery first, then everything registered.
//
// Recovery goes first because the original does it that way and the reason is
// sound: work handed back by a dead worker should be eligible before the tick
// creates more.
func (p *Pool) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(p.tickRate)
	defer ticker.Stop()

	// One pass immediately, so a daemon that starts after a crash recovers the
	// work its predecessor was holding without waiting a full tick.
	p.tickOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_ = now
			p.tickOnce(ctx)
		}
	}
}

// TickOnce runs the periodic pass once, for a test or an operator who does not
// want to wait fifteen minutes.
func (p *Pool) TickOnce(ctx context.Context) { p.tickOnce(ctx) }

func (p *Pool) tickOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if n, err := p.queue.RecoverStale(ctx); err != nil {
		if ctx.Err() != nil {
			return
		}
		p.log.Error("could not recover stale jobs", "err", err)
	} else if n > 0 {
		p.log.Warn("jobs were handed back after their worker stopped reporting", "count", n)
	}

	now := time.Now() //nolint:forbidigo // the tick is the wall clock by definition; handlers take theirs by injection
	for _, tick := range p.ticks {
		err := safe.Do(ctx, "worker.tick:"+tick.Name, func(ctx context.Context) error {
			return tick.Run(ctx, now)
		})
		if err != nil {
			p.log.Error("a periodic task failed", "tick", tick.Name, "err", err)
		}
	}
}

// sleep waits, and reports whether the wait completed rather than being
// cancelled.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// noRetry is a zero delay used to mark a failure that should not be retried by
// waiting. The queue still counts the attempt, so a job with no handler dies
// after its allowance rather than spinning.
func noRetry() *time.Duration {
	d := time.Duration(0)
	return &d
}
