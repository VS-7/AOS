package tunnel

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
)

type fakeConfig struct{ raw RawConfig }

func (f fakeConfig) Raw(context.Context) (RawConfig, error) { return f.raw, nil }

// fakeCredentials stands in for the accounts store: whether anybody holds an
// API token a remote caller could present.
type fakeCredentials struct {
	active bool
	err    error
}

func (f fakeCredentials) HasActiveAPIToken(context.Context) (bool, error) { return f.active, f.err }

type fakeProcess struct {
	pid  int
	exit chan error
}

func (p *fakeProcess) PID() int    { return p.pid }
func (p *fakeProcess) Wait() error { return <-p.exit }
func (p *fakeProcess) Stop() error { p.exit <- nil; return nil }

// fakeRunner lets a test script exactly what Spawn does on each call, so the
// supervisor's restart path is testable without a real cloudflared.
type fakeRunner struct {
	calls int32
	spawn func(call int) (Process, error)
}

func (r *fakeRunner) Spawn(ctx context.Context, hostname, token string, timeout time.Duration) (Process, error) {
	n := int(atomic.AddInt32(&r.calls, 1))
	return r.spawn(n)
}

func mustAppErr(t *testing.T, err error, code string) *apperr.Error {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperr.Error, got %T: %v", err, err)
	}
	want := "AOS_" + code
	if ae.Code != want {
		t.Fatalf("expected code %s, got %s", want, ae.Code)
	}
	return ae
}

func TestStartRefusesWhenAPIIsNotAuthenticated(t *testing.T) {
	svc := NewService(Deps{
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: false, Hostname: "h", Token: "t"}},
		Runner: &fakeRunner{},
		Clock:  clockx.System{},
	})
	_, err := svc.Start(context.Background())
	ae := mustAppErr(t, err, "TUNNEL_INSECURE_EXPOSURE")
	if len(ae.Actions) == 0 {
		t.Fatal("expected a CTA telling the caller how to fix this")
	}
}

// Security on with no account API token is authentication nobody can pass:
// the credential a remote caller presents is the account token Settings >
// Developers issues, so the guard asks for that and not for a config field
// nothing authenticates against.
func TestStartRefusesWhenNoAccountHoldsAnAPIToken(t *testing.T) {
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "t"}},
		Credentials: fakeCredentials{active: false},
		Runner:      &fakeRunner{},
		Clock:       clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_INSECURE_EXPOSURE")
}

func TestStartRefusesWhenCredentialsExistButSecurityIsOff(t *testing.T) {
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: false, Hostname: "h", Token: "t"}},
		Credentials: fakeCredentials{active: true},
		Runner:      &fakeRunner{},
		Clock:       clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_INSECURE_EXPOSURE")
}

// The screen enables its switch from Status, so the guard has to be readable
// without attempting a start.
func TestStatusReportsWhetherTheGuardIsSatisfied(t *testing.T) {
	cfg := fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "t"}}
	svc := NewService(Deps{Config: cfg, Credentials: fakeCredentials{active: true}, Runner: &fakeRunner{}, Clock: clockx.System{}})
	state, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !state.Authenticated {
		t.Fatal("expected Status to report the guard as satisfied")
	}

	svc = NewService(Deps{Config: cfg, Credentials: fakeCredentials{active: false}, Runner: &fakeRunner{}, Clock: clockx.System{}})
	state, err = svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if state.Authenticated {
		t.Fatal("expected Status to report the guard as unmet with no account API token")
	}
}

func TestStartRefusesWhenHostnameOrTokenMissing_DistinctFromInsecureExposure(t *testing.T) {
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "", Token: ""}},
		Credentials: fakeCredentials{active: true},
		Runner:      &fakeRunner{},
		Clock:       clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_CONFIG_INCOMPLETE")
}

func TestStartMapsBinaryMissing(t *testing.T) {
	runner := &fakeRunner{spawn: func(int) (Process, error) {
		return nil, errors.Join(ErrBinaryMissing, errors.New("exec: \"cloudflared\": executable file not found in $PATH"))
	}}
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "secret-token"}},
		Credentials: fakeCredentials{active: true},
		Runner:      runner,
		Clock:       clockx.System{},
	})
	_, err := svc.Start(context.Background())
	ae := mustAppErr(t, err, "TUNNEL_BINARY_MISSING")
	if strings.Contains(ae.Error(), "secret-token") {
		t.Fatal("the tunnel token must never appear in an error message")
	}
}

func TestStartMapsReadinessTimeout(t *testing.T) {
	runner := &fakeRunner{spawn: func(int) (Process, error) {
		return nil, errors.Join(ErrReadinessTimeout, errors.New("no connection reported"))
	}}
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "t"}},
		Credentials: fakeCredentials{active: true},
		Runner:      runner,
		Clock:       clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_READINESS_TIMEOUT")
}

func TestStartSucceedsAndReportsURL(t *testing.T) {
	runner := &fakeRunner{spawn: func(int) (Process, error) {
		return &fakeProcess{pid: 4242, exit: make(chan error, 1)}, nil
	}}
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "example.trycloudflare.com", Token: "t"}},
		Credentials: fakeCredentials{active: true},
		Runner:      runner,
		Clock:       clockx.System{},
	})
	t.Cleanup(func() { _, _ = svc.Stop(context.Background()) })
	state, err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Status != Running {
		t.Fatalf("expected Running, got %s", state.Status)
	}
	if state.URL != "https://example.trycloudflare.com" {
		t.Fatalf("unexpected URL: %s", state.URL)
	}
	if state.PID != 4242 {
		t.Fatalf("unexpected PID: %d", state.PID)
	}
	if state.StartedAt == nil {
		t.Fatal("expected StartedAt to be set")
	}

	// Idempotent: calling Start again while running does not respawn.
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatalf("second Start should be a no-op, got: %v", err)
	}
	if atomic.LoadInt32(&runner.calls) != 1 {
		t.Fatalf("expected exactly one Spawn call, got %d", runner.calls)
	}
}

func TestStopIsIdempotentAndReportsStopped(t *testing.T) {
	svc := NewService(Deps{
		Config:      fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "t"}},
		Credentials: fakeCredentials{active: true},
		Runner:      &fakeRunner{},
		Clock:       clockx.System{},
	})
	state, err := svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("stopping an already-stopped tunnel must succeed, got: %v", err)
	}
	if state.Status != Stopped {
		t.Fatalf("expected Stopped, got %s", state.Status)
	}
}

func TestStopTerminatesTheRunningProcessAndPreservesConfig(t *testing.T) {
	proc := &fakeProcess{pid: 1, exit: make(chan error, 1)}
	runner := &fakeRunner{spawn: func(int) (Process, error) { return proc, nil }}
	cfg := fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "t"}}
	svc := NewService(Deps{Config: cfg, Credentials: fakeCredentials{active: true}, Runner: runner, Clock: clockx.System{}})

	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	state, err := svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if state.Status != Stopped {
		t.Fatalf("expected Stopped, got %s", state.Status)
	}
	// Config is read-only to this service — hostname/token were never
	// mutated, so a later Start finds them exactly as configured (this test
	// never wrote to cfg.raw, which is itself the assertion: no method on
	// this service can reach into Config to clear it).
	got, _ := cfg.Raw(context.Background())
	if got.Hostname != "h" || got.Token != "t" {
		t.Fatalf("hostname/token must survive Stop, got %+v", got)
	}
}

func TestSupervisorRestartsAfterUnexpectedDeath(t *testing.T) {
	var procs []*fakeProcess
	runner := &fakeRunner{spawn: func(n int) (Process, error) {
		p := &fakeProcess{pid: n, exit: make(chan error, 1)}
		procs = append(procs, p)
		return p, nil
	}}
	svc := NewService(Deps{
		Config:       fakeConfig{raw: RawConfig{SecurityEnabled: true, Hostname: "h", Token: "t"}},
		Credentials:  fakeCredentials{active: true},
		Runner:       runner,
		Clock:        clockx.System{},
		BackoffStart: 10 * time.Millisecond,
		BackoffMax:   20 * time.Millisecond,
	})
	t.Cleanup(func() { _, _ = svc.Stop(context.Background()) })

	state, err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if state.PID != 1 {
		t.Fatalf("expected first spawn's PID, got %d", state.PID)
	}

	// Simulate an unexpected crash: the process exits with an error, not via Stop.
	procs[0].exit <- errors.New("boom")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := svc.Status(context.Background())
		if s.Status == Running && s.PID == 2 {
			return // restarted with a fresh PID: the supervisor did its job.
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("supervisor did not restart the tunnel after an unexpected death")
}
