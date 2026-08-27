package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/logging"
)

// The headers this transport reads and writes.
const (
	HeaderRequestID = "X-Request-ID"
	HeaderWorkspace = "X-Workspace-ID"
	HeaderAgent     = "X-Agent-ID"
	HeaderAuthToken = "X-Auth-Token"
	cookieSession   = "sessionToken"
	cookieWorkspace = "x-workspace-id"
)

// requestID puts a correlation id on every response, generating one when the
// caller did not bring its own.
//
// It is on every response including the failures, because the failures are
// where somebody has to find the matching log line.
func requestID(gen func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimSpace(r.Header.Get(HeaderRequestID))
			if id == "" {
				id = gen()
			}
			w.Header().Set(HeaderRequestID, id)

			ambient := identity.From(r.Context())
			ambient.RequestID = id
			next.ServeHTTP(w, r.WithContext(identity.With(r.Context(), ambient)))
		})
	}
}

// recoverer turns a panic into a 500 and keeps the daemon serving.
//
// This is defect #16. In the original an unhandled rejection takes the whole
// process down, so one bad request ends every other in flight. Here the blast
// radius is the request that caused it.
func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// A panic after the response started cannot be turned into a
				// 500: the status is already on the wire. Logging it is all
				// that is left, and pretending otherwise would corrupt the body.
				log.Error("panic in handler",
					"panic", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"requestId", w.Header().Get(HeaderRequestID),
					"stack", string(debug.Stack()))
				writeError(w, r, errPanic(w.Header().Get(HeaderRequestID)))
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter remembers what was written, so the access log can report it.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer, which is what
// a streaming handler needs in order to flush.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// accessLog records one line per request, at a level chosen by the status.
//
// The body is never read here. A streaming response would otherwise be consumed
// by its own logger, which is a way to break exactly the endpoints that matter
// most.
func accessLog(log *slog.Logger, clock func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := clock()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}
			log.Log(r.Context(), level, "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", sw.bytes,
				"duration", clock().Sub(start).String(),
				"requestId", sw.Header().Get(HeaderRequestID))
		})
	}
}

// ambientIdentity reads who is calling and what they are addressing.
//
// The token is taken from headers and cookies only. A credential in a query
// string ends up in the access log of every proxy between here and the caller,
// in the browser history, and in the Referer header of the next request — which
// is why the original accepting one is defect #4.
func ambientIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ambient := identity.From(r.Context())

		// A workspace id is not a credential, so the query string is a
		// legitimate source for it — a link that opens one workspace is useful.
		ambient.WorkspaceID = firstNonEmpty(
			r.Header.Get(HeaderWorkspace),
			cookieValue(r, cookieWorkspace),
			r.URL.Query().Get("workspace"),
		)
		ambient.AgentID = strings.ToLower(firstNonEmpty(
			r.Header.Get(HeaderAgent),
			r.URL.Query().Get("agent"),
		))

		ctx := identity.With(r.Context(), ambient)
		if token := bearerOf(r); token != "" {
			ctx = withPresentedToken(ctx, token)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bearerOf extracts the presented credential. Query string is deliberately not
// consulted; see ambientIdentity.
func bearerOf(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := cutPrefixFold(h, "bearer "); ok {
			return strings.TrimSpace(rest)
		}
	}
	if h := strings.TrimSpace(r.Header.Get(HeaderAuthToken)); h != "" {
		return h
	}
	return cookieValue(r, cookieSession)
}

type tokenKey struct{}

func withPresentedToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}

func presentedToken(ctx context.Context) string {
	v, _ := ctx.Value(tokenKey{}).(string)
	return v
}

// authenticate resolves the presented credential to an account and refuses the
// request when there is none.
//
// When security is switched off in the configuration the request proceeds
// unauthenticated — that is the original's behaviour and it is what makes a
// loopback-only installation usable without ceremony. It is also why binding to
// anything other than loopback requires authentication to be on (ADR-0009), a
// rule enforced at boot rather than here.
func authenticate(a Authenticator, enabled func() bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled() {
				next.ServeHTTP(w, r)
				return
			}
			// Security is on and there is nobody to ask. Letting the request
			// through here would mean an unauthenticated surface precisely
			// when the installation asked for an authenticated one — a
			// misconfiguration must fail closed.
			if a == nil {
				writeError(w, r, errNoAuthenticator())
				return
			}
			token := presentedToken(r.Context())
			if token == "" {
				writeError(w, r, errUnauthenticated())
				return
			}
			user, err := a.Authenticate(r.Context(), token)
			if err != nil {
				writeError(w, r, err)
				return
			}
			ambient := identity.From(r.Context())
			ambient.UserID = user.ID
			next.ServeHTTP(w, r.WithContext(identity.With(r.Context(), ambient)))
		})
	}
}

// requireAuth guards a route that must never be open, whatever the
// configuration says. The API playground is the reason it exists: the original
// serves it with `security: () => true` (defect #3).
func requireAuth(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a == nil {
				writeError(w, r, errUnauthenticated())
				return
			}
			user, err := a.Authenticate(r.Context(), presentedToken(r.Context()))
			if err != nil {
				writeError(w, r, err)
				return
			}
			ambient := identity.From(r.Context())
			ambient.UserID = user.ID
			next.ServeHTTP(w, r.WithContext(identity.With(r.Context(), ambient)))
		})
	}
}

// corsPolicy answers preflights for the origins that were declared, and only
// those.
//
// There is no wildcard and no reflection of the requesting origin. The daemon
// listens on the machine the browser is running on, so a permissive policy
// means any page the person opens can talk to it.
func corsPolicy(allowed []string) func(http.Handler) http.Handler {
	index := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		index[strings.ToLower(strings.TrimRight(o, "/"))] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.ToLower(strings.TrimRight(r.Header.Get("Origin"), "/"))
			if origin != "" && index[origin] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Headers",
					strings.Join([]string{"Authorization", "Content-Type", HeaderWorkspace, HeaderAgent, HeaderAuthToken, HeaderRequestID}, ", "))
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				h.Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				// An unlisted origin gets the same empty answer as a listed one
				// gets a permissive one: the browser decides, and it decides on
				// the absence of the header.
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

// loggerFrom prefers the logger the context carries, so a line written here
// correlates with the rest of the request.
func loggerFrom(r *http.Request, fallback *slog.Logger) *slog.Logger {
	if l := logging.FromContext(r.Context()); l != nil {
		return l
	}
	return fallback
}
