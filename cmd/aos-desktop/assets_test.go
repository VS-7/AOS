package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/transport/daemonclient"
)

type seen struct {
	path, query, auth, workspace string
}

func proxyDaemon(t *testing.T, got *seen) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = seen{r.URL.Path, r.URL.RawQuery, r.Header.Get("authorization"), r.Header.Get("x-workspace-id")}
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<h1>sales</h1>")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serveThrough(t *testing.T, daemon *daemonclient.Client, req *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	reachedAssets := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reachedAssets = true })
	rec := httptest.NewRecorder()
	bridgeDaemon(daemon, slog.New(slog.NewTextHandler(io.Discard, nil)))(next).ServeHTTP(rec, req)
	return rec, reachedAssets
}

// Opening an artifact loads its relative URL, /v/artifacts/{id}/, in the
// window's own origin — and the asset host had no route for it, so the tab
// showed a blank 404. The iframe cannot carry a bearer; this process can.
func TestAnArtifactIsServedThroughTheWindowWithItsCredential(t *testing.T) {
	var got seen
	srv := proxyDaemon(t, &got)
	daemon := daemonclient.New(daemonclient.Options{BaseURL: srv.URL, Token: "window-token", Workspace: "vs"})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v/artifacts/sales/style.css?password=p", nil)
	rec, reachedAssets := serveThrough(t, daemon, req)

	if reachedAssets {
		t.Fatal("the artifact request fell through to the interface's own assets")
	}
	if got.path != "/v/artifacts/sales/style.css" || got.query != "password=p" {
		t.Errorf("daemon saw %s?%s", got.path, got.query)
	}
	if got.auth != "Bearer window-token" || got.workspace != "vs" {
		t.Errorf("daemon saw authorization %q, workspace %q", got.auth, got.workspace)
	}
	if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("the artifact's own security headers were dropped: %v", rec.Header())
	}
	if !strings.Contains(rec.Body.String(), "sales") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// Wails' HTTP transport also runs a call sent as a GET, with object, method
// and args in the query string (a fallback for WebKitGTK, which can deliver a
// POST body that way). So `<img src="/wails/runtime?object=0&method=0&args=…">`
// in an artifact — no script, no custom header, no preflight, and allowed by
// the artifact's own img-src 'self' — ran DomainService.Invoke with the
// window's credential. The runtime sends its header with every call, whatever
// the method, so its absence is refused whatever the method.
func TestTheBridgeRefusesACallWithoutTheRuntimesHeaderWhateverTheMethod(t *testing.T) {
	daemon := daemonclient.New(daemonclient.Options{BaseURL: "http://127.0.0.1:1"})
	call := "/wails/runtime?object=0&method=0&args=" +
		`%7B%22call-id%22%3A%22x%22%2C%22methodName%22%3A%22wailsvc.DomainService.Invoke%22%7D`

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodOptions} {
		forged := httptest.NewRequestWithContext(t.Context(), method, call, nil)
		rec, reached := serveThrough(t, daemon, forged)
		if reached || rec.Code != http.StatusForbidden {
			t.Errorf("%s %s without the runtime's header got through (status %d)", method, call, rec.Code)
		}
	}

	genuine := httptest.NewRequestWithContext(t.Context(), http.MethodGet, call, nil)
	genuine.Header.Set("x-wails-client-id", "abc")
	if _, reached := serveThrough(t, daemon, genuine); !reached {
		t.Error("a call carrying the runtime's header was refused")
	}
}

func TestOnlyTheForwardedPathsLeaveTheWindow(t *testing.T) {
	daemon := daemonclient.New(daemonclient.Options{BaseURL: "http://127.0.0.1:1"})
	for _, path := range []string{"/", "/goals", "/assets/app.js", "/api/tasks/list", "/v/other"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if _, reached := serveThrough(t, daemon, req); !reached {
			t.Errorf("%s did not reach the interface's own assets", path)
		}
	}
	post := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v/artifacts/x/", nil)
	if _, reached := serveThrough(t, daemon, post); !reached {
		t.Error("a POST to an artifact was forwarded; only GET is")
	}
}
