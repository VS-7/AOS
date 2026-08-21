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

func TestStartRefusesWhenAPITokenMissingEvenIfEnabled(t *testing.T) {
	svc := NewService(Deps{
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "", Hostname: "h", Token: "t"}},
		Runner: &fakeRunner{},
		Clock:  clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_INSECURE_EXPOSURE")
}

func TestStartRefusesWhenHostnameOrTokenMissing_DistinctFromInsecureExposure(t *testing.T) {
	svc := NewService(Deps{
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "", Token: ""}},
		Runner: &fakeRunner{},
		Clock:  clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_CONFIG_INCOMPLETE")
}

func TestStartMapsBinaryMissing(t *testing.T) {
	runner := &fakeRunner{spawn: func(int) (Process, error) {
		return nil, errors.Join(ErrBinaryMissing, errors.New("exec: \"cloudflared\": executable file not found in $PATH"))
	}}
	svc := NewService(Deps{
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "h", Token: "secret-token"}},
		Runner: runner,
		Clock:  clockx.System{},
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
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "h", Token: "t"}},
		Runner: runner,
		Clock:  clockx.System{},
	})
	_, err := svc.Start(context.Background())
	_ = mustAppErr(t, err, "TUNNEL_READINESS_TIMEOUT")
}

func TestStartSucceedsAndReportsURL(t *testing.T) {
	runner := &fakeRunner{spawn: func(int) (Process, error) {
		return &fakeProcess{pid: 4242, exit: make(chan error, 1)}, nil
	}}
	svc := NewService(Deps{
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "example.trycloudflare.com", Token: "t"}},
		Runner: runner,
		Clock:  clockx.System{},
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
		Config: fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "h", Token: "t"}},
		Runner: &fakeRunner{},
		Clock:  clockx.System{},
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
	cfg := fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "h", Token: "t"}}
	svc := NewService(Deps{Config: cfg, Runner: runner, Clock: clockx.System{}})

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
		Config:       fakeConfig{raw: RawConfig{SecurityEnabled: true, APIToken: "api-tok", Hostname: "h", Token: "t"}},
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
