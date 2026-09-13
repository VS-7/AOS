package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/aos/internal/transport/daemonclient"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// registryDaemon answers workspace_list and workspace_get over a fixed set.
func registryDaemon(t *testing.T, workspaces ...registeredWorkspace) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/workspace/list", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"workspaces": workspaces})
	})
	mux.HandleFunc("/api/workspace/get", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Workspace string `json:"workspace"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		for _, ws := range workspaces {
			if ws.ID == in.Workspace {
				writeData(w, ws)
				return
			}
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"code": "AOS_WORKSPACE_NOT_FOUND", "message": "no workspace " + in.Workspace,
		}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// The user has workspaces aos < new-project < vs and works in vs. After the
// daemon restarted, or after signing in again without a reload, the window
// re-adopted the first workspace by id: the switcher still read "VS" while
// every read and write, and the event relay, went to "aos". The interface
// never said its choice again, because it had already said it once.
func TestReopeningKeepsTheWorkspaceTheInterfaceChose(t *testing.T) {
	srv := registryDaemon(t,
		registeredWorkspace{ID: "aos", Path: "/w/aos"},
		registeredWorkspace{ID: "vs", Path: "/w/vs"},
	)
	client := daemonclient.New(daemonclient.Options{BaseURL: srv.URL})
	choice := &chosenWorkspace{}
	choice.Set("vs")

	for _, event := range []wailsvc.AuthEvent{wailsvc.AuthLogin, wailsvc.AuthOnboarding} {
		opened, err := openWorkspace(context.Background(), client, "", choice.Get(), event)
		if err != nil {
			t.Fatal(err)
		}
		if opened.ID != "vs" || opened.Path != "/w/vs" {
			t.Errorf("%s: opened %+v, want the workspace the interface chose", event, opened)
		}
	}
}

// A choice is a preference, not a promise: the workspace may have been
// archived or deleted from another window since.
func TestAChosenWorkspaceThatIsGoneFallsBackToTheRegistry(t *testing.T) {
	srv := registryDaemon(t,
		registeredWorkspace{ID: "aos", Path: "/w/aos"},
		registeredWorkspace{ID: "old", Path: "/w/old", Archived: true},
	)
	client := daemonclient.New(daemonclient.Options{BaseURL: srv.URL})

	for _, preferred := range []string{"deleted", "old"} {
		opened, err := openWorkspace(context.Background(), client, "", preferred, wailsvc.AuthLogin)
		if err != nil {
			t.Fatal(err)
		}
		if opened.ID != "aos" {
			t.Errorf("preferred %q: opened %+v, want the first usable workspace", preferred, opened)
		}
	}
}
