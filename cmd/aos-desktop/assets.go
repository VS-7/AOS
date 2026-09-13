package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/transport/daemonclient"
)

// The daemon paths the window serves through its own asset host rather than
// over the bridge: things an element puts in a `src`, which can carry no bearer
// and cannot read a JSON string.
const (
	contentRoute   = "/api/file/content"
	artifactsRoute = "/v/artifacts/"
)

// runtimeRoute is where @wailsio/runtime sends every bridge call.
const runtimeRoute = "/wails/runtime"

// bridgeDaemon is the asset host's middleware: it forwards the two daemon
// paths an element loads by URL, with the window's credential attached, and
// refuses a bridge call the runtime did not make.
//
// The file content route exists because of what an `<img>` is. The Files
// panel's image, PDF and video viewers need a URL they can put in a `src`, and
// inside the desktop window neither of the two paths the interface normally
// uses can produce one: the Wails bridge answers a JSON string
// (`DomainService.Fetch`), which is not a video, and a plain URL to the daemon
// is cross-origin from `wails://localhost`, carries no bearer and no cookie,
// and is refused.
//
// The artifact route is the same problem in an `<iframe>`. Opening an artifact
// loads its relative URL (`/v/artifacts/{id}/`, as the daemon publishes it) in
// the window's own origin, where nothing answered it: every artifact opened
// from Home or Surfaces was a blank 404. The daemon also sends the artifact's
// own security headers — its CSP above all — and they are passed back as they
// came.
//
// A relative URL now works in both modes. In a browser the daemon serves the
// page and the request is same-origin. In the window it reaches this
// middleware, which forwards it with the token the process already holds — the
// credential never enters the page — and passes Range and the conditional
// headers through in both directions, so a video still seeks and a cached image
// still 304s.
//
// Only these paths, and only GET. The asset host is otherwise the interface's
// own bundle, and a proxy that forwarded anything would be a hole in the
// boundary `DomainService.Fetch`'s own allowlist exists to keep.
func bridgeDaemon(daemon *daemonclient.Client, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if refusedRuntimeCall(r) {
				http.Error(w, "not a call from this window's runtime", http.StatusForbidden)
				return
			}
			if r.Method != http.MethodGet || !forwarded(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			forward(w, r, daemon, log)
		})
	}
}

func forwarded(path string) bool {
	return strings.HasPrefix(path, contentRoute) || strings.HasPrefix(path, artifactsRoute)
}

// refusedRuntimeCall is a request to the bridge, of any method, without the
// header every call @wailsio/runtime makes carries.
//
// An artifact is HTML a model generated, and it is served from this window's
// own origin. Its document is sandboxed without allow-same-origin, so a script
// in it cannot reach the page or make a same-origin request — but plenty
// reaches the bridge without permission. A no-cors "simple" POST is sent
// anyway, and Wails processes its body without looking at the content type.
// A GET is enough, too: Wails' HTTP transport reads object, method and args
// from the query string when there is no body (its WebKitGTK fallback), so an
// `<img>`, a `<form method=get>` or a navigation runs DomainService.Invoke with
// the window's credential, without a line of script. Guarding POST alone left
// exactly that open.
//
// None of those can set a custom header: a no-cors request drops it, and a
// cross-origin request that asks for one is preflighted first — and the
// preflight, which carries no such header either, is refused here. The runtime
// sets this one on every call, chunked or not, so its absence is the tell.
func refusedRuntimeCall(r *http.Request) bool {
	return r.URL.Path == runtimeRoute &&
		strings.TrimSpace(r.Header.Get("x-wails-client-id")) == ""
}

// responseHeaders are the daemon's headers a forwarded answer keeps: what a
// player needs to seek and a cache needs to revalidate, and the security
// headers the artifact route sends with every file.
var responseHeaders = []string{
	"content-type", "content-length", "content-range",
	"accept-ranges", "last-modified", "etag", "cache-control",
	"content-security-policy", "x-content-type-options", "referrer-policy",
	"cross-origin-resource-policy", "content-disposition",
}

func forward(w http.ResponseWriter, r *http.Request, daemon *daemonclient.Client, log *slog.Logger) {
	path := r.URL.EscapedPath()
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	res, err := daemon.Stream(r.Context(), path, r.Header)
	if err != nil {
		log.Warn("a file could not be read from the daemon", "path", r.URL.Path, "err", err)
		http.Error(w, "the daemon did not answer", http.StatusBadGateway)
		return
	}
	defer func() { _ = res.Body.Close() }()

	for _, name := range responseHeaders {
		if v := res.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	if _, err := io.Copy(w, res.Body); err != nil {
		// The viewer navigated away mid-stream, which is ordinary.
		log.Debug("a file stream ended early", "err", err)
	}
}
