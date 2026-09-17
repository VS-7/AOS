package daemonclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/transport/daemonclient"
)

// A command that reached the daemon and has not been answered yet is not a
// daemon that is not there. Both used to be AOS_DAEMON_UNREACHABLE — every
// error from the HTTP client was — and the window asks again for exactly that
// code, since a daemon it started a moment ago may not be listening yet. So
// update_download on a slow link was cut by the client's thirty seconds and
// sent a second time, which the daemon refused with AOS_UPDATE_IN_PROGRESS;
// a command with no such guard would simply have run twice.

// slowDaemon answers a command after wait, counts what reached it, and
// answers its health check while healthy says so.
type slowDaemon struct {
	*httptest.Server
	commands atomic.Int32
	healthy  atomic.Bool
}

func newSlowDaemon(t *testing.T, wait time.Duration) *slowDaemon {
	t.Helper()
	d := &slowDaemon{}
	d.healthy.Store(true)
	release := make(chan struct{})
	d.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			if !d.healthy.Load() {
				// A daemon that is wedged: it takes the connection and never
				// answers, which is not a refusal a client can see quickly.
				<-release
				return
			}
			_, _ = io.WriteString(w, `{"status":"ok"}`)
			return
		}
		d.commands.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-time.After(wait):
		case <-release:
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"done":true}}`)
	}))
	t.Cleanup(func() {
		close(release)
		d.Close()
	})
	return d
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	got, ok := apperr.As(err)
	if !ok {
		t.Fatalf("the failure is not an apperr: %v", err)
	}
	return got.Code
}

// A command is not bounded by the clock that bounds a question about the
// daemon itself. A download, an install, a large write take as long as they
// take; what ends the wait is the daemon going away, not a number.
func TestACommandSlowerThanTheClientsTimeoutIsAnsweredOnce(t *testing.T) {
	daemon := newSlowDaemon(t, 600*time.Millisecond)
	client := daemonclient.New(daemonclient.Options{
		BaseURL: daemon.URL, Timeout: 100 * time.Millisecond, LivenessInterval: 50 * time.Millisecond,
	})

	raw, err := client.Invoke(ctx(), "update_download", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("a command the daemon was still running was cut: %v", err)
	}
	if !strings.Contains(string(raw), `"done":true`) {
		t.Errorf("answer = %s", raw)
	}
	if n := daemon.commands.Load(); n != 1 {
		t.Errorf("the daemon received the command %d times", n)
	}
}

// When the caller's own deadline ends the wait after the command was sent,
// the answer says the daemon may still be doing it — never that it was not
// there, which is the one code anything asks again for.
func TestACommandThatWasSentAndNotAnsweredInTimeIsNotUnreachable(t *testing.T) {
	daemon := newSlowDaemon(t, time.Hour)
	client := daemonclient.New(daemonclient.Options{BaseURL: daemon.URL})

	deadline, cancel := context.WithTimeout(ctx(), 200*time.Millisecond)
	defer cancel()
	_, err := client.Invoke(deadline, "update_download", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("a command with no answer reported success")
	}
	if code := codeOf(t, err); code != "AOS_DAEMON_TIMEOUT" {
		t.Fatalf("code = %s, want AOS_DAEMON_TIMEOUT: the command reached the daemon", code)
	}
	got, _ := apperr.As(err)
	if !strings.Contains(got.Message, "may still") || len(got.Actions) == 0 {
		t.Errorf("the error does not say the command may still be running, or what to do: %+v", got)
	}
	if n := daemon.commands.Load(); n != 1 {
		t.Errorf("the daemon received the command %d times", n)
	}
}

// A daemon that stops answering altogether ends a command's wait — through
// its health check, since the command's own connection says nothing while
// the daemon is wedged.
func TestAWedgedDaemonEndsTheWaitOfACommandItReceived(t *testing.T) {
	daemon := newSlowDaemon(t, time.Hour)
	client := daemonclient.New(daemonclient.Options{BaseURL: daemon.URL, LivenessInterval: 40 * time.Millisecond})
	daemon.healthy.Store(false)

	outcome := make(chan error, 1)
	go func() {
		_, err := client.Invoke(ctx(), "skills_install", json.RawMessage(`{}`))
		outcome <- err
	}()
	select {
	case err := <-outcome:
		if code := codeOf(t, err); code != "AOS_DAEMON_TIMEOUT" {
			t.Fatalf("code = %s, want AOS_DAEMON_TIMEOUT", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a command waited on a daemon that stopped answering its health check")
	}
	if n := daemon.commands.Load(); n != 1 {
		t.Errorf("the daemon received the command %d times", n)
	}
}

// The connection closing after the command was sent — a daemon that crashed
// halfway through it — is the same uncertainty, and it is not retried either.
func TestAConnectionThatClosesAfterTheCommandWasSentIsNotUnreachable(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})
	_, err := client.Invoke(ctx(), "tasks_create", json.RawMessage(`{}`))
	if code := codeOf(t, err); code != "AOS_DAEMON_ANSWER_LOST" {
		t.Fatalf("code = %s, want AOS_DAEMON_ANSWER_LOST", code)
	}
	if n := received.Load(); n != 1 {
		t.Errorf("the daemon received the command %d times", n)
	}
}

// Nothing listening is still what it was: the request went nowhere, which is
// the one failure that is safe to send again.
func TestARefusedConnectionIsStillUnreachableOnEverySurface(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := dead.URL
	dead.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: base})
	for name, call := range map[string]func() error{
		"Invoke": func() error { _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); return err },
		"Fetch": func() error {
			_, _, err := client.Fetch(ctx(), http.MethodPut, "/api/file/write", "application/json", []byte(`{}`))
			return err
		},
		"Stream": func() error { _, err := client.Stream(ctx(), "/api/file/content?path=a", http.Header{}); return err },
	} {
		if code := codeOf(t, call()); code != "AOS_DAEMON_UNREACHABLE" {
			t.Errorf("%s: code = %s, want AOS_DAEMON_UNREACHABLE", name, code)
		}
	}
}

// A file write goes through Fetch, and a large one is exactly the slow
// command this is about.
func TestAFileRequestSlowerThanTheClientsTimeoutIsAnsweredOnce(t *testing.T) {
	daemon := newSlowDaemon(t, 400*time.Millisecond)
	client := daemonclient.New(daemonclient.Options{
		BaseURL: daemon.URL, Timeout: 100 * time.Millisecond, LivenessInterval: 50 * time.Millisecond,
	})

	status, _, err := client.Fetch(ctx(), http.MethodPut, "/api/file/write", "application/json", []byte(`{}`))
	if err != nil || status != http.StatusOK {
		t.Fatalf("a file write the daemon was still doing was cut: %d, %v", status, err)
	}
	if n := daemon.commands.Load(); n != 1 {
		t.Errorf("the daemon received the write %d times", n)
	}
}

// A video is streamed through Stream for as long as somebody watches it. The
// client-wide timeout also covered reading the body, so playback through the
// window stopped after thirty seconds.
func TestAStreamIsNotCutWhileItsBodyKeepsArriving(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "video/mp4")
		flusher, _ := w.(http.Flusher)
		for range 6 {
			_, _ = io.WriteString(w, "frame")
			flusher.Flush()
			time.Sleep(60 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL, Timeout: 100 * time.Millisecond})
	res, err := client.Stream(ctx(), "/api/file/content?path=a.mp4", http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("the stream was cut after %d bytes: %v", len(body), err)
	}
	if string(body) != strings.Repeat("frame", 6) {
		t.Errorf("body = %q", body)
	}
}

// What bounds a stream is the wait for the daemon to start answering it.
func TestAStreamWhoseAnswerNeverStartsIsAbandoned(t *testing.T) {
	daemon := newSlowDaemon(t, time.Hour)
	client := daemonclient.New(daemonclient.Options{BaseURL: daemon.URL, Timeout: 100 * time.Millisecond})

	started := time.Now()
	_, err := client.Stream(ctx(), "/api/file/content?path=a.mp4", http.Header{})
	// A read, so the daemon not answering rather than something left in doubt
	// (see TestAnUnansweredQuestionIsTheDaemonNotAnswering).
	if code := codeOf(t, err); code != "AOS_DAEMON_UNREACHABLE" {
		t.Fatalf("code = %s, want AOS_DAEMON_UNREACHABLE", code)
	}
	if waited := time.Since(started); waited > 3*time.Second {
		t.Errorf("waited %s for an answer that never started", waited)
	}
}

// A question about the daemon — who is signed in, what it publishes, whether
// it is healthy — asked of a daemon that took it and never answered came back
// as AOS_DAEMON_TIMEOUT: "received /api/auth/status and has not answered it;
// it may still be doing it", with a call to action warning that asking again
// could do it twice. A read done twice is a read, and the code stopped being
// the one the sign-in screen recognises as the daemon not answering, so it
// showed that sentence instead. A read that goes unanswered is the daemon not
// answering; only a request that can change something is left in doubt.
func TestAnUnansweredQuestionIsTheDaemonNotAnswering(t *testing.T) {
	daemon := newSlowDaemon(t, time.Hour)
	daemon.healthy.Store(false)
	client := daemonclient.New(daemonclient.Options{BaseURL: daemon.URL, Timeout: 100 * time.Millisecond, Token: "t"})

	questions := map[string]func() error{
		"Status":   func() error { _, err := client.Status(ctx()); return err },
		"Session":  func() error { _, err := client.Session(ctx()); return err },
		"Commands": func() error { _, err := client.Commands(ctx()); return err },
		"Manifest": func() error { _, err := client.Manifest(ctx()); return err },
		"Health":   func() error { _, err := client.Health(ctx()); return err },
		"Stream": func() error {
			_, err := client.Stream(ctx(), "/api/file/content?path=a.mp4", http.Header{})
			return err
		},
	}
	for name, ask := range questions {
		err := ask()
		if code := codeOf(t, err); code != "AOS_DAEMON_UNREACHABLE" {
			t.Errorf("%s: code = %s, want AOS_DAEMON_UNREACHABLE", name, code)
		}
		if strings.Contains(strings.ToLower(err.Error()), "twice") || strings.Contains(err.Error(), "may still be doing it") {
			t.Errorf("%s: a read is reported as something that could run twice: %v", name, err)
		}
	}

	// Signing in can mint a session, so its fate stays in doubt.
	if _, err := client.Login(ctx(), "vitor", "pw"); codeOf(t, err) != "AOS_DAEMON_TIMEOUT" {
		t.Errorf("Login: code = %s, want AOS_DAEMON_TIMEOUT", codeOf(t, err))
	}
}
