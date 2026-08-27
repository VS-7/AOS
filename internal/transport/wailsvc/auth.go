package wailsvc

import "context"

// PublicUser mirrors auth.Public's JSON shape without importing the domain
// package — a client binary may not link domain code, the same reason
// DomainService carries a plain CommandInfo rather than a command.Descriptor.
type PublicUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// AuthResult is what a successful login or onboarding answers with.
type AuthResult struct {
	User      PublicUser `json:"user"`
	ExpiresAt string     `json:"expiresAt"`
}

// SessionResult wraps the signed-in account.
//
// The wrapper is not decoration: this method and the daemon's own
// GET /api/auth/session are two transports for one question, and the
// interface picks between them at runtime (frontend's lib/auth.ts tries the
// bridge and falls back to HTTP). HTTP answers {"data":{"user":{...}}}, so
// `const { user } = await session()` is what every caller writes. Returning a
// bare PublicUser here made that destructuring yield undefined for the whole
// desktop window — the account had no name in the sidebar, the settings page
// showed an empty form, and `user.role === "super"` was false, which is what
// hides the button that creates a workspace. Signed in, and invisible.
type SessionResult struct {
	User PublicUser `json:"user"`
}

// AuthStatus is what the window checks before deciding what to show: nobody
// onboarded yet, an account exists but this window is not signed in, or it
// already is.
type AuthStatus struct {
	Onboarded     bool `json:"onboarded"`
	Authenticated bool `json:"authenticated"`
}

// AuthCaller is how the desktop reaches the identity domain: over the
// daemon's own HTTP surface (internal/transport/daemonclient), never by
// linking internal/domain/auth directly. It stores the resulting token
// itself on success, which is what lets every DomainService.Invoke call
// after a login already carry it.
type AuthCaller interface {
	Status(ctx context.Context) (AuthStatus, error)
	Login(ctx context.Context, identifier, password string) (AuthResult, error)
	Onboarding(ctx context.Context, name, email, password string) (AuthResult, error)
	Logout(ctx context.Context) error
	Session(ctx context.Context) (PublicUser, error)
}

// AuthService is the desktop window's door to logging in.
//
// It exists for the same reason DomainService's Invoke does rather than a
// method per command: the window needs exactly these four operations, and
// they are the ones this codebase deliberately keeps out of the command
// registry — see internal/domain/auth's package doc, "there are no tools
// here and there never will be."
type AuthService struct {
	caller AuthCaller

	// afterAuth runs once a Login or Onboarding call succeeds. It exists for
	// workspace registration: that call needs a token, which a fresh
	// installation does not have until this moment — see cmd/aos-desktop's
	// wiring, the only place this is non-nil today.
	afterAuth func(ctx context.Context)
}

// NewAuth builds the service over a caller. afterAuth may be nil.
func NewAuth(caller AuthCaller, afterAuth func(ctx context.Context)) *AuthService {
	return &AuthService{caller: caller, afterAuth: afterAuth}
}

// ServiceName is what Wails calls this service in the generated bindings.
func (s *AuthService) ServiceName() string { return "AuthService" }

// Status reports what this window should show: Onboarding, Login, or the
// application itself.
func (s *AuthService) Status(ctx context.Context) (AuthStatus, error) {
	if s.caller == nil {
		return AuthStatus{}, errNoDaemon()
	}
	return s.caller.Status(ctx)
}

// Login signs into an existing account.
func (s *AuthService) Login(ctx context.Context, identifier, password string) (AuthResult, error) {
	if s.caller == nil {
		return AuthResult{}, errNoDaemon()
	}
	out, err := s.caller.Login(ctx, identifier, password)
	if err == nil && s.afterAuth != nil {
		s.afterAuth(ctx)
	}
	return out, err
}

// Onboarding creates the installation's first account.
func (s *AuthService) Onboarding(ctx context.Context, name, email, password string) (AuthResult, error) {
	if s.caller == nil {
		return AuthResult{}, errNoDaemon()
	}
	out, err := s.caller.Onboarding(ctx, name, email, password)
	if err == nil && s.afterAuth != nil {
		s.afterAuth(ctx)
	}
	return out, err
}

// Logout ends the current session.
func (s *AuthService) Logout(ctx context.Context) error {
	if s.caller == nil {
		return errNoDaemon()
	}
	return s.caller.Logout(ctx)
}

// Session reports who, if anyone, this window is currently signed in as.
//
// Wrapped in SessionResult so the bridge and the daemon's HTTP route answer
// the same shape — see SessionResult's own doc for what the bare value cost.
func (s *AuthService) Session(ctx context.Context) (SessionResult, error) {
	if s.caller == nil {
		return SessionResult{}, errNoDaemon()
	}
	user, err := s.caller.Session(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	return SessionResult{User: user}, nil
}
