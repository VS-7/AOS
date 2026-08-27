package auth

import (
	"fmt"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errPasswordTooShort(got int) error {
	return apperr.New("AUTH_PASSWORD_TOO_SHORT").
		Causer("auth.ValidatePassword").
		Msgf("a password needs at least %d characters, and this one has %d", MinPasswordLen, got).
		Issue("minimum", MinPasswordLen).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: fmt.Sprintf(
				"this account can run shell commands on this machine — use %d characters or more, and a passphrase counts",
				MinPasswordLen),
		})
}

func errPasswordBreached() error {
	return apperr.New("AUTH_PASSWORD_BREACHED").
		Causer("auth.ValidatePassword").
		Msgf("that password appears in the list of credentials tried first").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "pick something that is not in a wordlist; length helps more than punctuation does",
		})
}

// errInvalidCredentials is deliberately the same error whether the account does
// not exist or the password is wrong. Distinguishing them would turn the login
// endpoint into a way to enumerate accounts.
func errInvalidCredentials() error {
	return apperr.New("AUTH_INVALID_CREDENTIALS").
		Causer("auth.Service.Login").
		Msgf("those credentials do not match an account").
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{Label: "check the username and try again"})
}

func errUnauthenticated() error {
	return apperr.New("AUTH_UNAUTHENTICATED").
		Causer("auth.Service.Authenticate").
		Msgf("this request carries no valid credential").
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label:   "issue a token and send it as a bearer credential",
			Command: build.Name + " auth token issue",
		})
}

func errUserNotFound(id string) error {
	return apperr.New("AUTH_USER_NOT_FOUND").
		Causer("auth.Service").
		Msgf("no account %q", id).
		Issue("user", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the accounts on this installation",
			Command: build.Name + " auth users list",
		})
}

// errNameRequired refuses an account with nothing to call it.
//
// The name is not decoration: it is what the sidebar, the chat roster and
// every "assigned to" line show. Blanking it leaves an account nobody can
// point at.
func errNameRequired() error {
	return apperr.New("AUTH_NAME_REQUIRED").
		Causer("auth.Service.UpdateProfile").
		Msgf("an account needs a name").
		Issue("name", "is required").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "send a name — it is what identifies you everywhere in the interface",
		})
}

// errEmailTaken refuses a second account on one address.
//
// Login accepts either the username or the email, so two accounts sharing one
// would make which of them a password opens a matter of ordering.
func errEmailTaken(email string) error {
	return apperr.New("AUTH_EMAIL_TAKEN").
		Causer("auth.Service.UpdateProfile").
		Msgf("another account already uses %q", email).
		Issue("email", email).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "use an address no other account on this installation has",
		})
}

// errAlreadyOnboarded closes the hole that would otherwise exist on a daemon
// bound to something other than loopback: an unauthenticated endpoint that
// creates the first administrator is only safe while there is no first
// administrator.
func errAlreadyOnboarded() error {
	return apperr.New("AUTH_ALREADY_ONBOARDED").
		Causer("auth.Service.Onboarding").
		Msgf("this installation already has an account, so onboarding is closed").
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "sign in instead",
			Command: build.Name + " auth login",
		})
}

func errTokenNotFound(id string) error {
	return apperr.New("AUTH_TOKEN_NOT_FOUND").
		Causer("auth.Service").
		Msgf("no token %q on this account", id).
		Issue("tokenId", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the tokens of this account",
			Command: build.Name + " auth token list",
		})
}

// errLastSuperProtected keeps an installation from locking itself out. Removing
// or demoting the only administrator leaves nobody who can undo it.
func errLastSuperProtected(op string) error {
	return apperr.New("AUTH_LAST_SUPER_PROTECTED").
		Causer("auth.Service."+op).
		Msgf("this is the only administrator, and %s it would leave nobody able to undo that", op).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "promote another account to administrator first",
		})
}

func errStoreFailed(op string, cause error) error {
	return apperr.New("AUTH_STORE_FAILED").
		Causer("auth.Service." + op).
		Msgf("the account store could not be read or written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errInvalidRole(got string) error {
	return apperr.New("AUTH_INVALID_ROLE").
		Causer("auth.Service.SetRole").
		Msgf("%q is not a role", got).
		Issue("role", got).
		Issue("allowed", string(Super)+", "+string(Member)).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use super or member"})
}
