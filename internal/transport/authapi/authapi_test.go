package authapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/transport/authapi"
)

const goodPassword = "ruivo bicicleta trovoada 42"

type fakeStore struct {
	mu    sync.Mutex
	users []auth.User
}

func (s *fakeStore) Load(context.Context) ([]auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(s.users)
	var out []auth.User
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

func (s *fakeStore) Save(_ context.Context, users []auth.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = users
	return nil
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	svc := auth.NewService(auth.Deps{
		Store: &fakeStore{},
		Clock: clockx.Fixed{At: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)},
		IDs:   &ids.Sequence{Prefix: "u"},
	})
	handler := authapi.New(authapi.Config{Service: svc})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func post(t *testing.T, srv *httptest.Server, path string, body any, bearer string) (*http.Response, envelope) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var env envelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return res, env
}

func get(t *testing.T, srv *httptest.Server, path, bearer string) (*http.Response, envelope) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var env envelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return res, env
}

func TestStatusBeforeOnboardingSaysSo(t *testing.T) {
	srv := newServer(t)
	res, env := get(t, srv, "/status", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var out struct {
		Onboarded     bool `json:"onboarded"`
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Onboarded || out.Authenticated {
		t.Fatalf("got %+v, want both false", out)
	}
}

func TestOnboardingThenStatusThenLoginRoundTrip(t *testing.T) {
	srv := newServer(t)

	res, env := post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("onboarding status = %d, body = %s", res.StatusCode, env.Data)
	}
	var onboarded struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(env.Data, &onboarded); err != nil {
		t.Fatal(err)
	}
	if onboarded.Token == "" || onboarded.User.Email != "vitor@example.test" {
		t.Fatalf("got %+v", onboarded)
	}

	// A second onboarding is refused: the installation already has an account.
	res, env = post(t, srv, "/onboarding", map[string]string{
		"name": "Someone Else", "email": "other@example.test", "password": goodPassword,
	}, "")
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("second onboarding status = %d", res.StatusCode)
	}
	if env.Error == nil || env.Error.Code != "AOS_AUTH_ALREADY_ONBOARDED" {
		t.Fatalf("second onboarding error = %+v", env.Error)
	}

	_, statusEnv := get(t, srv, "/status", "")
	var status struct {
		Onboarded     bool `json:"onboarded"`
		Authenticated bool `json:"authenticated"`
	}
	_ = json.Unmarshal(statusEnv.Data, &status)
	if !status.Onboarded || status.Authenticated {
		t.Fatalf("got %+v, want onboarded and not authenticated (no credential sent)", status)
	}

	// Logging in with the account onboarding created.
	res, env = post(t, srv, "/login", map[string]string{
		"identifier": "vitor@example.test", "password": goodPassword,
	}, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", res.StatusCode, env.Data)
	}
	var logged struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &logged)
	if logged.Token == "" {
		t.Fatal("login returned no token")
	}

	// The session endpoint recognises the fresh session token.
	res, env = get(t, srv, "/session", logged.Token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", res.StatusCode, env.Data)
	}
}

func TestLoginWithTheWrongPasswordIsRefused(t *testing.T) {
	srv := newServer(t)
	post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")

	res, env := post(t, srv, "/login", map[string]string{
		"identifier": "vitor@example.test", "password": "wrong password entirely",
	}, "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if env.Error == nil || env.Error.Code != "AOS_AUTH_INVALID_CREDENTIALS" {
		t.Fatalf("error = %+v", env.Error)
	}
}

func TestSessionWithNoCredentialIsUnauthenticated(t *testing.T) {
	srv := newServer(t)
	res, _ := get(t, srv, "/session", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestLogoutRevokesOnlyThePresentedSession(t *testing.T) {
	srv := newServer(t)
	_, env := post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")
	var onboarded struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &onboarded)

	_, env = post(t, srv, "/login", map[string]string{
		"identifier": "vitor@example.test", "password": goodPassword,
	}, "")
	var second struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &second)

	res, _ := post(t, srv, "/logout", map[string]string{}, onboarded.Token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", res.StatusCode)
	}

	res, _ = get(t, srv, "/session", onboarded.Token)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatal("the revoked session should no longer authenticate")
	}
	res, _ = get(t, srv, "/session", second.Token)
	if res.StatusCode != http.StatusOK {
		t.Fatal("the other session should still work")
	}
}

func TestChangePasswordThenLoginWithTheNewOne(t *testing.T) {
	srv := newServer(t)
	_, env := post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")
	var onboarded struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &onboarded)

	const newPassword = "trombone abacaxi lanterna 77"
	res, _ := post(t, srv, "/password", map[string]string{
		"currentPassword": goodPassword, "newPassword": newPassword,
	}, onboarded.Token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("change password status = %d", res.StatusCode)
	}

	res, _ = post(t, srv, "/login", map[string]string{
		"identifier": "vitor@example.test", "password": newPassword,
	}, "")
	if res.StatusCode != http.StatusOK {
		t.Fatal("login with the new password should succeed")
	}
}

// TestUsersListsTheAccountsOnThisInstallation.
//
// The interface has three screens that need to know who else is here — the
// workspace roster the sidebar's Team tab draws, the assignee picker on a
// task, and Settings → Users — and every one of them rendered empty because
// nothing published the list. The domain has had it all along
// (auth.Service.Users); this is the surface it was missing.
func TestUsersListsTheAccountsOnThisInstallation(t *testing.T) {
	srv := newServer(t)

	// Signed out: an unauthenticated caller learns nothing about who has an
	// account here.
	res, _ := get(t, srv, "/users", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without a credential = %d, want 401", res.StatusCode)
	}

	_, env := post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &session); err != nil {
		t.Fatal(err)
	}

	res, env = get(t, srv, "/users", session.Token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.StatusCode, env.Data)
	}
	var out struct {
		Users []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Username string `json:"username"`
			Role     string `json:"role"`
			Password string `json:"password"`
		} `json:"users"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Users) != 1 {
		t.Fatalf("got %d users, want the one that was onboarded", len(out.Users))
	}
	got := out.Users[0]
	if got.Name != "Vitor" || got.Email != "vitor@example.test" || got.Role != "super" {
		t.Fatalf("user = %+v", got)
	}
	if got.ID == "" {
		t.Error("the account has no id, so nothing can reference it")
	}
	// The listing is a roster, not a credential dump.
	if got.Password != "" {
		t.Error("the password hash was published")
	}
	if strings.Contains(string(env.Data), "$argon2id$") {
		t.Error("a password hash reached the response body")
	}
}

// TestProfileChangesTheNameEveryScreenShows.
//
// The account settings page has a name field and a save button; until this
// route existed, saving it reported "Profile editing isn't wired up in this
// build yet" — the store action behind it was a disclosed stub, because there
// was nothing to call. This is what it calls.
func TestProfileChangesTheNameEveryScreenShows(t *testing.T) {
	srv := newServer(t)
	_, env := post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")
	var onboarded struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &onboarded)

	res, updated := post(t, srv, "/profile", map[string]string{
		"name": "Vitor Sérgio", "email": "vitor@example.test",
	}, onboarded.Token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("profile status = %d", res.StatusCode)
	}

	// Same shape as /session, so the interface can put the answer straight
	// back into the store it read /session into.
	var body struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	if err := json.Unmarshal(updated.Data, &body); err != nil {
		t.Fatal(err)
	}
	if body.User.Name != "Vitor Sérgio" {
		t.Errorf("answered name = %q", body.User.Name)
	}

	_, after := get(t, srv, "/session", onboarded.Token)
	if err := json.Unmarshal(after.Data, &body); err != nil {
		t.Fatal(err)
	}
	if body.User.Name != "Vitor Sérgio" {
		t.Errorf("the session still reports %q", body.User.Name)
	}
}

// TestProfileNeedsASession. It edits an account, so it may not be reachable by
// anyone who has not proved which account is theirs.
func TestProfileNeedsASession(t *testing.T) {
	srv := newServer(t)
	post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")

	res, _ := post(t, srv, "/profile", map[string]string{"name": "Alguém"}, "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
}

// TestProfileRefusesABlankName, at the surface as well as in the domain: a
// 400 with the field named is what lets the form point at the input.
func TestProfileRefusesABlankName(t *testing.T) {
	srv := newServer(t)
	_, env := post(t, srv, "/onboarding", map[string]string{
		"name": "Vitor", "email": "vitor@example.test", "password": goodPassword,
	}, "")
	var onboarded struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(env.Data, &onboarded)

	res, body := post(t, srv, "/profile", map[string]string{"name": " "}, onboarded.Token)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if body.Error == nil || body.Error.Code != "AOS_AUTH_NAME_REQUIRED" {
		t.Errorf("error = %+v, want AOS_AUTH_NAME_REQUIRED", body.Error)
	}
}

// newServerWithState is newServer plus a state directory, so a test can read
// the credential onboarding writes for the terminal.
func newServerWithState(t *testing.T) (*httptest.Server, corecfg.Paths) {
	t.Helper()
	svc := auth.NewService(auth.Deps{
		Store: &fakeStore{},
		Clock: clockx.Fixed{At: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)},
		IDs:   &ids.Sequence{Prefix: "u"},
	})
	paths := corecfg.Paths{Root: t.TempDir()}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(authapi.New(authapi.Config{Service: svc, Paths: paths}))
	t.Cleanup(srv.Close)
	return srv, paths
}

// Signing out of one client must not disarm another.
//
// Onboarding writes the terminal's credential to local.token and hands the
// window a session of its own. They used to be the same string, so the first
// logout revoked both — and nothing ever writes local.token again, so `aos`
// stayed broken for good: every domain command came back "unknown command".
func TestLoggingOutDoesNotRevokeTheTerminalsCredential(t *testing.T) {
	srv, paths := newServerWithState(t)

	res, env := post(t, srv, "/onboarding", map[string]string{
		"name": "A", "email": "a@b.c", "password": "a-good-long-password",
	}, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("onboarding = %d", res.StatusCode)
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &session); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(paths.LocalToken())
	if err != nil {
		t.Fatalf("local.token was not written: %v", err)
	}
	cliToken := strings.TrimSpace(string(raw))
	if cliToken == "" {
		t.Fatal("local.token is empty")
	}
	if cliToken == session.Token {
		t.Fatal("the window and the terminal hold the same credential; revoking one revokes the other")
	}

	if res, _ := post(t, srv, "/logout", map[string]string{}, session.Token); res.StatusCode != http.StatusOK {
		t.Fatalf("logout = %d", res.StatusCode)
	}

	if res, _ := get(t, srv, "/session", session.Token); res.StatusCode == http.StatusOK {
		t.Error("the window's own token survived its logout")
	}
	if res, _ := get(t, srv, "/session", cliToken); res.StatusCode != http.StatusOK {
		t.Errorf("the terminal's credential was revoked by the window's logout: %d", res.StatusCode)
	}
}
