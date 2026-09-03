package gateway

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/OWNER/aos/internal/core/build"
)

// The timings of supervision. They are generous because the alternative to
// waiting is reporting a daemon as failed while it is still opening its index.
const (
	// DefaultStartTimeout bounds the wait for the daemon to answer its health
	// check.
	DefaultStartTimeout = 30 * time.Second

	// DefaultStopTimeout is how long a daemon gets to shut down cleanly before
	// it is killed.
	DefaultStopTimeout = 15 * time.Second

	// probeInterval is how often the health endpoint is polled while starting.
	probeInterval = 100 * time.Millisecond
)

// Service supervises the daemon.
type Service struct {
	procs    Processes
	health   Health
	store    Store
	locker   Locker
	resolver Resolver
	clock    Clock
	sleeper  Sleeper
	log      *slog.Logger

	host         string
	port         int
	inside       bool
	startTimeout time.Duration
	stopTimeout  time.Duration
}

// Deps is what the service is built from.
type Deps struct {
	Processes Processes
	Health    Health
	Store     Store
	Locker    Locker
	Resolver  Resolver
	Clock     Clock
	Sleeper   Sleeper
	Log       *slog.Logger

	Host string
	Port int

	// Inside marks the service as living *in* the daemon rather than
	// supervising it from outside.
	//
	// The same service is built twice: once by whatever launches the daemon
	// (the desktop, `aos gateway`), and once by the daemon itself, which
	// registers the group so `aos gateway status` can be answered over HTTP.
	// Two of its methods are wrong when it is the second, and silently so —
	// Status describes a bookkeeping record that only the *supervisor* writes,
	// so a daemon started any other way (a server's systemd unit, `task dev`,
	// a developer running aosd by hand) reported itself stopped while
	// answering the call; and Restart signalled the pid in that record, which
	// is this process, so the daemon terminated itself mid-request, answered
	// an unclassified 500, and never came back.
	Inside bool

	StartTimeout time.Duration
	StopTimeout  time.Duration
}

// NewService wires the supervisor over its ports.
func NewService(d Deps) *Service {
	s := &Service{
		procs: d.Processes, health: d.Health, store: d.Store, locker: d.Locker,
		resolver: d.Resolver, clock: d.Clock, sleeper: d.Sleeper, log: d.Log,
		host: d.Host, port: d.Port, inside: d.Inside,
		startTimeout: d.StartTimeout, stopTimeout: d.StopTimeout,
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.host == "" {
		s.host = "127.0.0.1"
	}
	if s.port == 0 {
		s.port = build.Port
	}
	if s.startTimeout == 0 {
		s.startTimeout = DefaultStartTimeout
	}
	if s.stopTimeout == 0 {
		s.stopTimeout = DefaultStopTimeout
	}
	return s
}

// Status reports what is going on, without changing anything.
func (s *Service) Status(ctx context.Context, _ StatusInput) (State, error) {
	state, err := s.read(ctx)
	if err != nil {
		return State{}, err
	}
	// A daemon answering this call is running, whatever the record says.
	//
	// The record is the *supervisor's* bookkeeping, written by whatever
	// spawned the daemon. A daemon started any other way — a server's systemd
	// unit, `task dev`, a developer running `aosd serve` by hand — leaves it
	// absent, and this method then reported "stopped" to the very client it
	// was talking to. The window drew a red badge and offered to restart a
	// process that was healthy.
	if s.inside {
		meta := state.Meta
		if meta == nil {
			meta = &Meta{
				PID: os.Getpid(), Host: s.host, Port: s.port,
				Version: build.Version, StartedAt: s.clock.Now(),
			}
		}
		return State{Status: Running, Meta: meta, Healthy: true}, nil
	}
	if state.Status == Running {
		state.Healthy = s.health.Probe(ctx, s.host, state.Meta.Port) == nil
	}
	return state, nil
}

// read crosses the persisted record with the liveness of the process it names.
func (s *Service) read(ctx context.Context) (State, error) {
	meta, err := s.store.Read(ctx)
	if err != nil {
		return State{}, errStateUnreadable(err)
	}
	if meta == nil {
		return State{Status: Stopped}, nil
	}
	if !s.procs.Alive(meta.PID) {
		return State{Status: Stale, Meta: meta}, nil
	}
	return State{Status: Running, Meta: meta}, nil
}

// Start launches the daemon, and is idempotent.
//
// The lock is taken before the state is read, which is the whole point: that
// ordering is what closes the race the original leaves open (defect #18). Read
// first and two simultaneous callers both see "stopped" and both spawn.
func (s *Service) Start(ctx context.Context, _ StartInput) (State, error) {
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		return State{}, errLockUnavailable(err)
	}
	defer func() {
		if err := unlock(); err != nil {
			s.log.Warn("could not release the gateway lock", "err", err)
		}
	}()

	state, err := s.read(ctx)
	if err != nil {
		return State{}, err
	}
	switch state.Status {
	case Running:
		state.Healthy = s.health.Probe(ctx, s.host, state.Meta.Port) == nil
		return state, nil
	case Stale:
		// A crashed daemon left its record behind. Clearing it here is what
		// turns "the system thinks it is running" into a recoverable state.
		s.log.Info("clearing the record of a daemon that is no longer running", "pid", state.Meta.PID)
		if err := s.store.Clear(ctx); err != nil {
			return State{}, errStateUnwritable(err)
		}
	case Stopped:
	}

	cmd, err := s.resolver.Resolve(ctx)
	if err != nil {
		return State{}, err
	}
	pid, err := s.procs.Start(ctx, cmd)
	if err != nil {
		return State{}, errSpawnFailed(cmd.Path, err)
	}

	meta := Meta{
		PID: pid, Port: s.port, Host: s.host,
		StartedAt: s.clock.Now(), Version: build.Version,
		Command: cmd.Path, Args: cmd.Args,
	}
	if err := s.store.Write(ctx, meta); err != nil {
		// The record could not be written, so nothing would ever find this
		// process again. Killing it is better than leaking it.
		_ = s.procs.Kill(pid)
		return State{}, errStateUnwritable(err)
	}

	if err := s.waitHealthy(ctx, pid); err != nil {
		_ = s.procs.Kill(pid)
		_ = s.store.Clear(ctx)
		return State{}, err
	}
	return State{Status: Running, Meta: &meta, Healthy: true}, nil
}

// waitHealthy polls the health endpoint until the daemon answers, the process
// dies, or the budget runs out.
func (s *Service) waitHealthy(ctx context.Context, pid int) error {
	deadline := s.clock.Now().Add(s.startTimeout)
	for {
		if err := s.health.Probe(ctx, s.host, s.port); err == nil {
			return nil
		}
		// A process that exited is not going to start answering. Reporting that
		// immediately, rather than after the full timeout, is the difference
		// between a useful error and a wait.
		if !s.procs.Alive(pid) {
			return errDaemonExited(pid)
		}
		if !s.clock.Now().Before(deadline) {
			return errNeverBecameHealthy(s.host, s.port, s.startTimeout)
		}
		if err := s.sleeper.Sleep(ctx, probeInterval); err != nil {
			return err
		}
	}
}

// Stop asks the daemon to shut down, and insists if it does not.
func (s *Service) Stop(ctx context.Context, _ StopInput) (StopOutput, error) {
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		return StopOutput{}, errLockUnavailable(err)
	}
	defer func() {
		if err := unlock(); err != nil {
			s.log.Warn("could not release the gateway lock", "err", err)
		}
	}()

	state, err := s.read(ctx)
	if err != nil {
		return StopOutput{}, err
	}
	switch state.Status {
	case Stopped:
		return StopOutput{Stopped: false, Status: Stopped}, nil
	case Stale:
		if err := s.store.Clear(ctx); err != nil {
			return StopOutput{}, errStateUnwritable(err)
		}
		return StopOutput{Stopped: false, Status: Stopped, Cleaned: true}, nil
	case Running:
	}

	pid := state.Meta.PID
	if err := s.procs.Terminate(pid); err != nil {
		s.log.Warn("could not signal the daemon", "pid", pid, "err", err)
	}

	deadline := s.clock.Now().Add(s.stopTimeout)
	for s.procs.Alive(pid) {
		if !s.clock.Now().Before(deadline) {
			// It was asked and did not go. Escalating is the only remaining
			// option, and doing it silently would hide a daemon that cannot
			// shut down cleanly.
			s.log.Warn("the daemon ignored the shutdown request; killing it",
				"pid", pid, "waited", s.stopTimeout.String())
			if err := s.procs.Kill(pid); err != nil {
				return StopOutput{}, errKillFailed(pid, err)
			}
			if err := s.store.Clear(ctx); err != nil {
				return StopOutput{}, errStateUnwritable(err)
			}
			return StopOutput{Stopped: true, Status: Stopped, Killed: true, PID: pid}, nil
		}
		if err := s.sleeper.Sleep(ctx, probeInterval); err != nil {
			return StopOutput{}, err
		}
	}

	if err := s.store.Clear(ctx); err != nil {
		return StopOutput{}, errStateUnwritable(err)
	}
	return StopOutput{Stopped: true, Status: Stopped, PID: pid}, nil
}

// Restart stops and starts.
//
// The order is not negotiable and the reason is worth stating: an agent can
// call this, and it is calling it through the process being restarted. The
// answer to that call has to be produced before anything is killed, which is a
// property of the transport rather than of this method — see the command's
// documentation.
func (s *Service) Restart(ctx context.Context, _ RestartInput) (State, error) {
	// A daemon cannot restart itself through this path, and trying was worse
	// than refusing: Stop signals the pid in the record — this process — so
	// the daemon terminated itself while executing the request, the caller
	// got an unclassified 500 as the connection died, and nothing brought it
	// back. The window then had no daemon at all until the application was
	// relaunched.
	//
	// Restarting belongs to whatever launched it, which is exactly who has a
	// working copy of this service. The refusal says so.
	if s.inside {
		return State{}, errSelfRestart()
	}
	if _, err := s.Stop(ctx, StopInput{}); err != nil {
		return State{}, err
	}
	return s.Start(ctx, StartInput{})
}
