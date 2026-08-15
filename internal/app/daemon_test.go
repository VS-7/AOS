package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/coder/websocket"

	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/transport/httpapi"
	"github.com/OWNER/aos/internal/transport/realtime"
)

// configOff switches authentication off, which is the state a loopback-only
// installation may legitimately be in.
func configOff() config.UpdateInput {
	return config.UpdateInput{Set: map[string]any{"security.enabled": false}}
}

// serving starts the daemon on a free port and returns its address. The caller
// gets a daemon that is actually listening, not one that was asked to.
func serving(t *testing.T, settings map[string]string) (string, *app.App) {
	t.Helper()
	port := freePort(t)
	if settings == nil {
		settings = map[string]string{}
	}
	settings[env.KeyHome] = t.TempDir()
	settings[env.KeyServerPort] = strconv.Itoa(port)

	a, err := app.New(app.Options{
		Env:           env.New(env.Map(settings)),
		WorkspaceRoot: t.TempDir(),
		Clock:         clockx.Fixed{At: refTime},
		IDs:           &ids.Sequence{Prefix: "m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- a.Serve(ctx, app.ServeOptions{
			Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
			ShutdownTimeout: 2 * time.Second,
			Ready:           func(addr string) { ready <- addr },
		})
	}()

	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("the daemon stopped before it started serving: %v", err)
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("the daemon never reported that it was listening")
	}

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("the daemon reported an error on shutdown: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("the daemon did not shut down")
		}
	})
	return "http://" + addr, a
}

// TestTheDaemonServesAndShutsDownCleanly is the shape of the phase's delivery:
// a process that listens, answers, and stops when told.
func TestTheDaemonServesAndShutsDownCleanly(t *testing.T) {
	base, _ := serving(t, nil)

	res, err := http.Get(base + "/api/health") //nolint:noctx // a health probe in a test
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("health = %d", res.StatusCode)
	}
	var body map[string]any
	raw, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["name"] != "aos" {
		t.Fatalf("health = %s", raw)
	}
	// The cleanup registered by serving() is the rest of the assertion: it
	// cancels the context and fails if the daemon does not stop.
}

// TestTheDaemonRefusesToExposeItselfWithTheDoorOpen is ADR-0009 and defect #2.
// The original binds 0.0.0.0 by default. Binding wide is allowed here; doing it
// with authentication off is not.
func TestTheDaemonRefusesToExposeItselfWithTheDoorOpen(t *testing.T) {
	home := t.TempDir()
	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome:       home,
			env.KeyServerHost: "0.0.0.0",
			env.KeyServerPort: strconv.Itoa(freePort(t)),
		})),
		WorkspaceRoot: t.TempDir(),
		Clock:         clockx.Fixed{At: refTime},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()

	// Security is off in this installation's configuration.
	if _, err := a.Config.Update(context.Background(), configOff()); err != nil {
		t.Fatal(err)
	}

	err = a.Serve(context.Background(), app.ServeOptions{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("the daemon exposed itself with authentication switched off")
	}
	if !strings.Contains(err.Error(), "refusing to listen") {
		t.Fatalf("error = %v", err)
	}
}

// TestLoopbackWithoutAuthenticationIsAllowed is the other half of the same
// rule: a daemon nobody outside the machine can reach does not need a door.
func TestLoopbackWithoutAuthenticationIsAllowed(t *testing.T) {
	base, a := serving(t, nil)
	if _, err := a.Config.Update(context.Background(), configOff()); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(base + "/api/health") //nolint:noctx // a health probe in a test
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health = %d", res.StatusCode)
	}
}

// TestACommandRoundTripsThroughTheRunningDaemon closes the loop: a real client,
// over a real socket, to a route the registry generated.
func TestACommandRoundTripsThroughTheRunningDaemon(t *testing.T) {
	base, a := serving(t, nil)
	if _, err := a.Config.Update(context.Background(), configOff()); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		base+"/api/gateway/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d: %s", res.StatusCode, body)
	}
	if res.Header.Get(httpapi.HeaderRequestID) == "" {
		t.Error("no correlation id on a real response")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // asking the kernel for a free port
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// TestTheSocketIsAuthorisedAgainstTheWorkspace is defect #5, checked against
// the running daemon rather than against the handler: the original reads the
// workspace out of a cookie and attaches the socket without asking whether the
// caller has any claim to it.
func TestTheSocketIsAuthorisedAgainstTheWorkspace(t *testing.T) {
	base, a := serving(t, nil)
	if _, err := a.Config.Update(context.Background(), configOff()); err != nil {
		t.Fatal(err)
	}
	// One workspace exists, with a member who is not the caller.
	created, err := a.Workspaces.Create(context.Background(), workspace.CreateInput{
		Name: "Mine", Path: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Workspaces.Update(context.Background(), workspace.UpdateInput{
		Workspace: created.Workspace.ID,
		Set: map[string]any{"members": []map[string]any{
			{"userId": "somebody-else", "role": "owner", "addedAt": refTime},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	socketURL := "ws" + strings.TrimPrefix(base, "http") + "/ws"

	t.Run("a workspace the caller is not in is refused", func(t *testing.T) {
		headers := http.Header{}
		headers.Set(httpapi.HeaderWorkspace, created.Workspace.ID)
		conn, res, err := websocket.Dial(t.Context(), socketURL, &websocket.DialOptions{HTTPHeader: headers})
		if err == nil {
			_ = conn.Close(websocket.StatusNormalClosure, "")
			t.Fatal("the socket was granted")
		}
		if res == nil || res.StatusCode != http.StatusForbidden {
			t.Fatalf("response = %v", res)
		}
	})

	t.Run("a workspace that does not exist is refused", func(t *testing.T) {
		headers := http.Header{}
		headers.Set(httpapi.HeaderWorkspace, "invented")
		conn, res, err := websocket.Dial(t.Context(), socketURL, &websocket.DialOptions{HTTPHeader: headers})
		if err == nil {
			_ = conn.Close(websocket.StatusNormalClosure, "")
			t.Fatal("the socket was granted for a workspace that does not exist")
		}
		if res == nil || res.StatusCode != http.StatusForbidden {
			t.Fatalf("response = %v", res)
		}
	})

	t.Run("events arrive on a workspace the caller may read", func(t *testing.T) {
		// A workspace with no members is a single-user installation, which is
		// what every local one is until a second account exists.
		open, err := a.Workspaces.Create(context.Background(), workspace.CreateInput{
			Name: "Open", Path: t.TempDir(),
		})
		if err != nil {
			t.Fatal(err)
		}
		headers := http.Header{}
		headers.Set(httpapi.HeaderWorkspace, open.Workspace.ID)

		conn, _, err := websocket.Dial(t.Context(), socketURL, &websocket.DialOptions{HTTPHeader: headers})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		channel := realtime.ChannelFor(open.Workspace.ID)
		deadline := time.Now().Add(5 * time.Second)
		for a.Events.Subscribers(channel) == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		a.Events.Publish(t.Context(), channel, realtime.Event{Type: realtime.EventActivity})

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_, payload, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var e realtime.Event
		if err := json.Unmarshal(payload, &e); err != nil {
			t.Fatal(err)
		}
		if e.Type != realtime.EventActivity {
			t.Fatalf("event = %+v", e)
		}
	})
}
