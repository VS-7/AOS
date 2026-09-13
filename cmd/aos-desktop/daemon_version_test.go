package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/transport/daemonclient"
)

type fakeSupervision struct {
	state    gateway.State
	restarts int
}

func (f *fakeSupervision) Status(context.Context, gateway.StatusInput) (gateway.State, error) {
	return f.state, nil
}

func (f *fakeSupervision) Restart(context.Context, gateway.RestartInput) (gateway.State, error) {
	f.restarts++
	return f.state, nil
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// Reinstalling — the only update path that works — replaces the bundle while
// the daemon the old window started keeps running, detached. The new window
// found it healthy and adopted it: the old aosd, with every daemon-side defect
// the update fixed, until somebody killed it by hand or rebooted.
func TestAWindowReplacesTheOlderDaemonItsSupervisorStarted(t *testing.T) {
	sup := &fakeSupervision{state: gateway.State{Status: gateway.Running, Healthy: true, Meta: &gateway.Meta{PID: 42}}}
	guard := newVersionGuard("v0.15.2")

	if !guard.check(context.Background(), sup, "v0.15.0", quiet) {
		t.Error("an older daemon this window's supervisor owns was kept")
	}
	if sup.restarts != 1 {
		t.Fatalf("restarts = %d, want 1", sup.restarts)
	}

	// Once per version: a daemon that still reports the old version after a
	// restart (a different binary on AOS_DAEMON_PATH, say) is not restarted
	// in a loop.
	guard.check(context.Background(), sup, "v0.15.0", quiet)
	if sup.restarts != 1 {
		t.Errorf("restarts = %d after a second look, want still 1", sup.restarts)
	}
}

func TestADaemonItDidNotStartIsLeftAlone(t *testing.T) {
	sup := &fakeSupervision{state: gateway.State{Status: gateway.Stopped}}

	if newVersionGuard("v0.15.2").check(context.Background(), sup, "v0.14.9", quiet) {
		t.Error("the window restarted a daemon it has no record of")
	}
	if sup.restarts != 0 {
		t.Errorf("restarts = %d, want 0", sup.restarts)
	}
}

func TestMatchingAndDeveloperVersionsAreNotAMismatch(t *testing.T) {
	for _, tc := range []struct{ window, daemon string }{
		{"v0.15.2", "v0.15.2"},
		{"v0.15.2", "0.15.2"},
		{"dev", "v0.15.0"},
		{"v0.15.2", "dev"},
		{"v0.15.2", ""},
	} {
		sup := &fakeSupervision{state: gateway.State{Status: gateway.Running, Healthy: true, Meta: &gateway.Meta{PID: 42}}}
		if newVersionGuard(tc.window).check(context.Background(), sup, tc.daemon, quiet) || sup.restarts != 0 {
			t.Errorf("window %q, daemon %q: restarted", tc.window, tc.daemon)
		}
	}
}

type fakeStarter struct {
	fakeSupervision
	starts int
}

func (f *fakeStarter) Start(context.Context, gateway.StartInput) (gateway.State, error) {
	f.starts++
	return gateway.State{Status: gateway.Running, Healthy: true}, nil
}

// A daemon already serving on the port — `aosd serve` in a terminal, `task
// dev`, a lost gateway.json — read as stopped, because the startup asked the
// supervisor's record rather than the port. A second daemon was spawned beside
// it, died on "cannot listen", and its dead pid became the record.
func TestStartupAdoptsADaemonAlreadyServingInsteadOfSpawningAnother(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"aos","status":"ok","version":"dev"}`))
	})
	mux.HandleFunc("/api/auth/status", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"onboarded": true, "authenticated": true})
	})
	mux.HandleFunc("/api/workspace/list", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"workspaces": []map[string]any{{"id": "vs", "path": "/w/vs"}}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	starter := &fakeStarter{fakeSupervision: fakeSupervision{state: gateway.State{Status: gateway.Stopped}}}
	var adopted workspaceRef
	ensureDaemon(starter, daemonclient.New(daemonclient.Options{BaseURL: srv.URL, Token: "t"}), "",
		&chosenWorkspace{}, newVersionGuard("dev"), func(w workspaceRef) { adopted = w }, quiet)

	if starter.starts != 0 {
		t.Errorf("started %d daemons beside the one already serving", starter.starts)
	}
	if adopted.ID != "vs" {
		t.Errorf("adopted %+v, want the workspace of the daemon that is serving", adopted)
	}
}
