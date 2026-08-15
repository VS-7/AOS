// Package oauthfile reads a credential another tool on this machine owns.
//
// Two providers work this way: codex reads what the ChatGPT tooling wrote, and
// gemini-cli reads what the Gemini CLI wrote. It is how somebody uses a
// subscription they already pay for instead of paying per token, and it is the
// configuration that was actually in use on the machine this was reverse
// engineered from.
//
// The file belongs to somebody else, and that shapes two decisions. A refresh
// is serialised across processes with a lock file, because two of ours renewing
// at once would corrupt a third party's credential — damage outside our own
// system. And a missing file fails loudly instead of falling back to an API
// key: the person chose this for a reason, and switching them to per-token
// billing without saying so would be an expensive surprise.
package oauthfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/OWNER/aos/internal/core/apperr"
)

// RefreshFunc exchanges a refresh token for a new access token.
type RefreshFunc func(ctx context.Context, refresh string) (Credentials, error)

// Credentials is the shape both files share, under different key names.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"-"`
}

// Store reads and refreshes one credential file.
type Store struct {
	// Path is the file another tool owns.
	Path string

	// Owner names that tool in errors, so a person knows what to go and fix.
	Owner string

	// Parse reads the file's own shape. The two files differ, and the
	// difference is not worth a second package.
	Parse func(raw []byte) (Credentials, error)

	// Refresh renews an expired token. Nil means this store cannot refresh,
	// and an expired token is then an error that says to log in again.
	Refresh RefreshFunc

	// Clock is injected so a test can expire a token without waiting.
	Clock func() time.Time

	mu     sync.Mutex
	cached Credentials
}

// Codex reads ~/.codex/auth.json.
func Codex(home string) *Store {
	return &Store{
		Path:  filepath.Join(home, ".codex", "auth.json"),
		Owner: "the Codex CLI",
		Parse: parseCodex,
	}
}

// GeminiCLI reads ~/.gemini/oauth_creds.json.
func GeminiCLI(home string) *Store {
	return &Store{
		Path:  filepath.Join(home, ".gemini", "oauth_creds.json"),
		Owner: "the Gemini CLI",
		Parse: parseGemini,
	}
}

// Token returns a usable access token, refreshing it if it has expired.
func (s *Store) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cached.AccessToken != "" && !s.expired(s.cached) {
		return s.cached.AccessToken, nil
	}

	creds, err := s.read()
	if err != nil {
		return "", err
	}
	if !s.expired(creds) {
		s.cached = creds
		return creds.AccessToken, nil
	}
	if s.Refresh == nil || creds.RefreshToken == "" {
		return "", errExpired(s.Path, s.Owner)
	}

	// The lock is across processes: the daemon and a CLI both refreshing would
	// otherwise race to rewrite a file neither of them owns.
	lock := flock.New(s.Path + ".lock")
	lctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	locked, err := lock.TryLockContext(lctx, 100*time.Millisecond)
	if err != nil || !locked {
		return "", errLocked(s.Path, s.Owner, err)
	}
	defer func() { _ = lock.Unlock() }()

	// Another process may have refreshed while this one waited, in which case
	// there is nothing to do and one fewer round trip to make.
	if again, err := s.read(); err == nil && !s.expired(again) {
		s.cached = again
		return again.AccessToken, nil
	}

	fresh, err := s.Refresh(ctx, creds.RefreshToken)
	if err != nil {
		return "", errRefreshFailed(s.Path, s.Owner, err)
	}
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = creds.RefreshToken
	}
	if err := s.write(fresh); err != nil {
		return "", err
	}
	s.cached = fresh
	return fresh.AccessToken, nil
}

func (s *Store) read() (Credentials, error) {
	raw, err := os.ReadFile(s.Path) //nolint:gosec // the path is built from the user's own home directory
	if os.IsNotExist(err) {
		return Credentials{}, errMissing(s.Path, s.Owner)
	}
	if err != nil {
		return Credentials{}, errUnreadable(s.Path, s.Owner, err)
	}
	creds, err := s.Parse(raw)
	if err != nil {
		return Credentials{}, errUnreadable(s.Path, s.Owner, err)
	}
	if creds.AccessToken == "" {
		return Credentials{}, errMissing(s.Path, s.Owner)
	}
	return creds, nil
}

// write puts the refreshed token back, preserving whatever else the file held.
// It is the one write this package makes to somebody else's file, and it keeps
// every key it did not understand.
func (s *Store) write(creds Credentials) error {
	raw, err := os.ReadFile(s.Path) //nolint:gosec // the path is built from the user's own home directory
	if err != nil {
		return errUnreadable(s.Path, s.Owner, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return errUnreadable(s.Path, s.Owner, err)
	}
	applyTokens(doc, creds)

	next, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return errUnreadable(s.Path, s.Owner, err)
	}
	if err := os.WriteFile(s.Path, next, 0o600); err != nil {
		return errUnreadable(s.Path, s.Owner, err)
	}
	return nil
}

func (s *Store) expired(c Credentials) bool {
	if c.ExpiresAt.IsZero() {
		return false // a file that does not say cannot be assumed stale
	}
	// A minute of margin, so a token that expires mid-flight does not.
	return !c.ExpiresAt.After(s.now().Add(time.Minute))
}

func (s *Store) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now() //nolint:forbidigo // a credential's expiry is wall-clock, and the field above is how a test overrides it
}

// applyTokens writes the tokens back under whichever key names the file uses,
// so a refreshed credential stays readable by the tool that owns it.
func applyTokens(doc map[string]any, creds Credentials) {
	if tokens, ok := doc["tokens"].(map[string]any); ok {
		tokens["access_token"] = creds.AccessToken
		if creds.RefreshToken != "" {
			tokens["refresh_token"] = creds.RefreshToken
		}
		return
	}
	if _, ok := doc["access_token"]; ok {
		doc["access_token"] = creds.AccessToken
		if creds.RefreshToken != "" {
			doc["refresh_token"] = creds.RefreshToken
		}
		return
	}
	doc["access_token"] = creds.AccessToken
	if creds.RefreshToken != "" {
		doc["refresh_token"] = creds.RefreshToken
	}
}

func parseCodex(raw []byte) (Credentials, error) {
	var doc struct {
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			IDToken      string `json:"id_token"`
		} `json:"tokens"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		LastRefresh  string `json:"last_refresh"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Credentials{}, err
	}
	out := Credentials{
		AccessToken:  firstNonEmpty(doc.Tokens.AccessToken, doc.AccessToken),
		RefreshToken: firstNonEmpty(doc.Tokens.RefreshToken, doc.RefreshToken),
	}
	return out, nil
}

func parseGemini(raw []byte) (Credentials, error) {
	var doc struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiryDate   int64  `json:"expiry_date"` // milliseconds since the epoch
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Credentials{}, err
	}
	out := Credentials{AccessToken: doc.AccessToken, RefreshToken: doc.RefreshToken}
	if doc.ExpiryDate > 0 {
		out.ExpiresAt = time.UnixMilli(doc.ExpiryDate)
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func errMissing(path, owner string) error {
	return apperr.New("OAUTH_FILE_MISSING").
		Causer("oauthfile.Store.Token").
		Msgf("this provider reads its credential from %s, and there is nothing usable there", path).
		Issue("path", path).
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label: "sign in with " + owner + " so it writes the file, or configure a provider that uses an API key",
		})
}

func errUnreadable(path, owner string, cause error) error {
	return apperr.New("OAUTH_FILE_UNREADABLE").
		Causer("oauthfile.Store").
		Msgf("the credential file at %s could not be read", path).
		Issue("path", path).
		Status(apperr.StatusUnauthorized).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "sign in again with " + owner + " to rewrite it"})
}

func errExpired(path, owner string) error {
	return apperr.New("OAUTH_TOKEN_EXPIRED").
		Causer("oauthfile.Store.Token").
		Msgf("the credential in %s has expired and cannot be renewed from here", path).
		Issue("path", path).
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{Label: "sign in again with " + owner})
}

func errLocked(path, owner string, cause error) error {
	return apperr.New("OAUTH_FILE_LOCKED").
		Causer("oauthfile.Store.Token").
		Msgf("another process is renewing the credential in %s", path).
		Issue("path", path).
		Status(apperr.StatusConflict).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "try again in a moment; " + owner + " may be refreshing it"})
}

func errRefreshFailed(path, owner string, cause error) error {
	return apperr.New("OAUTH_REFRESH_FAILED").
		Causer("oauthfile.Store.Token").
		Msgf("the credential in %s could not be renewed", path).
		Issue("path", path).
		Status(apperr.StatusUnauthorized).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "sign in again with " + owner})
}
