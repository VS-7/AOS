package daemonclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/transport/daemonclient"
)

func ctx() context.Context { return context.Background() }

// TestACommandKeyBecomesThePathTheSurfacePublishes. The rule is the same one
// the frontend's browser transport uses, so a path that works in one works in
// both — and a rule in two places that disagreed would be two bugs.
func TestACommandKeyBecomesThePathTheSurfacePublishes(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})
	for _, key := range []string{"memories_store", "tasks_set-status", "themes_list"} {
		if _, err := client.Invoke(ctx(), key, json.RawMessage(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"/api/memories/store", "/api/tasks/set-status", "/api/themes/list"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", seen, want)
	}
}

// TestTheWorkspaceTravelsInAHeaderAndNotACookie. That is defect #5 of the
// original: a browser attaches a cookie to a WebSocket upgrade whether or not
// the page meant it to, which is what made its realtime channel reachable from
// another origin.
func TestTheWorkspaceTravelsInAHeaderAndNotACookie(t *testing.T) {
	var request *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.Clone(ctx())
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{
		BaseURL: server.URL, Token: "secret-token", Workspace: "atelier",
	})
	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}

	if got := request.Header.Get("x-workspace-id"); got != "atelier" {
		t.Fatalf("workspace header = %q", got)
	}
	if len(request.Cookies()) != 0 {
		t.Fatalf("the client sent cookies: %v", request.Cookies())
	}
	if got := request.Header.Get("authorization"); got != "Bearer secret-token" {
		t.Fatalf("authorization = %q", got)
	}
	if got := request.Header.Get("content-type"); got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}
}

// TestAFailureIsPassedThroughRatherThanReplaced. The domain already said what
// went wrong, in which terms, with which call to action; restating it here
// would lose all three.
func TestAFailureIsPassedThroughRatherThanReplaced(t *testing.T) {
	refusal := `{"error":{"code":"AOS_TASK_REVIEW_BLOCKED","message":"two steps are still open",` +
		`"actions":[{"label":"finish each open step"}]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(refusal))
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})
	raw, err := client.Invoke(ctx(), "tasks_set-status", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("a refusal became a transport error: %v", err)
	}
	if string(raw) != refusal {
		t.Fatalf("the envelope was rewritten:\n%s", raw)
	}
}

// TestADaemonThatIsNotThereIsNamed, with the command that would tell somebody
// what state it is in.
func TestADaemonThatIsNotThereIsNamed(t *testing.T) {
	// A port nothing is listening on. Connection refused is immediate, so this
	// does not wait on a timeout.
	client := daemonclient.New(daemonclient.Options{
		BaseURL: "http://127.0.0.1:1", Timeout: 2 * time.Second,
	})

	_, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("a call to nothing reported success")
	}
	got, ok := apperr.As(err)
	if !ok || !strings.HasSuffix(got.Code, "DAEMON_UNREACHABLE") {
		t.Fatalf("error = %v", err)
	}
	if len(got.Actions) == 0 {
		t.Fatal("the error says the daemon is down and not what to do about it")
	}

	if _, err := client.Commands(ctx()); err == nil {
		t.Fatal("the command listing came back from nothing")
	}
}

// TestTheCommandListingIsWhatAVersionCheckReads.
func TestTheCommandListingIsWhatAVersionCheckReads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/_commands" {
			t.Errorf("the listing was asked for at %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"key":"tasks_list","group":"tasks","name":"list","readOnly":true}]`))
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})
	got, err := client.Commands(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "tasks_list" || !got[0].ReadOnly {
		t.Fatalf("commands = %+v", got)
	}
}

// TestAnAnswerThisBuildCannotReadIsNamedAsSuch, which is the version-mismatch
// case and is worth its own message.
func TestAnAnswerThisBuildCannotReadIsNamedAsSuch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>not this at all</html>`))
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})
	_, err := client.Commands(ctx())
	if err == nil {
		t.Fatal("an answer that is not JSON was read")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "DAEMON_UNREADABLE") {
		t.Fatalf("error = %v", err)
	}
}

// TestReadyIsWhatTheSplashWaitsOn, and a daemon that is not up yet is not an
// incident — it is the thing the splash exists to wait for.
func TestReadyIsWhatTheSplashWaitsOn(t *testing.T) {
	healthy := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("health was asked for at %s", r.URL.Path)
		}
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})
	if ready, err := client.Ready(ctx()); err != nil || !ready {
		t.Fatalf("ready = %v, %v", ready, err)
	}

	healthy = false
	if ready, err := client.Ready(ctx()); err != nil || ready {
		t.Fatalf("an unhealthy daemon reported ready = %v, %v", ready, err)
	}

	down := daemonclient.New(daemonclient.Options{
		BaseURL: "http://127.0.0.1:1", Timeout: 2 * time.Second,
	})
	ready, err := down.Ready(ctx())
	if err != nil {
		t.Fatalf("a daemon that is not up yet reported an error: %v", err)
	}
	if ready {
		t.Fatal("nothing answered and it reported ready")
	}
}

// TestATrailingSlashInTheAddressDoesNotDoubleUp.
func TestATrailingSlashInTheAddressDoesNotDoubleUp(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL + "/"})
	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if path != "/api/tasks/list" {
		t.Fatalf("path = %q", path)
	}
}
