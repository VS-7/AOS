package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/transport/daemonclient"
)

// authDaemon answers login and logout, and records which bearer each logout
// presented — a logout is a revocation of exactly that credential.
type authDaemon struct {
	mu      sync.Mutex
	revoked []string
	minted  string
}

func (a *authDaemon) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"user": map[string]any{"id": "u1"}, "token": a.minted})
	})
	mux.HandleFunc("/api/auth/onboarding", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"user": map[string]any{"id": "u1"}, "token": a.minted})
	})
	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.revoked = append(a.revoked, r.Header.Get("authorization"))
		a.mu.Unlock()
		writeData(w, map[string]any{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (a *authDaemon) revocations() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.revoked...)
}

func newTestSession(t *testing.T, root, shared string, explicit bool, base string) (*windowSession, *daemonclient.Client) {
	t.Helper()
	session := newWindowSession(corecfg.Paths{Root: root}.DesktopToken(), shared, explicit, nil)
	client := daemonclient.New(daemonclient.Options{BaseURL: base, Token: session.Initial()})
	return session, client
}

// After Logout in the window, `aos` in a terminal got 401 and every later
// launch asked for a password: the window held ~/.aos/local.token — the
// terminal's credential — as its own session, and Logout revoked whatever it
// held. Nothing writes local.token a second time.
func TestLoggingOutOfTheWindowLeavesTheTerminalsCredentialAlone(t *testing.T) {
	root := t.TempDir()
	daemon := &authDaemon{}
	srv := daemon.server(t)
	session, client := newTestSession(t, root, "cli-token", false, srv.URL)
	caller := session.Caller(client)

	if client.Token() != "cli-token" {
		t.Fatalf("token = %q, want the installation's credential on a first launch", client.Token())
	}
	if err := caller.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, bearer := range daemon.revocations() {
		if bearer == "Bearer cli-token" {
			t.Fatal("logging out of the window revoked the terminal's credential")
		}
	}
	if client.Token() != "" {
		t.Errorf("the window still holds a credential after logging out: %q", client.Token())
	}

	// And it stays signed out: the next launch must not quietly sign in again
	// with the credential it just gave up.
	next, _ := newTestSession(t, root, "cli-token", false, srv.URL)
	if got := next.Initial(); got != "" {
		t.Errorf("next launch token = %q, want none — the person signed out", got)
	}
}

// The window's login was kept only in memory, so once the shared credential
// stopped working every launch started with a 401 and a password prompt.
func TestASignInOutlivesTheWindow(t *testing.T) {
	root := t.TempDir()
	daemon := &authDaemon{minted: "window-session"}
	srv := daemon.server(t)
	session, client := newTestSession(t, root, "", false, srv.URL)

	if _, err := session.Caller(client).Login(context.Background(), "vitor", "secret"); err != nil {
		t.Fatal(err)
	}

	next, _ := newTestSession(t, root, "cli-token", false, srv.URL)
	if got := next.Initial(); got != "window-session" {
		t.Errorf("next launch token = %q, want the session the window signed in with", got)
	}
	info, err := os.Stat(corecfg.Paths{Root: root}.DesktopToken())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("the session file is readable by others: %04o", perm)
	}
}

// The window's own session is the window's to end.
func TestLoggingOutRevokesTheWindowsOwnSession(t *testing.T) {
	root := t.TempDir()
	daemon := &authDaemon{minted: "window-session"}
	srv := daemon.server(t)
	session, client := newTestSession(t, root, "cli-token", false, srv.URL)
	caller := session.Caller(client)

	if _, err := caller.Onboarding(context.Background(), "Vitor", "v@e.test", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := caller.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := daemon.revocations()
	if len(got) != 1 || got[0] != "Bearer window-session" {
		t.Errorf("revoked = %v, want exactly the window's own session", got)
	}
}

// AOS_TOKEN points the window at a daemon it did not start. It wins over
// anything the window remembers, and it is not the window's to revoke.
func TestAnExplicitTokenIsNeitherReplacedNorRevoked(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(corecfg.Paths{Root: root}.DesktopToken(), []byte("remembered"), 0o600); err != nil {
		t.Fatal(err)
	}
	daemon := &authDaemon{}
	srv := daemon.server(t)
	session, client := newTestSession(t, root, "from-env", true, srv.URL)

	if client.Token() != "from-env" {
		t.Fatalf("token = %q, want the one the environment named", client.Token())
	}
	if err := session.Caller(client).Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(daemon.revocations()) != 0 {
		t.Errorf("revoked %v, want nothing — the credential belongs to whoever set AOS_TOKEN", daemon.revocations())
	}
}
