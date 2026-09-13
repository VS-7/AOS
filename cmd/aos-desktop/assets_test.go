package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; frame-ancestors 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// What the daemon sends: artifactapi a same-origin resource policy,
		// fileapi none. The window sets its own on what it forwards.
		if strings.HasPrefix(r.URL.Path, "/v/artifacts/") {
			w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<h1>sales</h1>")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serveThrough(t *testing.T, daemon *daemonclient.Client, req *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	return serveWith(bridgeDaemon(daemon, slog.New(slog.NewTextHandler(io.Discard, nil))), req)
}

func serveWith(middleware func(http.Handler) http.Handler, req *http.Request) (*httptest.ResponseRecorder, bool) {
	reachedAssets := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reachedAssets = true })
	rec := httptest.NewRecorder()
	middleware(next).ServeHTTP(rec, req)
	return rec, reachedAssets
}

// frameAddress asks the window where to frame target, the way the interface
// does (lib/wails.ts frameAddress).
func frameAddress(t *testing.T, middleware func(http.Handler) http.Handler, target string) string {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		frameAddressRoute+"?url="+url.QueryEscape(target)+"&base="+url.QueryEscape("wails://localhost"), nil)
	req.Header.Set(frameAddressHeader, "1")
	rec, reached := serveWith(middleware, req)
	if reached || rec.Code != http.StatusOK {
		t.Fatalf("frame address for %s: status %d, reached assets %v, body %q", target, rec.Code, reached, rec.Body.String())
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("frame address for %s: %v in %q", target, err, rec.Body.String())
	}
	return out.URL
}

// Opening an artifact loads its relative URL, /v/artifacts/{id}/, in the
// window's own origin — and the asset host had no route for it, so the tab
// showed a blank 404. The iframe cannot carry a bearer; this process can.
//
// The frame is sandboxed into an opaque origin, so everything the artifact
// loads of its own is a cross-origin request, and passing on the daemon's
// same-origin resource policy cancelled every stylesheet, script and image.
// The window frames it at an address only this window hands out, and that
// address alone answers cross-origin.
func TestAnOpenedArtifactIsServedWithTheWindowsCredential(t *testing.T) {
	var got seen
	srv := proxyDaemon(t, &got)
	daemon := daemonclient.New(daemonclient.Options{BaseURL: srv.URL, Token: "window-token", Workspace: "vs"})
	proxy := bridgeDaemon(daemon, slog.New(slog.NewTextHandler(io.Discard, nil)))

	opened := frameAddress(t, proxy, "/v/artifacts/sales/")
	if !strings.HasPrefix(opened, artifactFrameRoute) || !strings.HasSuffix(opened, "/sales/") {
		t.Fatalf("frame address = %q", opened)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, opened+"css/style.css?password=p", nil)
	rec, reachedAssets := serveWith(proxy, req)

	if reachedAssets {
		t.Fatal("the artifact request fell through to the interface's own assets")
	}
	if got.path != "/v/artifacts/sales/css/style.css" || got.query != "password=p" {
		t.Errorf("daemon saw %s?%s", got.path, got.query)
	}
	if got.auth != "Bearer window-token" || got.workspace != "vs" {
		t.Errorf("daemon saw authorization %q, workspace %q", got.auth, got.workspace)
	}
	if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("the artifact's own security headers were dropped: %v", rec.Header())
	}
	if corp := rec.Header().Get("Cross-Origin-Resource-Policy"); corp != "cross-origin" {
		t.Errorf("an opened artifact's file answers Cross-Origin-Resource-Policy %q; its own opaque page cannot load it", corp)
	}
	// fetch("data.json") from the artifact's own opaque page is a CORS request.
	if acao := rec.Header().Get("Access-Control-Allow-Origin"); acao != "*" {
		t.Errorf("an opened artifact's file answers Access-Control-Allow-Origin %q; its own page cannot fetch it", acao)
	}
	// And WebKit does not count the artifact's own URL as connect-src 'self'
	// once the page is opaque: the fetch was refused before it was sent. The
	// window names the artifact's own address, and nothing wider.
	wantCSP := "default-src 'self'; connect-src 'self' wails://localhost" + opened + "; frame-ancestors 'self'"
	if csp := rec.Header().Get("Content-Security-Policy"); csp != wantCSP {
		t.Errorf("CSP = %q, want %q", csp, wantCSP)
	}
	if !strings.Contains(rec.Body.String(), "sales") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// What this window forwards carries its credential, so anything else that can
// make the webview request it — a page from another site open in a browser
// tab, or another artifact — must not be able to embed it as a script or an
// image. An artifact's own address is issued per artifact, by this process,
// to the page that asks with a header only a same-origin request can carry.
func TestOnlyTheWindowCanOpenAnArtifactAndOnlyThatOne(t *testing.T) {
	var got seen
	srv := proxyDaemon(t, &got)
	daemon := daemonclient.New(daemonclient.Options{BaseURL: srv.URL, Token: "window-token", Workspace: "vs"})
	proxy := bridgeDaemon(daemon, slog.New(slog.NewTextHandler(io.Discard, nil)))

	sales := frameAddress(t, proxy, "/v/artifacts/sales/")
	grant := strings.Split(strings.TrimPrefix(sales, artifactFrameRoute), "/")[0]
	other := frameAddress(t, bridgeDaemon(daemon, slog.New(slog.NewTextHandler(io.Discard, nil))), "/v/artifacts/sales/")

	for _, path := range []string{
		artifactFrameRoute + grant + "/payroll/data.js", // another artifact, same grant
		artifactFrameRoute + "forged/sales/data.js",     // no grant at all
		other + "data.js", // another window's grant
		artifactFrameRoute + grant + "/%2e%2e/payroll/x.js", // a segment that is not an id
		artifactFrameRoute + grant,
	} {
		got = seen{}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		rec, reached := serveWith(proxy, req)
		if reached || rec.Code != http.StatusNotFound || got.path != "" {
			t.Errorf("%s: status %d, reached assets %v, daemon saw %q", path, rec.Code, reached, got.path)
		}
	}

	// Asked without the header: a no-cors request cannot set it, and a CORS
	// request that asks to is preflighted and refused.
	bare := httptest.NewRequestWithContext(t.Context(), http.MethodGet, frameAddressRoute+"?url=%2Fv%2Fartifacts%2Fsales%2F", nil)
	if rec, reached := serveWith(proxy, bare); reached || rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), grant) {
		t.Errorf("a frame address was handed out without the header: status %d, body %q", rec.Code, rec.Body.String())
	}
	// A base that could smuggle a directive, or name something else than a
	// scheme and a host, widens nothing.
	for _, base := range []string{"http://a;script-src *", "javascript:alert(1)", "http://x/ y", "http://x/path", ""} {
		fresh := bridgeDaemon(daemon, slog.New(slog.NewTextHandler(io.Discard, nil)))
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			frameAddressRoute+"?url=%2Fv%2Fartifacts%2Fsales%2F&base="+url.QueryEscape(base), nil)
		req.Header.Set(frameAddressHeader, "1")
		rec, _ := serveWith(fresh, req)
		var out struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		file := httptest.NewRequestWithContext(t.Context(), http.MethodGet, out.URL+"data.json", nil)
		served, _ := serveWith(fresh, file)
		if csp := served.Header().Get("Content-Security-Policy"); csp != "default-src 'self'; connect-src 'self'; frame-ancestors 'self'" {
			t.Errorf("base %q changed the CSP to %q", base, csp)
		}
	}

	for _, target := range []string{"/api/file/content?path=a", "http://elsewhere/v/artifacts/sales/", "/v/artifacts/", "/v/artifacts/../x/"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, frameAddressRoute+"?url="+url.QueryEscape(target), nil)
		req.Header.Set(frameAddressHeader, "1")
		if rec, reached := serveWith(proxy, req); reached || rec.Code != http.StatusNotFound {
			t.Errorf("a frame address for %q: status %d, body %q", target, rec.Code, rec.Body.String())
		}
	}
}

// Without an opened address, what the window forwards answers the window's own
// origin only: the Files panel's <img> and <video> are same-origin and load,
// and nothing else embeds a workspace file with this window's credential.
func TestWhatTheWindowForwardsOtherwiseAnswersOnlyTheWindow(t *testing.T) {
	var got seen
	srv := proxyDaemon(t, &got)
	daemon := daemonclient.New(daemonclient.Options{BaseURL: srv.URL, Token: "window-token", Workspace: "vs"})

	for _, path := range []string{"/api/file/content?path=secrets.js", "/v/artifacts/sales/data.js"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		rec, reached := serveThrough(t, daemon, req)
		if reached || got.auth != "Bearer window-token" {
			t.Fatalf("%s was not forwarded with the window's credential", path)
		}
		if corp := rec.Header().Get("Cross-Origin-Resource-Policy"); corp != "same-origin" {
			t.Errorf("%s answers Cross-Origin-Resource-Policy %q, want same-origin", path, corp)
		}
		if acao := rec.Header().Get("Access-Control-Allow-Origin"); acao != "" {
			t.Errorf("%s answers Access-Control-Allow-Origin %q, want none", path, acao)
		}
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
	for _, path := range []string{"/", "/goals", "/assets/app.js", "/api/tasks/list", "/v/other", "/v/frames/x/y/"} {
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
