// Package antigravity speaks the Cloud Code v1internal API behind Google's
// Antigravity CLI.
//
// It exists because the Gemini CLI is gone: Google retired it on 18 June 2026
// and moved individual accounts onto Antigravity, so the `gemini-cli` adapter
// beside this one now reads a credential that no longer buys a personal
// account anything. This is the same idea pointed at what replaced it — spend
// an allowance somebody already has, through the login the official client
// already wrote, instead of a metered API key.
//
// Three decisions in this package are about the account rather than about
// making a call succeed, and they are why this is not the google adapter with
// a different base URL:
//
//   - The credential is read, never minted. There is no browser flow and no
//     device flow here. If the machine holds no Antigravity login, this says
//     so and stops. Signing in belongs to the official client, and a second
//     thing minting grants against the same OAuth client is a good way to end
//     up with a revoked one.
//   - Every request carries what the official client carries: the same host,
//     the same user agent, the same client metadata. A request that looks like
//     the product it is billed to is the one that does not read as abuse.
//   - A refusal is never retried, and it closes the door for a while.
//     guard.go is the whole of that argument.
package antigravity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/runtime/providers/oauthfile"
)

// The Antigravity CLI's own OAuth client, which renewal needs and this
// repository does not carry.
//
// The pair is Google's, not ours. An installed application's client
// credentials are distributed in every copy of the product and Google issues
// them precisely so a desktop client can complete the refresh half of the
// flow — but they still identify Google's client, and committing them here
// would republish another party's credential from a repository that is not
// theirs, with the OAuth client's revocation as the blast radius for every
// user of the official CLI. GitHub's own push protection refuses it, and it
// is right to.
//
// So they come from the environment, with no default. Read them out of the
// shipped `agy` binary on the machine that already has it installed:
//
//	strings "$(command -v agy)" | grep -E 'apps\.googleusercontent\.com|^GOCSPX-'
//
// then set AOS_ANTIGRAVITY_CLIENT_ID and AOS_ANTIGRAVITY_CLIENT_SECRET. The
// binary carries a second client id belonging to another surface, which
// refuses these tokens; the pair that works is the one whose id is quoted
// beside the secret.
//
// Everything else in this adapter works without them. Only renewal needs the
// pair, so an installation that never lets its token expire never notices —
// which is why this is checked at renewal rather than at construction.
const (
	envClientID     = "ANTIGRAVITY_CLIENT_ID"
	envClientSecret = "ANTIGRAVITY_CLIENT_SECRET"

	oauthTokenURL = "https://oauth2.googleapis.com/token"
)

// oauthClient reads the pair, reporting whether both are present.
func oauthClient() (id, secret string, ok bool) {
	id = strings.TrimSpace(os.Getenv(env.Key(envClientID)))
	secret = strings.TrimSpace(os.Getenv(env.Key(envClientSecret)))
	return id, secret, id != "" && secret != ""
}

// owner names the tool whose file this is, for the errors that ask a person to
// go and fix something. It is the CLI's user-facing name, not the binary's.
const owner = "the Antigravity CLI (agy)"

// refreshTimeout bounds a token renewal. It is short on purpose: a renewal
// that has not answered in half a minute has failed, and the call waiting on
// it should say so rather than sit inside the turn's ten-minute budget.
const refreshTimeout = 30 * time.Second

// credentialPaths lists the files an Antigravity login is written to, in the
// order they are tried.
//
// Two, because two official clients write one. The CLI keeps
// ~/.gemini/antigravity-cli/antigravity-oauth-token; the IDE keeps
// ~/.gemini/jetski-standalone-oauth-token, "jetski" being the internal name
// that still shows through in the CLI's own state files. On a machine with
// both they hold the same grant, so the first that yields a usable token is
// the answer and the second is never opened.
func credentialPaths(home string) []string {
	return []string{
		filepath.Join(home, ".gemini", "antigravity-cli", "antigravity-oauth-token"),
		filepath.Join(home, ".gemini", "jetski-standalone-oauth-token"),
	}
}

// tokens reads whichever of those files holds a login.
//
// Neither is the primary store on macOS: the CLI keeps the live token in the
// keychain and treats the file as its fallback, which is why the access token
// on disk is routinely stale by the time this reads it. That costs nothing.
// The refresh token beside it is what this package actually needs, it does not
// rotate, and renewing from it is the same exchange the official client makes.
type tokens struct {
	stores []*oauthfile.Store
}

// newTokens builds the reader for one home directory.
func newTokens(home string, clock func() time.Time) *tokens {
	paths := credentialPaths(home)
	out := &tokens{stores: make([]*oauthfile.Store, 0, len(paths))}
	for _, path := range paths {
		out.stores = append(out.stores, &oauthfile.Store{
			Path:    path,
			Owner:   owner,
			Parse:   parseCredential,
			Refresh: renew,
			Clock:   clock,
		})
	}
	return out
}

// Token returns a usable access token, renewing it when the file's has expired.
//
// The first store that answers wins. A store that has no file is skipped
// rather than reported, because "the IDE never signed in" is not a fault when
// the CLI did; only the last failure survives, and it is the one that names a
// path worth looking at.
func (t *tokens) Token(ctx context.Context) (string, error) {
	var last error
	for _, store := range t.stores {
		token, err := store.Token(ctx)
		if err == nil {
			return token, nil
		}
		last = err
	}
	if last == nil {
		return "", errNoLogin(credentialPaths(""))
	}
	return "", last
}

// parseCredential reads the shape the Go client marshals.
//
// It is an oauth2.Token under a "token" key, with an "auth_method" beside it
// naming the kind of account. The flat form is accepted too: it is what the
// same struct serialises to when it is stored on its own, and accepting both
// costs four lines rather than a support conversation.
func parseCredential(raw []byte) (oauthfile.Credentials, error) {
	var doc struct {
		Token struct {
			AccessToken  string    `json:"access_token"`
			RefreshToken string    `json:"refresh_token"`
			Expiry       time.Time `json:"expiry"`
		} `json:"token"`
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return oauthfile.Credentials{}, err
	}
	out := oauthfile.Credentials{
		AccessToken:  firstNonEmpty(doc.Token.AccessToken, doc.AccessToken),
		RefreshToken: firstNonEmpty(doc.Token.RefreshToken, doc.RefreshToken),
	}
	if expiry := firstNonZero(doc.Token.Expiry, doc.Expiry); !expiry.IsZero() {
		out.ExpiresAt = expiry
	}
	return out, nil
}

// renew exchanges a refresh token for a new access token.
//
// It is the standard installed-application exchange, made with the official
// client's own credentials, which is what makes the resulting token
// indistinguishable from one the official client renewed. The caller holds a
// cross-process lock around this — see oauthfile.Store.Token — because the
// file being rewritten belongs to somebody else.
func renew(ctx context.Context, refresh string) (oauthfile.Credentials, error) {
	clientID, clientSecret, ok := oauthClient()
	if !ok {
		return oauthfile.Credentials{}, errNoOAuthClient()
	}

	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()

	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refresh},
		"grant_type":    {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthfile.Credentials{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthfile.Credentials{}, err
	}
	defer func() { _ = res.Body.Close() }()

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return oauthfile.Credentials{}, err
	}
	if res.StatusCode >= 300 || body.AccessToken == "" {
		return oauthfile.Credentials{}, errRenewRefused(res.StatusCode, body.Error, body.Description)
	}

	out := oauthfile.Credentials{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken}
	if body.ExpiresIn > 0 {
		// The deadline is computed from the answer rather than trusted from
		// the file, so a clock that has drifted since the token was written
		// does not make a live credential look expired on every call.
		out.ExpiresAt = timeNow().Add(time.Duration(body.ExpiresIn) * time.Second)
	}
	return out, nil
}

// timeNow is the one wall-clock read this package makes on its own behalf.
//
// It is a variable rather than a call so a test can pin it, and it exists
// because an OAuth expiry is wall-clock by definition: there is no injected
// clock at the point a refresh answers, only the "expires_in" the token
// endpoint returned and the instant it arrived.
var timeNow = time.Now //nolint:forbidigo // an OAuth expiry is wall-clock, and this indirection is what a test overrides

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(values ...time.Time) time.Time {
	for _, v := range values {
		if !v.IsZero() {
			return v
		}
	}
	return time.Time{}
}

func errNoLogin(paths []string) error {
	return apperr.New("ANTIGRAVITY_NOT_SIGNED_IN").
		Causer("antigravity.tokens.Token").
		Msgf("no Antigravity login was found on this machine").
		Issue("paths", paths).
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label:   "sign in once with the Antigravity CLI; this provider reads the login it writes and never creates one",
			Command: "agy",
		})
}

// errNoOAuthClient is what a renewal says when the pair it needs is not set.
//
// It names both variables and where to find their values, because the person
// reading it has the answer on their own disk — see the constants above.
func errNoOAuthClient() error {
	return apperr.New("ANTIGRAVITY_NO_OAUTH_CLIENT").
		Causer("antigravity.renew").
		Msgf("the Antigravity login expired and renewing it needs the CLI's OAuth client, which this build does not carry").
		Issue("variables", []string{env.Key(envClientID), env.Key(envClientSecret)}).
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label: "read the pair out of the installed CLI — strings \"$(command -v agy)\" | grep -E 'apps[.]googleusercontent[.]com|^GOCSPX-' — and set " +
				env.Key(envClientID) + " and " + env.Key(envClientSecret) + "; or sign in again to get a fresh token",
			Command: "agy",
		})
}

func errRenewRefused(status int, code, description string) error {
	return apperr.New("ANTIGRAVITY_RENEW_REFUSED").
		Causer("antigravity.renew").
		Msgf("Google refused to renew the Antigravity login: %s", firstNonEmpty(description, code, "no reason given")).
		Issue("status", status).
		Issue("reason", code).
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label:   "the stored login is no longer valid; sign in again with the Antigravity CLI",
			Command: "agy",
		})
}
