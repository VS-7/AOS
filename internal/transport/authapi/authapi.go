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
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
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
	s := &server{svc: cfg.Service, log: cfg.Log, clock: cfg.Clock}

	r := chi.NewRouter()
	r.Get("/status", s.status)
	r.Post("/login", s.login)
	r.Post("/onboarding", s.onboarding)
	r.Post("/logout", s.logout)
	r.Get("/session", s.session)
	r.Post("/password", s.changePassword)
	return r
}

type server struct {
	svc   *auth.Service
	log   *slog.Logger
	clock clockx.Clock
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
	// Onboarding mints a token but not an expiry the way Login does — see
	// its own doc: it is the first, indefinite credential of the account. A
	// far-future date here is display-only; the token has no ExpiresAt on
	// the server side to actually enforce one.
	s.writeSession(w, out.User, out.Token, s.clock.Now().AddDate(10, 0, 0))
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

func (s *server) authenticate(r *http.Request) (*auth.User, error) {
	bearer := bearerOf(r)
	if bearer == "" {
		return nil, errUnauthenticated()
	}
	return s.svc.Authenticate(r.Context(), bearer)
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
