package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/transport/daemonclient"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// fakeDaemon answers the three calls openWorkspace makes, and records them.
type fakeDaemon struct {
	mu         sync.Mutex
	calls      []string
	workspaces []map[string]any
}

func (f *fakeDaemon) record(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, path)
}

func (f *fakeDaemon) called(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == path {
			return true
		}
	}
	return false
}

func (f *fakeDaemon) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/workspace/list", func(w http.ResponseWriter, r *http.Request) {
		f.record("workspace_list")
		writeData(w, map[string]any{"workspaces": f.workspaces})
	})
	mux.HandleFunc("/api/workspace/introspect", func(w http.ResponseWriter, r *http.Request) {
		f.record("workspace_introspect")
		// A real introspect registers, so the fake does too: a second list
		// would see it, which is exactly how the bug hid.
		f.mu.Lock()
		created := map[string]any{"id": "the-directory", "path": "/tmp/the-directory"}
		f.workspaces = append(f.workspaces, created)
		f.mu.Unlock()
		writeData(w, map[string]any{"workspace": created, "orchestrator": "atlas"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func writeData(w http.ResponseWriter, data any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

// TestOnboardingDoesNotRegisterAWorkspaceBehindTheWizard is defect #1 and #2
// of the desktop audit, which are one defect.
//
// AuthService calls afterAuth inside Onboarding, before the answer reaches the
// window. afterAuth is openWorkspace, and on a fresh installation the registry
// is empty, so it introspected — registering a workspace named after the
// directory, with the default orchestrator, "Atlas".
//
// By the time the wizard's own `workspace_create` ran — carrying the name, the
// tone, the style and the autonomy the person had just chosen — a workspace
// already existed, so it did nothing. The copilot was always called Atlas and
// the workspace was always named after the folder, whatever anyone typed.
//
// The wizard owns the first workspace. Onboarding adopts; it does not create.
func TestOnboardingDoesNotRegisterAWorkspaceBehindTheWizard(t *testing.T) {
	daemon := &fakeDaemon{}
	srv := daemon.server(t)
	client := daemonclient.New(daemonclient.Options{BaseURL: srv.URL})

	_, err := openWorkspace(context.Background(), client, "/home/me/project", wailsvc.AuthOnboarding)
	if err == nil {
		t.Fatal("with nothing registered yet, onboarding has no workspace to adopt and must say so")
	}
	if daemon.called("workspace_introspect") {
		t.Error("onboarding registered a workspace before the wizard could name it")
	}
}

// TestOnboardingStillAdoptsAWorkspaceThatAlreadyExists: not creating is not
// the same as doing nothing. A registry that already holds one — a second
// window, or an installation whose account was removed and remade — is
// adopted, exactly as before.
func TestOnboardingStillAdoptsAWorkspaceThatAlreadyExists(t *testing.T) {
	daemon := &fakeDaemon{workspaces: []map[string]any{
		{"id": "acme", "path": "/home/me/acme"},
	}}
	srv := daemon.server(t)
	client := daemonclient.New(daemonclient.Options{BaseURL: srv.URL})

	opened, err := openWorkspace(context.Background(), client, "", wailsvc.AuthOnboarding)
	if err != nil {
		t.Fatal(err)
	}
	if opened.ID != "acme" || opened.Path != "/home/me/acme" {
		t.Errorf("opened = %+v, want the workspace already registered", opened)
	}
	if daemon.called("workspace_introspect") {
		t.Error("onboarding registered a second workspace over one that existed")
	}
}

// TestLoginStillRegistersTheDirectoryItWasLaunchedIn: signing into an
// installation that already has an account is the other door, and it keeps its
// behaviour — opening the application inside a repository registers that
// repository. There is no wizard in that flow to own the naming.
func TestLoginStillRegistersTheDirectoryItWasLaunchedIn(t *testing.T) {
	daemon := &fakeDaemon{}
	srv := daemon.server(t)
	client := daemonclient.New(daemonclient.Options{BaseURL: srv.URL})

	opened, err := openWorkspace(context.Background(), client, "/home/me/project", wailsvc.AuthLogin)
	if err != nil {
		t.Fatal(err)
	}
	if !daemon.called("workspace_introspect") {
		t.Fatal("login no longer registers the directory the window names")
	}
	if opened.ID == "" {
		t.Error("login adopted nothing")
	}
}

// TestLaunchDirectoryReachesTheInterface: the wizard asks where the first
// workspace should live and offers to pick a folder. Left empty, the workspace
// is created under the state directory — which is right for an application
// opened from the dock, and wrong for one launched inside a repository, where
// that repository is the obvious answer and used to be registered
// automatically. The interface can only offer it as a default if it is told
// what it is.
func TestLaunchDirectoryReachesTheInterface(t *testing.T) {
	svc := wailsvc.NewSystem(nil, nil, "")
	svc.SetLaunchDirectory("/home/me/project")

	got, err := svc.LaunchDirectory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "/home/me/project" {
		t.Errorf("LaunchDirectory = %q, want where the window was launched", got)
	}

	// An application opened from the dock names no directory, and must not
	// invent one: "/" is not a project.
	empty := wailsvc.NewSystem(nil, nil, "")
	if got, err := empty.LaunchDirectory(context.Background()); err != nil || got != "" {
		t.Errorf("LaunchDirectory with no directory = %q, %v — want empty", got, err)
	}
}

// TestAfterAuthKnowsWhichDoorItCameThrough: login and onboarding need
// different behaviour from the hook, so the hook has to be told which one ran.
func TestAfterAuthKnowsWhichDoorItCameThrough(t *testing.T) {
	var seen []wailsvc.AuthEvent
	svc := wailsvc.NewAuth(
		&recordingAuthCaller{},
		func(_ context.Context, event wailsvc.AuthEvent) { seen = append(seen, event) },
	)

	if _, err := svc.Login(context.Background(), "vitor", "whatever"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Onboarding(context.Background(), "Vitor", "v@example.test", "whatever"); err != nil {
		t.Fatal(err)
	}
	want := []wailsvc.AuthEvent{wailsvc.AuthLogin, wailsvc.AuthOnboarding}
	if len(seen) != len(want) || seen[0] != want[0] || seen[1] != want[1] {
		t.Errorf("events = %v, want %v", seen, want)
	}
}

type recordingAuthCaller struct{}

func (recordingAuthCaller) Status(context.Context) (wailsvc.AuthStatus, error) {
	return wailsvc.AuthStatus{}, nil
}
func (recordingAuthCaller) Login(context.Context, string, string) (wailsvc.AuthResult, error) {
	return wailsvc.AuthResult{}, nil
}
func (recordingAuthCaller) Onboarding(context.Context, string, string, string) (wailsvc.AuthResult, error) {
	return wailsvc.AuthResult{}, nil
}
func (recordingAuthCaller) Logout(context.Context) error { return nil }
func (recordingAuthCaller) Session(context.Context) (wailsvc.PublicUser, error) {
	return wailsvc.PublicUser{}, nil
}

var _ = strings.TrimSpace
