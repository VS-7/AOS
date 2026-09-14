// Package authapi is the HTTP surface of the identity domain: the account a
// person logs into, mounted at /api/auth outside the authenticated group
// httpapi guards its command routes with — a request has to reach login or
// onboarding before it can possibly hold a credential.
//
// Like fileapi, this is a router of its own rather than a command.Registry
// surface: internal/domain/auth's own package doc says it plainly — "there
// are no tools here and there never will be: an agent operates the domain,
// not the identity that authorises it." Keeping auth structurally outside
// the registry means there is no flag to forget; it is simply not reachable
// from there.
package authapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/domain/auth"
)

const maxBodyBytes = 1 << 20 // 1 MiB — these bodies are a handful of short fields

// cookieSession must match httpapi's own cookie name — that is the whole
// point: a browser that logs in here is recognised by the same middleware
// that guards every other route, with no second credential channel to keep
// in sync.
const cookieSession = "sessionToken"

// Config is what the router is built from.
type Config struct {
	Service *auth.Service
	Log     *slog.Logger

	// Clock is the same one the auth domain mints tokens against. It is here
	// because onboarding's response carries a display-only expiry this layer
	// computes itself (see onboarding), and a handler that read the wall
	// clock directly would drift from the service that issued the token.
	Clock clockx.Clock

	// Paths locates local.token — see its own doc comment
	// (internal/core/config/paths.go). Onboarding writes the freshly minted,
	// indefinite token there, once, the moment the account is created: the
	// one place a same-machine, same-user process (the CLI first among them)
	// can read a working credential without an interactive login. Zero value
	// (Paths{}) skips the write rather than writing to "local.token" in the
	// process's own working directory, which a test building this Config
	// without a real installation could otherwise do by accident.
	Paths corecfg.Paths
}

// New builds the router. It is mounted by the caller — see httpapi's
// AuthRoutes field — outside any authentication middleware; each handler
// below decides for itself whether it needs an identity already.
func New(cfg Config) http.Handler {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = clockx.System{}
	}
	s := &server{svc: cfg.Service, log: cfg.Log, clock: cfg.Clock, paths: cfg.Paths}

	r := chi.NewRouter()
	r.Get("/status", s.status)
	r.Post("/login", s.login)
	r.Post("/onboarding", s.onboarding)
	r.Post("/logout", s.logout)
	r.Get("/session", s.session)
	r.Post("/password", s.changePassword)
	r.Post("/profile", s.updateProfile)
	r.Get("/users", s.users)
	r.Get("/api-token", s.apiToken)
	r.Post("/api-token", s.regenerateAPIToken)
	return r
}

type server struct {
	svc   *auth.Service
	log   *slog.Logger
	clock clockx.Clock
	paths corecfg.Paths
}

// status answers "what should this page show" without requiring a
// credential: no account yet means Onboarding, an account but no valid
// session means Login, and a valid session means the app itself.
func (s *server) status(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.Users(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	authenticated := false
	if bearer := bearerOf(r); bearer != "" {
		if _, err := s.svc.Authenticate(r.Context(), bearer); err == nil {
			authenticated = true
		}
	}
	s.writeJSON(w, map[string]any{
		"onboarded":     len(users) > 0,
		"authenticated": authenticated,
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	session, err := s.svc.Login(r.Context(), auth.LoginInput{Identifier: in.Identifier, Password: in.Password})
	if err != nil {
		s.writeError(w, err)
		return
	}
	user, err := s.svc.Get(r.Context(), session.UserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeSession(w, user, session.Token, session.ExpiresAt)
}

func (s *server) onboarding(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	// Onboarding has no separate username field in the wizard — the email
	// itself doubles as the identifier Login accepts, which is the only
	// thing that has to be true for a person to log back in afterward.
	out, err := s.svc.Onboarding(r.Context(), auth.OnboardingInput{
		Name: in.Name, Username: in.Email, Email: in.Email, Password: in.Password,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	// The one place local.token is ever written — see Config.Paths' own doc
	// comment. A failure here is logged, not returned: the account exists
	// and the browser session below still works regardless of whether the
	// CLI's own bootstrap credential landed on disk.
	//
	// A credential of its own, not the one this response carries. They used
	// to be the same string, and logging out of the window then revoked the
	// terminal's only credential too — permanently, since nothing writes
	// this file a second time: `aos agents list` went back to answering with
	// four built-in commands and no explanation, and reinstalling did not
	// help. They are two credentials for two clients, and one signing out
	// says nothing about the other.
	if s.paths.Root != "" {
		_, cli, err := s.svc.IssueToken(r.Context(), auth.IssueTokenInput{
			UserID: out.User.ID, Name: "cli",
		})
		if err != nil {
			s.log.Warn("could not mint the terminal's credential", "err", err)
		} else if err := corecfg.WriteSecret(s.paths.LocalToken(), []byte(cli)); err != nil {
			s.log.Warn("could not write local.token", "err", err)
		}
	}

	// Onboarding mints a token but not an expiry the way Login does — see
	// its own doc: it is the first, indefinite credential of the account. A
	// far-future date here is display-only; the token has no ExpiresAt on
	// the server side to actually enforce one.
	s.writeSession(w, out.User, out.Token, s.clock.Now().AddDate(10, 0, 0))
}

// users lists the accounts on this installation.
//
// Three screens need it — the workspace roster the sidebar's Team tab draws,
// the assignee picker on a task, and Settings → Users — and all three rendered
// empty, because the domain had the list (auth.Service.Users) and nothing
// published it.
//
// It answers the public projection, which is the whole point of that type
// existing: a roster is names and addresses, never the argon2 hash beside them
// in users.json.
//
// A credential is required. Who has an account here is not something an
// unauthenticated caller on the loopback interface should be able to enumerate.
func (s *server) users(w http.ResponseWriter, r *http.Request) {
	if _, err := s.authenticate(r); err != nil {
		s.writeError(w, err)
		return
	}
	found, err := s.svc.Users(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	// A non-nil slice: an installation with no accounts answers `[]`, not
	// `null`, so the interface can map over it without a guard.
	if found == nil {
		found = []auth.Public{}
	}
	s.writeJSON(w, map[string]any{"users": found})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if bearer := bearerOf(r); bearer != "" {
		if err := s.svc.RevokeByToken(r.Context(), bearer); err != nil {
			s.writeError(w, err)
			return
		}
	}
	clearSessionCookie(w)
	s.writeJSON(w, map[string]any{})
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	user, err := s.authenticate(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]any{"user": user.ToPublic()})
}

func (s *server) changePassword(w http.ResponseWriter, r *http.Request) {
	user, err := s.authenticate(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	var in struct {
		Current string `json:"currentPassword"`
		New     string `json:"newPassword"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if err := s.svc.ChangePassword(r.Context(), auth.ChangePasswordInput{
		UserID: user.ID, Current: in.Current, New: in.New,
	}); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]any{})
}

// updateProfile changes the signed-in account's name, email and avatar.
//
// Same shape as session on the way out — {"user": {...}} — so the interface
// can drop the answer straight into the store it read session into, rather
// than reconciling two projections of one account.
func (s *server) updateProfile(w http.ResponseWriter, r *http.Request) {
	user, err := s.authenticate(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	var in struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		// A pointer, so a body that does not mention the avatar leaves it
		// alone and one that sends "" removes it.
		Image *string `json:"image"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	updated, err := s.svc.UpdateProfile(r.Context(), auth.UpdateProfileInput{
		UserID: user.ID, Name: in.Name, Email: in.Email, Image: in.Image,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]any{"user": updated})
}

// apiTokenView is what a client is told about an API credential: enough to
// tell which one is configured, nothing that authenticates.
type apiTokenView struct {
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

// apiToken describes the signed-in account's API credential, or answers
// {"token": null} when it has none.
//
// Settings > Developers shows it, so a person can tell whether the token in
// their MCP client is the current one. Identity sits outside the command
// registry (see the package doc), so this is a route and not a command.
func (s *server) apiToken(w http.ResponseWriter, r *http.Request) {
	user, err := s.authenticate(r)
	if err != nil {
		s.writeError(w, err)
		return
	}
	current, err := s.svc.APIToken(r.Context(), user.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if current == nil {
		s.writeJSON(w, map[string]any{"token": nil})
		return
	}
	s.writeJSON(w, map[string]any{"token": apiTokenView{Prefix: current.Prefix, CreatedAt: current.CreatedAt, LastUsedAt: current.LastUsed}})
}

// regenerateAPIToken replaces the signed-in account's API credential and
// answers the new value — the only time it is ever answered.
//
// It revokes the token every MCP client the person configured is using, so two
// callers that can authenticate are still refused. The API token itself: it is
// what those clients and agents hold, and accepting it let any of them replace
// the person's token and keep the new one. And a request a browser sends
// without asking — the session cookie is SameSite=Lax, and a page on any other
// 127.0.0.1 port is the same site, so a plain form there posted here with the
// cookie attached. A form cannot send application/json, and a script that does
// needs a preflight the daemon's CORS policy answers only for the window. That
// check runs first, so a forged request touches no credential at all.
func (s *server) regenerateAPIToken(w http.ResponseWriter, r *http.Request) {
	if !sentAsJSON(r) {
		s.writeError(w, errNotJSON())
		return
	}
	user, presented, err := s.svc.Credential(r.Context(), bearerOf(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if presented.Name == auth.APITokenName {
		s.writeError(w, errAPITokenReplacingItself())
		return
	}
	token, plain, err := s.svc.RegenerateAPIToken(r.Context(), user.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, map[string]any{"token": plain, "prefix": token.Prefix, "createdAt": token.CreatedAt})
}

func (s *server) authenticate(r *http.Request) (*auth.User, error) {
	bearer := bearerOf(r)
	if bearer == "" {
		return nil, errUnauthenticated()
	}
	return s.svc.Authenticate(r.Context(), bearer)
}

// sentAsJSON reports whether r declares a JSON body, which only a request a
// browser would preflight can.
func sentAsJSON(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func (s *server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		s.writeError(w, errBodyTooLarge(maxBodyBytes))
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		s.writeError(w, errBadRequestBody(err))
		return false
	}
	return true
}

// writeSession answers a successful login or onboarding: the plain token in
// the body, for a caller that is not a browser (the desktop's Go-side
// client, a future CLI), and the same value set as an HttpOnly cookie for
// one that is — a browser tab never has to read or store the token itself.
func (s *server) writeSession(w http.ResponseWriter, user auth.Public, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSession,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	s.writeJSON(w, map[string]any{
		"user":      user,
		"token":     token,
		"expiresAt": expiresAt,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieSession, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// bearerOf mirrors httpapi's own credential extraction exactly (Authorization
// header, then X-Auth-Token, then the sessionToken cookie) — duplicated
// rather than exported across a package boundary for ten lines nobody else
// needs.
func bearerOf(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if rest, ok := cutPrefixFold(h, "bearer "); ok {
			return strings.TrimSpace(rest)
		}
	}
	if h := strings.TrimSpace(r.Header.Get("X-Auth-Token")); h != "" {
		return h
	}
	if c, err := r.Cookie(cookieSession); err == nil {
		return strings.TrimSpace(c.Value)
	}
	return ""
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

func (s *server) writeJSON(w http.ResponseWriter, out any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(command.Wrap(out, nil)); err != nil {
		s.log.Warn("an auth response could not be written", "err", err)
	}
}

func (s *server) writeError(w http.ResponseWriter, err error) {
	e, ok := apperr.As(err)
	if !ok {
		e = apperr.New("AUTH_HTTP_INTERNAL").
			Causer("authapi").
			Msgf("the request could not be completed").
			Status(apperr.StatusInternalServerError).
			Wrap(err)
		s.log.Error("unclassified error escaped an auth handler", "err", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": e})
}
