// Package artifactapi serves one artifact's own files at
// /v/artifacts/{id}/* — see docs/05 - Transporte/Artifacts e Estáticos.md
// and docs/04 - Domínio/Artifact (Go).md.
//
// An artifact is HTML an LLM generated, hosted on this daemon's own origin —
// stricter rules apply here than to the app's own assets. It carries no
// session cookie: unlike every route inside httpapi's guarded /api group,
// which accepts a cookie as a fallback credential (see httpapi's own
// bearerOf), this one reads only a deliberately presented credential — an
// Authorization header, or, for a by_password artifact, the password
// itself — never whatever the browser happened to attach automatically.
package artifactapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/auth"
)

// Artifacts is the slice of artifact.Service this router needs: look one up,
// and decide whether a request may read it.
type Artifacts interface {
	Get(ctx context.Context, in artifact.GetInput) (*artifact.Artifact, error)
	Authorize(ctx context.Context, a *artifact.Artifact, req artifact.AccessRequest) error
}

// Files resolves a path inside one artifact's own directory — see
// internal/adapters/artifactfiles.Files.Resolve, the concrete implementation
// wired in production.
type Files interface {
	Resolve(id, path string) (string, error)
}

// Authenticator is the same narrow port httpapi.Authenticator declares —
// duplicated rather than imported so this package does not depend on
// httpapi for a two-method interface either side already satisfies.
type Authenticator interface {
	Authenticate(ctx context.Context, bearer string) (*auth.User, error)
}

// Config is what the router is built from.
type Config struct {
	Artifacts Artifacts
	Files     Files

	// Auth resolves a presented bearer credential. Nil means every request is
	// treated as unauthenticated for Private/Workspace visibility — by_password
	// artifacts are unaffected, since they never consult this at all.
	Auth Authenticator

	// SecurityEnabled mirrors httpapi.Config's own field: read per request,
	// since the configuration file is meant to be edited with the daemon
	// running. Nil is treated as always enabled — the safe default.
	SecurityEnabled func() bool

	Log *slog.Logger
}

// New builds the router. It is mounted by the caller at /v — see httpapi's
// Artifacts field — outside the authenticated /api group: this route
// authorises itself, per-artifact, rather than gating the whole mount on one
// credential check.
func New(cfg Config) http.Handler {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.SecurityEnabled == nil {
		cfg.SecurityEnabled = func() bool { return true }
	}
	s := &server{cfg: cfg}

	r := chi.NewRouter()
	r.Get("/artifacts/{id}/*", s.serve)
	return r
}

type server struct{ cfg Config }

func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	a, err := s.cfg.Artifacts.Get(r.Context(), artifact.GetInput{ID: id})
	if err != nil {
		writeError(w, s.cfg.Log, err)
		return
	}

	req := artifact.AccessRequest{
		Authenticated: s.authenticated(r),
		Password:      r.URL.Query().Get("password"),
	}
	if err := s.cfg.Artifacts.Authorize(r.Context(), a, req); err != nil {
		writeError(w, s.cfg.Log, err)
		return
	}

	rel := chi.URLParam(r, "*")
	if rel == "" {
		rel = a.Entrypoint
	}
	target, err := s.cfg.Files.Resolve(id, rel)
	if err != nil {
		// A path outside the artifact's own directory reads the same as one
		// that does not exist — naming the reason would confirm to a prober
		// that traversal was attempted rather than simply refused.
		http.NotFound(w, r)
		return
	}
	info, statErr := os.Stat(target)
	if statErr != nil || info.IsDir() {
		// A directory is refused rather than listed or index.html-resolved:
		// http.ServeFile would do either, and an artifact's own file layout
		// is not something this route discloses.
		http.NotFound(w, r)
		return
	}

	setArtifactHeaders(w, target)
	http.ServeFile(w, r, target)
}

// authenticated reports whether r carries a credential this daemon accepts —
// never a session cookie, per this package's own doc comment. Security
// switched off is treated the same way httpapi's own authenticate
// middleware treats it: every request proceeds, matching the loopback-only,
// no-ceremony default the rest of the API already applies.
func (s *server) authenticated(r *http.Request) bool {
	if s.cfg.Auth == nil || !s.cfg.SecurityEnabled() {
		return true
	}
	token := bearerHeader(r)
	if token == "" {
		return false
	}
	_, err := s.cfg.Auth.Authenticate(r.Context(), token)
	return err == nil
}

// bearerHeader reads a presented credential from a header only. The query
// string is not consulted for it, the same rule httpapi's own bearerOf
// applies to the system's session bearer — a token in a URL ends up in
// server logs and browser history. The artifact's own password, a different
// and deliberately shareable secret, is the one credential this route does
// read from the query string — see serve's own AccessRequest construction.
func bearerHeader(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		const prefix = "bearer "
		if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
			return strings.TrimSpace(h[len(prefix):])
		}
	}
	return strings.TrimSpace(r.Header.Get("X-Auth-Token"))
}

// csp is the fixed, restrictive policy every artifact is served under: no
// external network, no inline script, no framing by a third-party page. An
// artifact is generated content served from this daemon's own origin, and
// without this a script inside one could call this system's own /api/*
// endpoints or exfiltrate data to an attacker-controlled host. The design
// doc's own sketch anticipates a per-artifact opt-in to relax this; no such
// opt-in field exists on the entity yet, so every artifact gets the strict
// policy today — a smaller, disclosed limitation, not a silent one.
const csp = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; font-src 'self' data:; connect-src 'self'; " +
	"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"

// setArtifactHeaders applies the policy that keeps generated content from
// reaching back into the workspace it was generated in, and from being
// mistaken for something this daemon vouches for.
func setArtifactHeaders(w http.ResponseWriter, path string) {
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		w.Header().Set("Content-Type", ct)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
}

// writeError matches the JSON envelope every other surface answers an error
// with — see httpapi's own writeError.
func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	e, ok := apperr.As(err)
	if !ok {
		e = apperr.New("ARTIFACTAPI_INTERNAL").
			Causer("artifactapi").
			Msgf("the request could not be completed").
			Status(apperr.StatusInternalServerError).
			Wrap(err)
		log.Error("unclassified error escaped an artifact handler", "err", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": e})
}
