// Package httpapi exposes the command registry over HTTP.
//
// There is no business rule here. Every route is derived from the registry, so
// publishing a capability publishes its route — the original maintains 26
// controllers by hand, and the drift between them and the tools is the reason
// this layer generates instead.
package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/auth"
)

// maxBodyBytes bounds a request body. A command payload is a small JSON object;
// anything larger is either a mistake or an attempt to exhaust memory, and both
// are better answered with a 413 than with an allocation.
const maxBodyBytes = 4 << 20 // 4 MiB

// Authenticator is the slice of the identity service this transport needs: turn
// a bearer credential into an account, or refuse.
type Authenticator interface {
	Authenticate(ctx context.Context, bearer string) (*auth.User, error)
}

// Config is what the server is built from.
type Config struct {
	Registry *command.Registry

	// Auth resolves credentials. Nil means no authentication is possible, which
	// is only legitimate while security is switched off.
	Auth Authenticator

	// SecurityEnabled is read per request rather than captured, because the
	// configuration file is meant to be edited with the daemon running.
	SecurityEnabled func() bool

	// DocsEnabled exposes the schema browser. It is authenticated regardless,
	// and off in production by default (defect #3, defect #12).
	DocsEnabled bool

	// AllowedOrigins is the CORS allowlist. Empty means no browser origin is
	// allowed, which is the right default for a daemon on loopback.
	AllowedOrigins []string

	// Realtime is the server-push channel, mounted at /ws. It is a plain
	// handler rather than a typed dependency because the upgrade does its own
	// authorisation — the workspace it attaches to is not the one this layer
	// authenticated, and conflating them is defect #5.
	Realtime http.Handler

	// MCP is the tool surface over HTTP, mounted at /mcp. Nil leaves it
	// unmounted, which is the state while the only transport is stdio.
	MCP http.Handler

	Log   *slog.Logger
	Now   func() time.Time
	NewID func() string
}

// Server is the HTTP surface.
type Server struct {
	router chi.Router
	cfg    Config
}

// New builds the router.
func New(cfg Config) *Server {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Now == nil {
		// Through clockx, which is the one package allowed to read the wall
		// clock — everything else takes it injected so a golden stays stable.
		cfg.Now = clockx.System{}.Now
	}
	if cfg.NewID == nil {
		cfg.NewID = ids.UUID{}.New
	}
	if cfg.SecurityEnabled == nil {
		cfg.SecurityEnabled = func() bool { return true }
	}

	r := chi.NewRouter()
	r.Use(requestID(cfg.NewID))
	r.Use(recoverer(cfg.Log))
	r.Use(accessLog(cfg.Log, cfg.Now))
	r.Use(corsPolicy(cfg.AllowedOrigins))
	r.Use(ambientIdentity)

	s := &Server{router: r, cfg: cfg}

	r.Route("/api", func(api chi.Router) {
		// Health is outside authentication on purpose: the gateway polls it to
		// decide whether the daemon it just started is actually serving, and it
		// has no credential at that moment.
		api.Get("/health", s.health)

		api.Group(func(guarded chi.Router) {
			guarded.Use(authenticate(cfg.Auth, cfg.SecurityEnabled))
			s.mountCommands(guarded)
		})

		if cfg.DocsEnabled {
			api.With(requireAuth(cfg.Auth)).Get("/docs", s.docs)
		}
	})

	if cfg.Realtime != nil {
		// Outside /api because it is not one: it takes no payload, returns no
		// envelope, and authorises itself.
		r.Handle("/ws", cfg.Realtime)
	}
	if cfg.MCP != nil {
		r.Mount("/mcp", cfg.MCP)
	}

	r.NotFound(s.notFound)
	r.MethodNotAllowed(s.methodNotAllowed)
	return s
}

// Handler returns the router.
func (s *Server) Handler() http.Handler { return s.router }

// Routes reports every command route, in registry order. It exists so a test
// can assert that the surface and the registry agree without walking chi.
func (s *Server) Routes() []string {
	out := make([]string, 0, len(s.cfg.Registry.Sorted()))
	for _, d := range s.cfg.Registry.Sorted() {
		out = append(out, RouteOf(d))
	}
	return out
}

// RouteOf is the path a command is published at.
//
// Group and name, not the flat tool key: a path is read by people and by proxy
// logs, and `/api/memories/store` says what `/api/memories_store` does not.
func RouteOf(d command.Descriptor) string {
	return "/api/" + d.Group() + "/" + d.Name()
}

// mountCommands turns the registry into routes.
//
// This lives here rather than in internal/core/command, which the note's sketch
// suggests: the Command Layer would then know chi, and the whole point of the
// layer is that it knows no transport.
func (s *Server) mountCommands(r chi.Router) {
	for _, d := range s.cfg.Registry.Sorted() {
		descriptor := d
		// POST for everything, including reads. A read here still carries a
		// JSON body — the same payload every other surface sends — and a GET
		// with a body is a thing proxies are entitled to discard.
		r.Post("/"+descriptor.Group()+"/"+descriptor.Name(), s.invoke(descriptor, nil))
	}
	// A renamed command keeps answering at its old path, with the notice that
	// says where it moved to. The consumer of these paths is an integration
	// written months ago; breaking it silently is the failure ADR-0011 exists
	// to prevent, and it has to fail the same way on every surface.
	for old, alias := range s.cfg.Registry.Aliases() {
		d, _, ok := s.cfg.Registry.Lookup(old)
		if !ok {
			continue
		}
		group, name, found := strings.Cut(old, "_")
		if !found {
			continue
		}
		r.Post("/"+group+"/"+name, s.invoke(d, alias.Notice()))
	}
}

func (s *Server) invoke(d command.Descriptor, notice *command.DeprecationNotice) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeError(w, r, errBodyTooLarge(maxBodyBytes))
			return
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			body = []byte("{}")
		}

		out, err := d.Invoke(r.Context(), command.SurfaceHTTP, body)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, command.Wrap(out, notice))
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": build.Version,
		"name":    build.Name,
	})
}

// docs publishes the schema of every command: what an integrator reads, and
// what a model reads when it is discovering the surface.
func (s *Server) docs(w http.ResponseWriter, _ *http.Request) {
	type entry struct {
		Tool    string `json:"tool"`
		Route   string `json:"route"`
		Summary string `json:"summary"`
		Schema  any    `json:"schema"`
	}
	out := make([]entry, 0, len(s.cfg.Registry.Sorted()))
	for _, d := range s.cfg.Registry.Sorted() {
		out = append(out, entry{
			Tool: d.Key(), Route: RouteOf(d), Summary: d.Summary(), Schema: d.InputSchema(),
		})
	}
	writeJSON(w, http.StatusOK, command.Wrap(out, nil))
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, errNoSuchRoute(r.Method, r.URL.Path))
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, errWrongMethod(r.Method, r.URL.Path))
}

// writeJSON renders a payload. It sets the content type before the status,
// because the header map is frozen once the status is written.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

// writeError renders a failure in the same envelope shape as a success, so a
// caller parses one structure rather than two.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	e, ok := apperr.As(err)
	if !ok {
		// An error that escaped the domain without being classified is an
		// internal failure by definition, and its message is not for a caller.
		e = apperr.New("HTTP_INTERNAL").
			Causer("httpapi").
			Msgf("the request could not be completed").
			Status(apperr.StatusInternalServerError).
			Wrap(err)
		loggerFrom(r, slog.Default()).Error("unclassified error escaped a handler",
			"err", err, "path", r.URL.Path, "requestId", w.Header().Get(HeaderRequestID))
	}
	writeJSON(w, e.HTTPStatus, map[string]any{"error": e})
}
