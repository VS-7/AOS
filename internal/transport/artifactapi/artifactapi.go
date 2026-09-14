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
//
// Not reading the cookie here was not enough on its own: the browser still
// attaches it to whatever /api request the artifact's own script makes, from
// the same origin. So every answer is also sandboxed into an opaque origin —
// see sandbox.
package artifactapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

	// Scope, when set, resolves the Artifacts and Files a request reaches from
	// the workspace it names (httpapi's ambientIdentity has already read the
	// X-Workspace-ID header or cookie into the context); Artifacts and Files
	// above serve only when it is nil. A Scope that fails is the answer — it
	// never falls back to another workspace's artifacts.
	//
	// Without it the route served only the workspace the daemon started in,
	// while artifacts_create and artifacts_list route per workspace: an
	// artifact made in any other workspace had a URL and answered
	// AOS_ARTIFACT_NOT_FOUND.
	Scope func(ctx context.Context) (Artifacts, Files, error)

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
	artifacts, files := s.cfg.Artifacts, s.cfg.Files
	if s.cfg.Scope != nil {
		scoped, scopedFiles, err := s.cfg.Scope(r.Context())
		if err != nil {
			writeError(w, s.cfg.Log, err)
			return
		}
		artifacts, files = scoped, scopedFiles
	}

	id := chi.URLParam(r, "id")
	a, err := artifacts.Get(r.Context(), artifact.GetInput{ID: id})
	if err != nil {
		writeError(w, s.cfg.Log, err)
		return
	}

	req := artifact.AccessRequest{
		Authenticated: s.authenticated(r),
		Password:      r.URL.Query().Get("password"),
	}
	if err := artifacts.Authorize(r.Context(), a, req); err != nil {
		writeError(w, s.cfg.Log, err)
		return
	}

	rel := chi.URLParam(r, "*")
	if rel == "" {
		rel = a.Entrypoint
	}
	target, err := files.Resolve(id, rel)
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

	setArtifactHeaders(w, target, ownDirectory(r, id), a.Visibility == artifact.ByPassword)
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

// sandbox gives every artifact document an opaque origin, wherever it is
// opened: framed by the window or a browser tab, or on its own at a share link.
//
// 'self' in the policy below kept an artifact's script on this origin, which
// is exactly the problem: this origin is the API's. A browser signed in to it
// attaches the session cookie to every same-origin request, so a by_password
// artifact whose page loaded `probe.js?password=pw`, opened at its share link,
// ran as the signed-in person — /api/auth/session answered with the user,
// /api/tasks/list with the workspace, and localStorage was the interface's.
// The frame's own sandbox attribute cannot be the whole answer: a share link
// has no frame, and a browser tab framed artifacts with allow-same-origin.
//
// Sandboxed without allow-same-origin, a request the page makes to /api is
// cross-site: the Lax session cookie stays behind, and the answer is not
// readable. Scripts, forms and popups (which inherit the sandbox) are what an
// artifact is for; the top window is never its to navigate. Sent on every
// answer, not only on HTML: an SVG or an XML file opened on its own is a
// document that runs script too, and on a subresource the directive is
// ignored.
const sandbox = "sandbox allow-scripts allow-forms allow-popups"

// csp is the fixed, restrictive policy every artifact is served under: an
// opaque origin (sandbox above), no external network, no inline script, no
// framing by a third-party page. An artifact is generated content served from
// this daemon's own origin, and without this a script inside one could call
// this system's own /api/* endpoints or exfiltrate data to an
// attacker-controlled host. The design doc's own sketch anticipates a
// per-artifact opt-in to relax this; no such opt-in field exists on the entity
// yet, so every artifact gets the strict policy today — a smaller, disclosed
// limitation, not a silent one.
//
// connectTo, when not empty, is the artifact's own directory as an absolute
// address (see ownDirectory), added to connect-src.
func csp(connectTo string) string {
	connect := "connect-src 'self'"
	if connectTo != "" {
		connect += " " + connectTo
	}
	return sandbox + "; default-src 'self'; script-src 'self'; style-src 'self'; " +
		"img-src 'self' data:; font-src 'self' data:; " + connect + "; " +
		"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"
}

// ownDirectory is the address of the artifact's own directory, as the request
// reached it, for connect-src — or "" when the request does not say it plainly.
//
// WebKit stops counting an opaque page's own URL as connect-src 'self' (its
// scripts, styles and images still match; in Chromium everything does), so a
// shared artifact's fetch of its own data.json was refused before it was sent.
// Naming the directory lets exactly that through, and /api stays out of reach
// there. The host is the one the request was addressed to, and a scheme a
// proxy in front says it was reached by (a tunnel serves https); anything that
// is not a plain host and port adds nothing, since a CSP would read a stray
// ";" or "," as another directive or source.
func ownDirectory(r *http.Request, id string) string {
	if !plainHost.MatchString(r.Host) {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	// The mount point is the caller's (httpapi mounts this at /v), so it is
	// read back from the path the request actually carries.
	mount, _, found := strings.Cut(r.URL.Path, "/artifacts/"+id+"/")
	if !found {
		return ""
	}
	return scheme + "://" + r.Host + (&url.URL{Path: mount}).EscapedPath() + "/artifacts/" + url.PathEscape(id) + "/"
}

// plainHost is a host name or address with an optional port.
var plainHost = regexp.MustCompile(`^(\[[0-9A-Fa-f:.]+\]|[A-Za-z0-9.-]+)(:[0-9]{1,5})?$`)

// setArtifactHeaders applies the policy that keeps generated content from
// reaching back into the workspace it was generated in, and from being
// mistaken for something this daemon vouches for.
//
// passwordRead says the request was authorised by the artifact's password,
// which it presented itself. The page is opaque (see sandbox), so everything
// it loads of its own is a cross-origin request: a same-origin resource policy
// cancelled its stylesheet, script and image, and its fetch of its own data
// needed Access-Control-Allow-Origin. Whoever holds the password can read the
// file from anywhere already, so those answers go to any origin — without
// credentials, since nothing here is read with one the browser attaches. What
// is read on any other ground (a bearer header, or security switched off) is
// not the requester's to share, and still answers this origin only: a page
// elsewhere must not be able to embed it. The desktop window frames those at
// an address of its own that answers the opaque page (cmd/aos-desktop's
// artifactFrames).
func setArtifactHeaders(w http.ResponseWriter, path, connectTo string, passwordRead bool) {
	w.Header().Set("Content-Security-Policy", csp(connectTo))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if passwordRead {
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	}
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
