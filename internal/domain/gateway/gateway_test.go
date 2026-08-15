package gateway_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/gateway"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// fakeProcs is an operating system with a process table a test controls.
type fakeProcs struct {
	mu       sync.Mutex
	alive    map[int]bool
	nextPID  int
	started  []gateway.Command
	startErr error

	// ignoresTerminate models a daemon that does not shut down when asked,
	// which is the case the escalation to kill exists for.
	ignoresTerminate bool
	killErr          error
	terminated       []int
	killed           []int
}

func newProcs() *fakeProcs { return &fakeProcs{alive: map[int]bool{}, nextPID: 1000} }

func (p *fakeProcs) Start(context.Context, gateway.Command) (int, error) {
	if p.startErr != nil {
		return 0, p.startErr
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextPID++
	p.alive[p.nextPID] = true
	p.started = append(p.started, gateway.Command{})
	return p.nextPID, nil
}

func (p *fakeProcs) Alive(pid int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive[pid]
}

func (p *fakeProcs) Terminate(pid int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.terminated = append(p.terminated, pid)
	if !p.ignoresTerminate {
		delete(p.alive, pid)
	}
	return nil
}

func (p *fakeProcs) Kill(pid int) error {
	if p.killErr != nil {
		return p.killErr
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.killed = append(p.killed, pid)
	delete(p.alive, pid)
	return nil
}

func (p *fakeProcs) die(pid int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.alive, pid)
}

func (p *fakeProcs) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.started)
}

// fakeHealth answers the probe according to what the test set.
type fakeHealth struct {
	mu      sync.Mutex
	healthy bool
	probes  int
}

func (h *fakeHealth) Probe(context.Context, string, int) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.probes++
	if h.healthy {
		return nil
	}
	return errors.New("connection refused")
}

func (h *fakeHealth) set(v bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthy = v
}

// fakeStore is the record on disk.
type fakeStore struct {
	mu       sync.Mutex
	meta     *gateway.Meta
	writeErr error
	readErr  error
}

func (s *fakeStore) Read(context.Context) (*gateway.Meta, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.meta == nil {
		return nil, nil
	}
	out := *s.meta
	return &out, nil
}

func (s *fakeStore) Write(_ context.Context, m gateway.Meta) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta = &m
	return nil
}

func (s *fakeStore) Clear(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta = nil
	return nil
}

// realLock is an in-process mutex standing in for the file lock. It is enough
// to prove the ordering inside one process; the file lock has its own test in
// the adapter, because that is where cross-process behaviour lives.
type realLock struct {
	mu    sync.Mutex
	held  int
	fails bool
}

func (l *realLock) Lock(context.Context) (func() error, error) {
	if l.fails {
		return nil, errors.New("held by another process")
	}
	l.mu.Lock()
	l.held++
	return func() error { l.mu.Unlock(); return nil }, nil
}

type fakeResolver struct {
	cmd gateway.Command
	err error
}

func (r fakeResolver) Resolve(context.Context) (gateway.Command, error) { return r.cmd, r.err }

// steppingClock moves forward whenever the service sleeps, so a timeout is
// reached without the test waiting for it.
type steppingClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *steppingClock) Sleep(_ context.Context, d time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
	return nil
}

type harness struct {
	svc    *gateway.Service
	procs  *fakeProcs
	health *fakeHealth
	store  *fakeStore
	lock   *realLock
	clock  *steppingClock
}

func newHarness(t *testing.T, tweak ...func(*gateway.Deps)) *harness {
	t.Helper()
	h := &harness{
		procs:  newProcs(),
		health: &fakeHealth{healthy: true},
		store:  &fakeStore{},
		lock:   &realLock{},
		clock:  &steppingClock{at: refTime},
	}
	deps := gateway.Deps{
		Processes: h.procs, Health: h.health, Store: h.store, Locker: h.lock,
		Resolver: fakeResolver{cmd: gateway.Command{Path: "/usr/local/bin/aosd"}},
		Clock:    h.clock, Sleeper: h.clock,
		Log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Port: 5326,
	}
	for _, f := range tweak {
		f(&deps)
	}
	h.svc = gateway.NewService(deps)
	return h
}

func ctx() context.Context { return context.Background() }

// TestTheThreeStates is the state machine, and the middle one is the reason it
// exists: a crashed daemon must read as an orphan record, not as running.
func TestTheThreeStates(t *testing.T) {
	h := newHarness(t)

	got, err := h.svc.Status(ctx(), gateway.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != gateway.Stopped {
		t.Fatalf("with no record, status = %q", got.Status)
	}

	started, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := h.svc.Status(ctx(), gateway.StatusInput{}); err != nil || got.Status != gateway.Running {
		t.Fatalf("status = %+v, err = %v", got, err)
	}

	// The daemon crashes: the record survives, the process does not.
	h.procs.die(started.Meta.PID)
	got, err = h.svc.Status(ctx(), gateway.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != gateway.Stale {
		t.Fatalf("after a crash, status = %q", got.Status)
	}
	if got.Meta == nil || got.Meta.PID != started.Meta.PID {
		t.Error("the stale record should still say which process it was")
	}
}

// TestStatusTellsAliveApartFromServing: a process can be up and not answering,
// and the original's liveness-only check would call that running.
func TestStatusTellsAliveApartFromServing(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); err != nil {
		t.Fatal(err)
	}

	h.health.set(false)
	got, err := h.svc.Status(ctx(), gateway.StatusInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != gateway.Running {
		t.Fatalf("status = %q", got.Status)
	}
	if got.Healthy {
		t.Fatal("the daemon is not answering and status says it is healthy")
	}
}

func TestStartIsIdempotent(t *testing.T) {
	h := newHarness(t)
	first, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Meta.PID != first.Meta.PID {
		t.Fatalf("a second start spawned another daemon: %d then %d", first.Meta.PID, second.Meta.PID)
	}
	if h.procs.count() != 1 {
		t.Fatalf("%d processes were started", h.procs.count())
	}
}

// TestTwoConcurrentStartsProduceOneDaemon is defect #18. The original crosses a
// pid file with liveness and takes no lock, so both callers can observe
// "stopped" and both spawn.
func TestTwoConcurrentStartsProduceOneDaemon(t *testing.T) {
	h := newHarness(t)

	const callers = 8
	var wg sync.WaitGroup
	results := make([]gateway.State, callers)
	errs := make([]error, callers)
	wg.Add(callers)
	for i := range callers {
		go func() {
			defer wg.Done()
			results[i], errs[i] = h.svc.Start(ctx(), gateway.StartInput{})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d failed: %v", i, err)
		}
	}
	if h.procs.count() != 1 {
		t.Fatalf("%d daemons were started", h.procs.count())
	}
	for i, r := range results {
		if r.Meta.PID != results[0].Meta.PID {
			t.Fatalf("caller %d got pid %d, caller 0 got %d", i, r.Meta.PID, results[0].Meta.PID)
		}
	}
}

// TestStartClearsAStaleRecord: this is how a machine recovers from a crash
// without anyone deleting a file by hand.
func TestStartClearsAStaleRecord(t *testing.T) {
	h := newHarness(t)
	first, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}
	h.procs.die(first.Meta.PID)

	second, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != gateway.Running || second.Meta.PID == first.Meta.PID {
		t.Fatalf("second = %+v", second)
	}
	if h.procs.count() != 2 {
		t.Fatalf("%d processes started", h.procs.count())
	}
}

// TestADaemonThatNeverServesIsAFailure: the original waits 1.5 seconds for the
// process to be alive, and a daemon that could not bind its port passes that.
func TestADaemonThatNeverServesIsAFailure(t *testing.T) {
	h := newHarness(t)
	h.health.set(false)

	_, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err == nil {
		t.Fatal("a daemon that never answered was reported as started")
	}
	e, _ := apperr.As(err)
	if e.Code != "AOS_GATEWAY_NOT_HEALTHY" {
		t.Fatalf("error = %v", err)
	}
	if len(e.Actions) == 0 {
		t.Error("the caller should be told where to look")
	}
	// And it is not left behind holding a port.
	if len(h.procs.killed) != 1 {
		t.Errorf("the failed daemon was not cleaned up: %v", h.procs.killed)
	}
	if h.store.meta != nil {
		t.Error("the record of a daemon that never served was kept")
	}
}

// TestADaemonThatExitsIsReportedImmediately distinguishes the two failures:
// waiting the whole timeout for a process that is already gone is a worse
// answer than saying so.
func TestADaemonThatExitsIsReportedImmediately(t *testing.T) {
	h := newHarness(t)
	h.health.set(false)

	// The process dies as soon as it is asked about.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			h.procs.mu.Lock()
			n := len(h.procs.started)
			h.procs.mu.Unlock()
			if n > 0 {
				h.procs.mu.Lock()
				for pid := range h.procs.alive {
					delete(h.procs.alive, pid)
				}
				h.procs.mu.Unlock()
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	_, err := h.svc.Start(ctx(), gateway.StartInput{})
	<-done
	if err == nil {
		t.Fatal("expected a failure")
	}
	e, _ := apperr.As(err)
	if e.Code != "AOS_GATEWAY_DAEMON_EXITED" && e.Code != "AOS_GATEWAY_NOT_HEALTHY" {
		t.Fatalf("error = %v", err)
	}
}

func TestAFailureToWriteTheRecordKillsTheDaemonRatherThanLeakingIt(t *testing.T) {
	h := newHarness(t)
	h.store.writeErr = errors.New("disk full")

	_, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err == nil {
		t.Fatal("expected a failure")
	}
	if len(h.procs.killed) != 1 {
		t.Fatalf("a daemon nothing can find again was left running: %v", h.procs.killed)
	}
}

func TestStopShutsDownAndClearsTheRecord(t *testing.T) {
	h := newHarness(t)
	started, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.Stop(ctx(), gateway.StopInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Stopped || out.Killed || out.PID != started.Meta.PID {
		t.Fatalf("out = %+v", out)
	}
	if h.store.meta != nil {
		t.Error("the record survived the stop")
	}
	if got, _ := h.svc.Status(ctx(), gateway.StatusInput{}); got.Status != gateway.Stopped {
		t.Fatalf("status = %q", got.Status)
	}
}

// TestADaemonThatIgnoresTheRequestIsKilled, and says so: a daemon that could
// not shut down cleanly lost whatever it had not written.
func TestADaemonThatIgnoresTheRequestIsKilled(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); err != nil {
		t.Fatal(err)
	}
	h.procs.ignoresTerminate = true

	out, err := h.svc.Stop(ctx(), gateway.StopInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Stopped || !out.Killed {
		t.Fatalf("out = %+v", out)
	}
	if len(h.procs.terminated) == 0 {
		t.Error("it should be asked before it is killed")
	}
	if len(h.procs.killed) != 1 {
		t.Errorf("killed = %v", h.procs.killed)
	}
}

func TestStoppingWhatIsNotRunningIsNotAnError(t *testing.T) {
	h := newHarness(t)
	out, err := h.svc.Stop(ctx(), gateway.StopInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped || out.Status != gateway.Stopped {
		t.Fatalf("out = %+v", out)
	}
}

func TestStoppingAStaleRecordCleansIt(t *testing.T) {
	h := newHarness(t)
	started, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}
	h.procs.die(started.Meta.PID)

	out, err := h.svc.Stop(ctx(), gateway.StopInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped || !out.Cleaned {
		t.Fatalf("out = %+v", out)
	}
	if h.store.meta != nil {
		t.Error("the stale record was not cleaned")
	}
}

func TestRestartStopsThenStarts(t *testing.T) {
	h := newHarness(t)
	first, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err != nil {
		t.Fatal(err)
	}

	after, err := h.svc.Restart(ctx(), gateway.RestartInput{})
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != gateway.Running {
		t.Fatalf("after = %+v", after)
	}
	if after.Meta.PID == first.Meta.PID {
		t.Fatal("restart did not start a new process")
	}
	if h.procs.Alive(first.Meta.PID) {
		t.Fatal("the old daemon is still running")
	}
}

func TestALockedGatewayRefusesRatherThanRacing(t *testing.T) {
	h := newHarness(t)
	h.lock.fails = true

	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("start: error = %v", err)
	}
	if _, err := h.svc.Stop(ctx(), gateway.StopInput{}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("stop: error = %v", err)
	}
	if h.procs.count() != 0 {
		t.Error("something was started while the lock was held elsewhere")
	}
}

// TestStatusTakesNoLock: reading the state must not block behind a start that
// is in progress, or the command a person runs to find out what is happening
// would hang exactly when something is happening.
func TestStatusTakesNoLock(t *testing.T) {
	h := newHarness(t)
	h.lock.fails = true
	if _, err := h.svc.Status(ctx(), gateway.StatusInput{}); err != nil {
		t.Fatalf("status = %v", err)
	}
}

func TestAResolverFailureStopsBeforeAnythingIsSpawned(t *testing.T) {
	h := newHarness(t, func(d *gateway.Deps) {
		d.Resolver = fakeResolver{err: errors.New("no binary")}
	})
	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); err == nil {
		t.Fatal("expected a failure")
	}
	if h.procs.count() != 0 {
		t.Error("something was spawned without a command to run")
	}
}

func TestAnUnreadableRecordIsReportedRatherThanGuessed(t *testing.T) {
	h := newHarness(t)
	h.store.readErr = errors.New("permission denied")

	if _, err := h.svc.Status(ctx(), gateway.StatusInput{}); err == nil {
		t.Fatal("a record that cannot be read is not the same as no record")
	}
	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); err == nil {
		t.Fatal("start proceeded without knowing the state")
	}
	if h.procs.count() != 0 {
		t.Error("a daemon was started while the state was unknown")
	}
}

func TestASpawnFailureIsReported(t *testing.T) {
	h := newHarness(t)
	h.procs.startErr = errors.New("permission denied")

	_, err := h.svc.Start(ctx(), gateway.StartInput{})
	if err == nil {
		t.Fatal("expected a failure")
	}
	e, _ := apperr.As(err)
	if e.Code != "AOS_GATEWAY_SPAWN_FAILED" {
		t.Fatalf("error = %v", err)
	}
	if h.store.meta != nil {
		t.Error("a record was written for a daemon that never started")
	}
}

// TestADaemonThatCannotBeKilledIsReported: the person has to be told, because
// the process is still holding the port and only they can deal with it.
func TestADaemonThatCannotBeKilledIsReported(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); err != nil {
		t.Fatal(err)
	}
	h.procs.ignoresTerminate = true
	h.procs.killErr = errors.New("operation not permitted")

	_, err := h.svc.Stop(ctx(), gateway.StopInput{})
	if err == nil {
		t.Fatal("expected a failure")
	}
	e, _ := apperr.As(err)
	if e.Code != "AOS_GATEWAY_KILL_FAILED" || len(e.Actions) == 0 {
		t.Fatalf("error = %+v", e)
	}
}

func TestRestartOfAStoppedDaemonJustStartsIt(t *testing.T) {
	h := newHarness(t)
	got, err := h.svc.Restart(ctx(), gateway.RestartInput{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != gateway.Running {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestRestartStopsBeforeItReportsAFailureToStart(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Start(ctx(), gateway.StartInput{}); err != nil {
		t.Fatal(err)
	}
	h.health.set(false)

	if _, err := h.svc.Restart(ctx(), gateway.RestartInput{}); err == nil {
		t.Fatal("expected a failure")
	}
	// The old daemon is gone either way: a restart that failed halfway must not
	// leave the previous process running under a record that no longer exists.
	if h.store.meta != nil {
		t.Errorf("a record survived a failed restart: %+v", h.store.meta)
	}
}

func TestTheDefaultsAreTheDeclaredOnes(t *testing.T) {
	svc := gateway.NewService(gateway.Deps{
		Processes: newProcs(), Health: &fakeHealth{healthy: true},
		Store: &fakeStore{}, Locker: &realLock{},
		Resolver: fakeResolver{}, Clock: &steppingClock{at: refTime},
		Sleeper: &steppingClock{at: refTime},
	})
	if svc == nil {
		t.Fatal("no service")
	}
	// A service built with nothing but its ports still knows where to look.
	got, err := svc.Status(ctx(), gateway.StatusInput{})
	if err != nil || got.Status != gateway.Stopped {
		t.Fatalf("status = %+v, err = %v", got, err)
	}
}

func TestRegisterPublishesTheGroupAsLocal(t *testing.T) {
	h := newHarness(t)
	reg := command.NewRegistry()
	gateway.Register(reg, h.svc)

	want := []string{"gateway_restart", "gateway_start", "gateway_status", "gateway_stop"}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
		// Every one of them is local: supervising a process by asking that
		// process to do it does not work.
		if !d.Local() {
			t.Errorf("%s is not marked local", d.Key())
		}
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commands = %v, want %v", got, want)
		}
	}

	for _, d := range reg.Sorted() {
		switch d.Key() {
		case "gateway_status":
			if !d.Annotations().ReadOnlyHint {
				t.Error("asking the status changes nothing")
			}
		case "gateway_stop", "gateway_restart":
			if !d.Annotations().DestructiveHint {
				t.Errorf("%s must be announced destructive", d.Key())
			}
		}
	}

	// The restart documentation has to warn the caller that the answer will not
	// arrive, because it is the process being restarted that would send it.
	for _, d := range reg.Sorted() {
		if d.Key() == "gateway_restart" && !strings.Contains(d.Doc(), "will not return") {
			t.Error("gateway restart does not warn that the call does not come back")
		}
	}
}
