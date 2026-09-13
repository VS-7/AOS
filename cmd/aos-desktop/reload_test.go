package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

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
