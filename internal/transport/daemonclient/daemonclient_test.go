package daemonclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// The auth surface is the one part of this client that does not go through
// Invoke: identity has no command group to route through (see
// internal/domain/auth's package doc), so it speaks to /api/auth directly.
// What follows holds that separate path to the same standards as the rest.

// authServer records every request and answers each auth route from a table,
// so a test states the daemon's answer and nothing else.
type authServer struct {
	*httptest.Server
	requests []*http.Request
	bodies   []string
	answers  map[string]string
	status   map[string]int
}

func newAuthServer(t *testing.T, answers map[string]string) *authServer {
	t.Helper()
	a := &authServer{answers: answers, status: map[string]int{}}
	a.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body strings.Builder
		if r.Body != nil {
			_, _ = io.Copy(&body, r.Body)
		}
		a.requests = append(a.requests, r.Clone(r.Context()))
		a.bodies = append(a.bodies, body.String())

		w.Header().Set("content-type", "application/json")
		if code, ok := a.status[r.URL.Path]; ok {
			w.WriteHeader(code)
		}
		answer, ok := a.answers[r.URL.Path]
		if !ok {
			answer = `{"data":{}}`
		}
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(a.Close)
	return a
}

func (a *authServer) lastHeader(name string) string {
	if len(a.requests) == 0 {
		return ""
	}
	return a.requests[len(a.requests)-1].Header.Get(name)
}

// Status is the one auth call that must work without a credential — it is
// what the window asks before it knows whether it has one.
func TestStatusAnswersWithoutACredential(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/status": `{"data":{"onboarded":true,"authenticated":false}}`,
	})
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	got, err := client.Status(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Onboarded || got.Authenticated {
		t.Fatalf("status = %+v, want onboarded and not authenticated", got)
	}
	if h := server.lastHeader("authorization"); h != "" {
		t.Fatalf("a client with no token sent authorization = %q", h)
	}
}

func TestStatusCarriesTheTokenWhenThereIsOne(t *testing.T) {
	server := newAuthServer(t, nil)
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL, Token: "abc"})

	if _, err := client.Status(ctx()); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("authorization"); h != "Bearer abc" {
		t.Fatalf("authorization = %q, want Bearer abc", h)
	}
}

// The token never leaves Login: the caller gets who signed in, and the
// credential stays inside the client — the desktop-side equivalent of the
// browser's HttpOnly cookie being unreadable from JS.
func TestLoginStoresTheTokenAndReturnsOnlyTheUser(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/login": `{"data":{"user":{"id":"u1","name":"Vitor","email":"v@example.com",` +
			`"username":"v@example.com","role":"owner"},"token":"minted-token",` +
			`"expiresAt":"2026-09-01T10:00:00Z"}}`,
	})
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	got, err := client.Login(ctx(), "v@example.com", "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if got.User.ID != "u1" || got.User.Name != "Vitor" {
		t.Fatalf("user = %+v", got.User)
	}
	if !strings.HasPrefix(got.ExpiresAt, "2026-09-01T10:00:00") {
		t.Fatalf("expiresAt = %q", got.ExpiresAt)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), "minted-token") {
		t.Fatalf("the token leaked into the result: %+v", got)
	}
	// The proof it was stored is that the next call carries it.
	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("authorization"); h != "Bearer minted-token" {
		t.Fatalf("the call after login sent authorization = %q", h)
	}
}

func TestLoginSendsTheIdentifierAndPasswordAsJSON(t *testing.T) {
	server := newAuthServer(t, nil)
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	if _, err := client.Login(ctx(), "vitor", "hunter2"); err != nil {
		t.Fatal(err)
	}
	var sent map[string]string
	if err := json.Unmarshal([]byte(server.bodies[0]), &sent); err != nil {
		t.Fatalf("the body is not JSON: %q", server.bodies[0])
	}
	if sent["identifier"] != "vitor" || sent["password"] != "hunter2" {
		t.Fatalf("body = %v", sent)
	}
	if ct := server.lastHeader("content-type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestOnboardingSendsTheThreeFieldsTheWizardCollects(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/onboarding": `{"data":{"user":{"id":"u1","name":"Vitor"},"token":"first-token"}}`,
	})
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	got, err := client.Onboarding(ctx(), "Vitor", "v@example.com", "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if got.User.ID != "u1" {
		t.Fatalf("user = %+v", got.User)
	}
	var sent map[string]string
	if err := json.Unmarshal([]byte(server.bodies[0]), &sent); err != nil {
		t.Fatalf("the body is not JSON: %q", server.bodies[0])
	}
	if sent["name"] != "Vitor" || sent["email"] != "v@example.com" || sent["password"] != "hunter2" {
		t.Fatalf("body = %v", sent)
	}
	if server.requests[0].URL.Path != "/api/auth/onboarding" {
		t.Fatalf("path = %q", server.requests[0].URL.Path)
	}
}

// A refusal from the daemon has to arrive as an apperr carrying the daemon's
// own code — the desktop shows that message, and replacing it with a generic
// transport failure would tell the person nothing about why they were denied.
func TestARefusedLoginKeepsTheDaemonsOwnCode(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/login": `{"error":{"code":"AOS_AUTH_INVALID_CREDENTIALS","message":"that password does not match"}}`,
	})
	server.status["/api/auth/login"] = http.StatusUnauthorized
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	_, err := client.Login(ctx(), "vitor", "wrong")
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	if app.Code != "AOS_AUTH_INVALID_CREDENTIALS" {
		t.Fatalf("code = %q, want AOS_AUTH_INVALID_CREDENTIALS", app.Code)
	}
	if !strings.Contains(app.Message, "that password does not match") {
		t.Fatalf("message = %q, want the daemon's own", app.Message)
	}
	// A failed login must not have stored anything.
	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("authorization"); h != "" {
		t.Fatalf("a failed login left a credential behind: %q", h)
	}
}

func TestSessionReportsWhoTheHeldTokenBelongsTo(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/session": `{"data":{"user":{"id":"u1","name":"Vitor","role":"owner"}}}`,
	})
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL, Token: "abc"})

	got, err := client.Session(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "u1" || got.Role != "owner" {
		t.Fatalf("user = %+v", got)
	}
	if h := server.lastHeader("authorization"); h != "Bearer abc" {
		t.Fatalf("authorization = %q", h)
	}
}

func TestSessionSurfacesARefusalAsTheDaemonsError(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/session": `{"error":{"code":"AOS_AUTH_HTTP_UNAUTHENTICATED","message":"no session"}}`,
	})
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL, Token: "stale"})

	_, err := client.Session(ctx())
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	if app.Code != "AOS_AUTH_HTTP_UNAUTHENTICATED" {
		t.Fatalf("code = %q", app.Code)
	}
}

// Logout forgets the token whatever the daemon says: a client that kept a
// credential after the person asked to sign out is worse than one that
// dropped a token the server had already revoked anyway.
func TestLogoutForgetsTheTokenLocally(t *testing.T) {
	server := newAuthServer(t, nil)
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL, Token: "abc"})

	if err := client.Logout(ctx()); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("authorization"); h != "Bearer abc" {
		t.Fatalf("logout did not present the token it was revoking: %q", h)
	}
	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("authorization"); h != "" {
		t.Fatalf("the client still holds a credential after logout: %q", h)
	}
}

// A daemon that is not there has to be named as such on the auth routes too,
// with the command that would tell somebody what state it is in — the same
// treatment Invoke already gets.
func TestTheAuthRoutesNameAnUnreachableDaemon(t *testing.T) {
	// A server that is closed immediately: the address is well-formed and
	// refuses connections, which is exactly a daemon that is not running.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := dead.URL
	dead.Close()

	client := daemonclient.New(daemonclient.Options{BaseURL: base, Token: "abc"})
	for name, call := range map[string]func() error{
		"Status":     func() error { _, err := client.Status(ctx()); return err },
		"Login":      func() error { _, err := client.Login(ctx(), "a", "b"); return err },
		"Onboarding": func() error { _, err := client.Onboarding(ctx(), "a", "b", "c"); return err },
		"Session":    func() error { _, err := client.Session(ctx()); return err },
		"Logout":     func() error { return client.Logout(ctx()) },
	} {
		var app *apperr.Error
		if !errors.As(call(), &app) {
			t.Fatalf("%s: the failure is not an apperr", name)
		}
		if app.Code != "AOS_DAEMON_UNREACHABLE" {
			t.Fatalf("%s: code = %q, want AOS_DAEMON_UNREACHABLE", name, app.Code)
		}
	}
}

// An answer this build cannot decode is a version mismatch, not a refusal.
func TestAnAuthAnswerThisBuildCannotReadIsNamedAsSuch(t *testing.T) {
	server := newAuthServer(t, map[string]string{
		"/api/auth/status":  `this is not json`,
		"/api/auth/session": `this is not json`,
		"/api/auth/login":   `this is not json`,
	})
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	for name, call := range map[string]func() error{
		"Status":  func() error { _, err := client.Status(ctx()); return err },
		"Session": func() error { _, err := client.Session(ctx()); return err },
		"Login":   func() error { _, err := client.Login(ctx(), "a", "b"); return err },
	} {
		var app *apperr.Error
		if !errors.As(call(), &app) {
			t.Fatalf("%s: the failure is not an apperr", name)
		}
		if app.Code != "AOS_DAEMON_UNREADABLE" {
			t.Fatalf("%s: code = %q, want AOS_DAEMON_UNREADABLE", name, app.Code)
		}
	}
}

// SetWorkspace and SetToken exist because neither answer is known when the
// client is constructed — the daemon may not be running yet.
func TestTheWorkspaceAndTokenCanBeSetAfterConstruction(t *testing.T) {
	server := newAuthServer(t, nil)
	client := daemonclient.New(daemonclient.Options{BaseURL: server.URL})

	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("x-workspace-id"); h != "" {
		t.Fatalf("a client built without a workspace sent one: %q", h)
	}

	client.SetToken("later-token")
	client.SetWorkspace("atelier")
	if _, err := client.Invoke(ctx(), "tasks_list", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if h := server.lastHeader("authorization"); h != "Bearer later-token" {
		t.Fatalf("authorization = %q", h)
	}
	if h := server.lastHeader("x-workspace-id"); h != "atelier" {
		t.Fatalf("x-workspace-id = %q", h)
	}
}
