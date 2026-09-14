package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails installs a default menu whose View › Reload and Force Reload (Cmd+R,
// Cmd+Shift+R) reload the webview at the URL as it is — after the router has
// stripped `?daemon=` from it. The bundle came back as a browser tab and the
// window stayed broken until the application was restarted. The page's own
// reload keeps the parameters; the menu hands the reload to it.
func TestTheViewMenuReloadsThroughThePage(t *testing.T) {
	reloaded := 0
	view := viewMenu(func() { reloaded++ })

	if view.FindByRole(application.Reload) != nil || view.FindByRole(application.ForceReload) != nil {
		t.Error("the menu still carries Wails' own reload, which drops the window's parameters")
	}
	item := view.FindByLabel("Reload")
	if item == nil {
		t.Fatal("the View menu has no Reload at all")
	}
	if acc := item.GetAccelerator(); !strings.EqualFold(acc, "Cmd+R") && !strings.EqualFold(acc, "Ctrl+R") {
		t.Errorf("Reload accelerator = %q, want Cmd+R where people expect it", item.GetAccelerator())
	}
	// Tests build without the production tag, which is a development build:
	// Wails' own View menu has Open DevTools there, and so should this one.
	if view.FindByRole(application.OpenDevTools) == nil {
		t.Error("a development build's View menu lost Open DevTools")
	}
}

// The menu's Reload used to depend on the page entirely: it sent aos:reload and
// the page's own listener reloaded. A bundle that failed before installing that
// listener — a module error, a white window — left Cmd+R doing nothing, and the
// application had to be restarted. The window now reloads itself, at the URL
// it was opened with, when the page does not say it heard.
func TestAReloadThePageDoesNotAnswerIsDoneByTheWindow(t *testing.T) {
	emitted := make(chan struct{}, 2)
	fellBack := make(chan struct{}, 2)
	reloads := &pageReload{
		emit:     func() { emitted <- struct{}{} },
		fallback: func() { fellBack <- struct{}{} },
		wait:     30 * time.Millisecond,
	}

	reloads.reload()
	select {
	case <-fellBack:
	case <-time.After(2 * time.Second):
		t.Fatal("the page never answered and the window did not reload")
	}
	if len(emitted) != 1 {
		t.Errorf("the page was asked %d times", len(emitted))
	}
}

// A page that answers reloads itself, keeping its route and the window's
// parameters; the window reloading as well would throw the route away.
func TestAReloadThePageAnswersIsLeftToThePage(t *testing.T) {
	fellBack := make(chan struct{}, 2)
	reloads := &pageReload{emit: func() {}, fallback: func() { fellBack <- struct{}{} }, wait: 30 * time.Millisecond}

	reloads.reload()
	reloads.acknowledged()
	time.Sleep(150 * time.Millisecond)
	if len(fellBack) != 0 {
		t.Fatal("the window reloaded over a page that had answered")
	}

	// An answer with nothing pending, and a second Reload after it, behave.
	reloads.acknowledged()
	reloads.reload()
	select {
	case <-fellBack:
	case <-time.After(2 * time.Second):
		t.Fatal("a later Reload the page did not answer was not done by the window")
	}
}

// The menu is macOS's alone. On Linux, Wails makes the application menu the
// GTK menubar of every window that has no menu of its own, so installing one
// there put a File/Edit/View strip above the frameless window's own tab bar.
// Before this application had a menu of its own, App.Run installed Wails'
// default on macOS only, and Linux and Windows had none.
func TestOnlyMacOSGetsAMenuBar(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "freebsd"} {
		if hasMenuBar(goos) {
			t.Errorf("%s gets a menu bar above its frameless window", goos)
		}
	}
	if !hasMenuBar("darwin") {
		t.Error("macOS has no menu bar, so Cmd+C, Cmd+V and Cmd+R do nothing")
	}
}

type recorded struct{ paths []string }

func (r *recorded) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.paths = append(r.paths, req.URL.Path)
	w.WriteHeader(http.StatusOK)
}

// The embedded asset server answers index.html only for "/" and a bare 404 for
// every route the router owns, so reloading on /goals drew a blank page.
func TestAReloadOnARouteGetsTheInterface(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":      {Data: []byte("<html>")},
		"assets/app-1.js": {Data: []byte("js")},
		"icons/logo.svg":  {Data: []byte("<svg>")},
	}
	for _, tc := range []struct {
		path, accept, want string
	}{
		{"/goals", "text/html,application/xhtml+xml", "/"},
		{"/tasks/abc-123", "text/html", "/"},
		{"/assets/app-1.js", "*/*", "/assets/app-1.js"},
		{"/assets/missing.js", "*/*", "/assets/missing.js"},
		{"/api/file/content", "text/html", "/api/file/content"},
		{"/wails/runtime", "text/html", "/wails/runtime"},
		{"/v/artifacts/x/", "text/html", "/v/artifacts/x/"},
		{"/", "text/html", "/"},
	} {
		next := &recorded{}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil)
		req.Header.Set("Accept", tc.accept)
		spaFallback(dist, next).ServeHTTP(httptest.NewRecorder(), req)
		if len(next.paths) != 1 || next.paths[0] != tc.want {
			t.Errorf("%s (accept %s) reached %v, want %s", tc.path, tc.accept, next.paths, tc.want)
		}
	}
}
