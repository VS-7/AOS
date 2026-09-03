package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/transport/daemonclient"
)

// contentRoute is the one daemon path the window serves through its own asset
// host rather than over the bridge.
const contentRoute = "/api/file/content"

// bridgeContent proxies GET /api/file/content to the daemon, with the
// window's credential attached.
//
// It exists because of what an `<img>` is. The Files panel's image, PDF and
// video viewers need a URL they can put in a `src`, and inside the desktop
// window neither of the two paths the interface normally uses can produce
// one: the Wails bridge answers a JSON string (`DomainService.Fetch`), which
// is not a video, and a plain URL to the daemon is cross-origin from
// `wails://localhost`, carries no bearer and no cookie, and is refused. The
// viewers were pointed at a relative `/api/files/content` — a path that does
// not exist even in the plural — so every picture, PDF and video in the
// panel failed to load, in both modes, for different reasons.
//
// A relative URL now works in both. In a browser the daemon serves the page,
// so `/api/file/content` is same-origin and the session cookie goes with it.
// In the window it reaches this middleware, which forwards it with the token
// the process already holds — the credential never enters the page — and
// passes Range and the conditional headers through in both directions, so a
// video still seeks and a cached image still 304s.
//
// Only this one path, and only GET. The asset host is otherwise the
// interface's own bundle, and a proxy that forwarded anything would be a hole
// in the boundary `DomainService.Fetch`'s own allowlist exists to keep.
func bridgeContent(daemon *daemonclient.Client, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, contentRoute) {
				next.ServeHTTP(w, r)
				return
			}

			path := contentRoute
			if r.URL.RawQuery != "" {
				path += "?" + r.URL.RawQuery
			}
			res, err := daemon.Stream(r.Context(), path, r.Header)
			if err != nil {
				log.Warn("a file could not be read from the daemon", "path", r.URL.RawQuery, "err", err)
				http.Error(w, "the daemon did not answer", http.StatusBadGateway)
				return
			}
			defer func() { _ = res.Body.Close() }()

			// The daemon's own headers, so the player learns the length, the
			// type, and that ranges are available.
			for _, name := range []string{
				"content-type", "content-length", "content-range",
				"accept-ranges", "last-modified", "etag",
			} {
				if v := res.Header.Get(name); v != "" {
					w.Header().Set(name, v)
				}
			}
			w.WriteHeader(res.StatusCode)
			if _, err := io.Copy(w, res.Body); err != nil {
				// The viewer navigated away mid-stream, which is ordinary.
				log.Debug("a file stream ended early", "err", err)
			}
		})
	}
}
