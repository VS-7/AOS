package tunnel

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
)

// readinessTimeout bounds how long Start waits for the runner to report a
// connected tunnel before giving up.
const readinessTimeout = 15 * time.Second

// defaultBackoffStart and defaultBackoffMax bound the supervisor's restart
// delay after an unexpected death — starting fast, capped so a persistently
// broken tunnel does not spin the CPU.
const (
	defaultBackoffStart = time.Second
	defaultBackoffMax   = 30 * time.Second
)

// Deps are what Service needs, narrowed to exactly what it reads.
type Deps struct {
	Config      Config
	Credentials Credentials
	Runner      Runner
	Clock       clockx.Clock
	Log         *slog.Logger

	// BackoffStart and BackoffMax override the supervisor's restart delay.
	// Zero means the defaults above — overridden by service_test.go so the
	// restart-after-crash test does not take 30+ seconds, and instance
	// fields (not package vars) so tests running in parallel never race on
	// them the way a shared package var would.
	BackoffStart time.Duration
	BackoffMax   time.Duration
}

// service is the default Service.
type service struct {
	cfg    Config
	creds  Credentials
	runner Runner
	clock  clockx.Clock
	log    *slog.Logger

	backoffStart time.Duration
	backoffMax   time.Duration

	mu       sync.Mutex
	state    State
	proc     Process
	stopping bool          // Stop was called: the supervisor's exit is expected, not a crash to restart from.
	done     chan struct{} // closed when the supervisor goroutine returns, so Stop can wait for it.
}

// NewService builds the tunnel service.
func NewService(deps Deps) Service {
	clock := deps.Clock
	if clock == nil {
		clock = clockx.System{}
	}
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	backoffStart := deps.BackoffStart
	if backoffStart <= 0 {
		backoffStart = defaultBackoffStart
	}
	backoffMax := deps.BackoffMax
	if backoffMax <= 0 {
		backoffMax = defaultBackoffMax
	}
	return &service{
		cfg:          deps.Config,
		creds:        deps.Credentials,
		runner:       deps.Runner,
		clock:        clock,
		log:          log,
		backoffStart: backoffStart,
		backoffMax:   backoffMax,
		state:        State{Status: Stopped},
	}
}

func (s *service) Status(ctx context.Context) (State, error) {
	s.mu.Lock()
	state := s.state
	s.mu.Unlock()
	return s.reporting(ctx, state), nil
}

// reporting fills State.Authenticated in before the state leaves the service.
//
// The guard is read rather than remembered: the switch that depends on it
// lives on a settings screen where authentication is turned on and API tokens
// are issued, so the verdict changes under the reader's hands. A failure to
// read it is not a failure to report the tunnel's state — the state is still
// answered, with the guard reported as unmet.
func (s *service) reporting(ctx context.Context, state State) State {
	authenticated, err := s.authenticated(ctx)
	if err != nil {
		s.log.Warn("could not read the tunnel's authentication guard", "error", err)
	}
	state.Authenticated = authenticated
	return state
}

// authenticated reports whether the API this tunnel would publish actually
// asks callers for a credential: security on, and at least one account
// holding an active API token. Both halves are read fresh — see Start.
func (s *service) authenticated(ctx context.Context) (bool, error) {
	cfg, err := s.cfg.Raw(ctx)
	if err != nil {
		return false, err
	}
	return s.authenticatedWith(ctx, cfg)
}

// authenticatedWith is authenticated over a configuration the caller has
// already read, so Start does not read it twice.
func (s *service) authenticatedWith(ctx context.Context, cfg RawConfig) (bool, error) {
	if !cfg.SecurityEnabled {
		return false, nil
	}
	// No Credentials port wired is a misconfiguration, and one that would
	// open the door: fail closed rather than assume the tokens exist.
	if s.creds == nil {
		return false, nil
	}
	return s.creds.HasActiveAPIToken(ctx)
}

// Start publishes the local daemon. Before spawning anything it verifies the
// two things that make the difference between "remote access" and "an open
// door" — see docs/04 - Domínio/Tunnel (Go).md.
func (s *service) Start(ctx context.Context) (State, error) {
	s.mu.Lock()
	if s.state.Status == Running || s.state.Status == Starting {
		state := s.state
		s.mu.Unlock()
		return s.reporting(ctx, state), nil
	}
	s.mu.Unlock()

	cfg, err := s.cfg.Raw(ctx)
	if err != nil {
		return State{}, err
	}
	authenticated, err := s.authenticatedWith(ctx, cfg)
	if err != nil {
		return State{}, err
	}
	if !authenticated {
		return State{}, errInsecureExposure()
	}
	if cfg.Hostname == "" || cfg.Token == "" {
		return State{}, errConfigIncomplete()
	}

	s.mu.Lock()
	s.state = State{Status: Starting}
	s.stopping = false
	s.mu.Unlock()

	proc, err := s.runner.Spawn(ctx, cfg.Hostname, cfg.Token, readinessTimeout)
	if err != nil {
		wrapped := translateSpawnErr(err)
		s.mu.Lock()
		s.state = State{Status: Failed, Error: wrapped.Error()}
		s.mu.Unlock()
		return State{}, wrapped
	}

	started := s.clock.Now()
	url := "https://" + cfg.Hostname
	s.mu.Lock()
	s.proc = proc
	s.state = State{Status: Running, URL: url, PID: proc.PID(), StartedAt: &started}
	s.done = make(chan struct{})
	// The guard above let this Start through, so the state this call reports
	// says so; the stored state leaves it to Status, which re-reads it.
	state := s.state
	state.Authenticated = true
	s.mu.Unlock()

	go s.supervise(cfg.Hostname, cfg.Token)

	return state, nil
}

// translateSpawnErr maps a Runner's sentinel-wrapped error to the domain's
// own apperr catalog entry, so a caller sees "install cloudflared" rather
// than "not available in this build" for the case that is actually missing a
// binary, and "check the hostname/token" for the case that connected to
// nothing.
func translateSpawnErr(err error) error {
	switch {
	case errors.Is(err, ErrBinaryMissing):
		return errBinaryMissing(err)
	case errors.Is(err, ErrReadinessTimeout):
		return errReadinessTimeout(err)
	default:
		return errSpawnFailed(err)
	}
}

// supervise waits for the process this Start spawned to exit. An exit while
// s.stopping is true is Stop's own doing and ends the loop quietly; any other
// exit is an unexpected death, restarted with exponential backoff — without
// this, a webhook channel or a remote session dies in silence and nobody is
// told, which is exactly the gap the design doc calls out over the original.
func (s *service) supervise(hostname, token string) {
	backoff := s.backoffStart
	for {
		s.mu.Lock()
		proc := s.proc
		s.mu.Unlock()

		err := proc.Wait()

		s.mu.Lock()
		stopping := s.stopping
		s.mu.Unlock()
		if stopping {
			s.mu.Lock()
			close(s.done)
			s.mu.Unlock()
			return
		}

		s.mu.Lock()
		s.state = State{Status: Failed, Error: errString(err)}
		s.mu.Unlock()
		s.log.Warn("tunnel died unexpectedly, restarting", "err", err, "backoff", backoff)

		var next Process
		var spawnErr error
		for {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > s.backoffMax {
				backoff = s.backoffMax
			}

			s.mu.Lock()
			stopping = s.stopping
			s.mu.Unlock()
			if stopping {
				s.mu.Lock()
				close(s.done)
				s.mu.Unlock()
				return
			}

			next, spawnErr = s.runner.Spawn(context.Background(), hostname, token, readinessTimeout)
			if spawnErr == nil {
				break
			}
			s.mu.Lock()
			s.state = State{Status: Failed, Error: spawnErr.Error()}
			s.mu.Unlock()
			s.log.Warn("tunnel restart failed, will retry", "err", spawnErr, "backoff", backoff)
		}

		started := s.clock.Now()
		s.mu.Lock()
		s.proc = next
		s.state = State{Status: Running, URL: "https://" + hostname, PID: next.PID(), StartedAt: &started}
		s.mu.Unlock()
		backoff = s.backoffStart
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Stop tears the tunnel down. Hostname and token stay in the config — see
// the design doc's decision — so a later Start needs no reconfiguring.
func (s *service) Stop(ctx context.Context) (State, error) {
	s.mu.Lock()
	if s.state.Status != Running && s.state.Status != Starting {
		state := s.state
		s.mu.Unlock()
		return s.reporting(ctx, state), nil
	}
	s.stopping = true
	proc := s.proc
	done := s.done
	s.mu.Unlock()

	if proc != nil {
		_ = proc.Stop()
	}
	if done != nil {
		<-done
	}

	s.mu.Lock()
	s.state = State{Status: Stopped}
	s.proc = nil
	state := s.state
	s.mu.Unlock()
	return s.reporting(ctx, state), nil
}
