package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/transport/daemonclient"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// desktopTokenFile is where the window keeps its own session, beside
// local.token in the state directory.
const desktopTokenFile = "desktop.token"

// windowSession is the credential this window signs in with, and what it
// remembers about it between launches.
//
// There are two kinds, and the window used to treat them as one. The shared
// credential — AOS_TOKEN, or ~/.aos/local.token, which onboarding writes for
// the terminal — belongs to somebody else: `aos` reads the same file. The
// window's own session is what AuthService.Login or Onboarding minted for it.
// Logout revoked whatever the window held, so logging out of the window
// revoked the terminal's only credential, permanently (nothing writes
// local.token a second time), and the window's own login lived only in memory,
// so every launch after that asked for a password.
//
// desktop.token says which state the window is in:
//
//   - absent: it has never signed in or out itself, and uses the shared
//     credential, which is what makes the application open signed in with
//     nobody watching;
//   - holding a token: its own session, from the last sign-in;
//   - empty: the person signed out, and the next launch shows Login rather
//     than quietly signing in with the credential they just gave up.
//
// AOS_TOKEN, when set, is neither replaced nor revoked: it points the window
// at a daemon it did not start, and belongs to whoever set it.
type windowSession struct {
	path     string
	shared   string
	explicit bool
	log      *slog.Logger
}

func newWindowSession(path, shared string, explicit bool, log *slog.Logger) *windowSession {
	if log == nil {
		log = slog.Default()
	}
	return &windowSession{path: path, shared: strings.TrimSpace(shared), explicit: explicit, log: log}
}

// Initial is the token a launch starts with.
func (s *windowSession) Initial() string {
	if s.explicit {
		return s.shared
	}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s.shared
	}
	if err != nil {
		// Unreadable is not "signed out": fall back to what a first launch
		// would use, and say so.
		s.log.Warn("the window's session could not be read; using the installation's credential", "path", s.path, "err", err)
		return s.shared
	}
	return strings.TrimSpace(string(raw))
}

// remember writes the window's own session, or "" for signed out.
func (s *windowSession) remember(token string) {
	if s.explicit {
		return
	}
	if err := corecfg.WriteSecret(s.path, []byte(token)); err != nil {
		s.log.Warn("the window's session could not be saved; the next launch will ask again", "path", s.path, "err", err)
	}
}

// Caller is the AuthCaller the window's AuthService goes through: the daemon
// client, with sign-in remembered and sign-out kept off the shared credential.
func (s *windowSession) Caller(client *daemonclient.Client) wailsvc.AuthCaller {
	return sessionCaller{Client: client, session: s}
}

type sessionCaller struct {
	*daemonclient.Client
	session *windowSession
}

func (c sessionCaller) Login(ctx context.Context, identifier, password string) (wailsvc.AuthResult, error) {
	out, err := c.Client.Login(ctx, identifier, password)
	if err == nil {
		c.session.remember(c.Token())
	}
	return out, err
}

func (c sessionCaller) Onboarding(ctx context.Context, name, email, password string) (wailsvc.AuthResult, error) {
	out, err := c.Client.Onboarding(ctx, name, email, password)
	if err == nil {
		c.session.remember(c.Token())
	}
	return out, err
}

// Logout ends the window's session. Its own is revoked; the shared credential
// is only let go of, because it is not the window's to end.
func (c sessionCaller) Logout(ctx context.Context) error {
	held := c.Token()
	if held != "" && held == c.session.shared {
		c.SetToken("")
	} else if err := c.Client.Logout(ctx); err != nil {
		return err
	}
	c.session.remember("")
	return nil
}
