package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/auth"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// goodPassword satisfies the policy: long, and not in the list.
const goodPassword = "ruivo bicicleta trovoada 42"

type fakeStore struct {
	mu      sync.Mutex
	users   []auth.User
	saves   int
	loadErr error
	saveErr error
}

func (s *fakeStore) Load(context.Context) ([]auth.User, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A copy, so a caller mutating the result cannot reach the store — the
	// file-backed implementation cannot be mutated that way either.
	raw, _ := json.Marshal(s.users)
	var out []auth.User
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

func (s *fakeStore) Save(_ context.Context, users []auth.User) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves++
	s.users = users
	return nil
}

func newService(t *testing.T) (*auth.Service, *fakeStore) {
	t.Helper()
	store := &fakeStore{}
	svc := auth.NewService(auth.Deps{
		Store: store,
		Clock: clockx.Fixed{At: refTime},
		IDs:   &ids.Sequence{Prefix: "u"},
	})
	return svc, store
}

func ctx() context.Context { return context.Background() }

func onboard(t *testing.T, svc *auth.Service) auth.OnboardingOutput {
	t.Helper()
	out, err := svc.Onboarding(ctx(), auth.OnboardingInput{
		Name: "Vitor", Username: "vitor", Email: "vitor@example.test", Password: goodPassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestAPasswordOfElevenIsRejected pins the divergence: the original accepts
// six, for an account that can run shell commands on the machine.
func TestAPasswordOfElevenIsRejected(t *testing.T) {
	if err := auth.ValidatePassword("12345678901"); !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("eleven characters: error = %v", err)
	}
	if err := auth.ValidatePassword("123456789012"); err == nil {
		t.Fatal("twelve digits should still be refused by the breach list")
	}
	if err := auth.ValidatePassword(goodPassword); err != nil {
		t.Fatalf("a long passphrase was refused: %v", err)
	}
}

func TestABreachedPasswordIsRejectedWhateverItsLength(t *testing.T) {
	for _, p := range []string{
		"correct horse battery staple",
		"Correct Horse Battery Staple",
		"  password123456  ",
		"qwertyuiop123",
	} {
		if err := auth.ValidatePassword(p); !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("%q was accepted", p)
		}
	}
}

// TestTheBreachListIsNotEmpty guards against the policy silently becoming a
// no-op: a list that gets emptied still reads as protection.
func TestTheBreachListIsNotEmpty(t *testing.T) {
	if n := auth.BreachedCount(); n < 50 {
		t.Fatalf("the embedded list holds %d passwords", n)
	}
}

// TestTheHashCarriesItsParameters is what lets the cost be raised later without
// locking everyone out of the hashes written before the change.
func TestTheHashCarriesItsParameters(t *testing.T) {
	hash, err := auth.HashPassword(goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=2,p=1$") {
		t.Fatalf("hash = %q, want the original's parameters", hash)
	}
	if !auth.VerifyPassword(hash, goodPassword) {
		t.Fatal("the hash does not verify its own password")
	}
	if auth.VerifyPassword(hash, goodPassword+" ") {
		t.Fatal("a different password verified")
	}
}

func TestTheSaltIsFreshEveryTime(t *testing.T) {
	a, err := auth.HashPassword(goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	b, err := auth.HashPassword(goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two hashes of the same password are identical — the salt is not random")
	}
}

func TestAMalformedHashDoesNotVerify(t *testing.T) {
	for _, bad := range []string{
		"", "not a hash", "$argon2id$", "$argon2i$v=19$m=1,t=1,p=1$AAAA$AAAA",
		"$argon2id$v=99$m=1,t=1,p=1$AAAA$AAAA",
		"$argon2id$v=19$m=0,t=0,p=0$AAAA$AAAA",
	} {
		if auth.VerifyPassword(bad, goodPassword) {
			t.Errorf("%q verified", bad)
		}
	}
}

func TestOnboardingCreatesTheAdministratorAndOneToken(t *testing.T) {
	svc, store := newService(t)
	out := onboard(t, svc)

	if out.User.Role != auth.Super {
		t.Errorf("role = %q, want the first account to administer", out.User.Role)
	}
	if !strings.HasPrefix(out.Token, "aos_") {
		t.Errorf("token = %q, want a recognisable prefix", out.Token)
	}
	if len(store.users) != 1 || len(store.users[0].Tokens) != 1 {
		t.Fatalf("store = %+v", store.users)
	}
}

// TestTheDiskNeverHoldsTheToken is the divergence that matters most here: the
// original writes the value in clear into users.json, and again into the MCP
// client configuration.
func TestTheDiskNeverHoldsTheToken(t *testing.T) {
	svc, store := newService(t)
	out := onboard(t, svc)

	raw, err := json.Marshal(store.users)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), out.Token) {
		t.Fatalf("the stored record contains the plain token:\n%s", raw)
	}
	// The prefix is there on purpose: enough to tell two tokens apart, useless
	// on its own.
	if !strings.Contains(string(raw), store.users[0].Tokens[0].Prefix) {
		t.Error("the prefix should be stored, for identification")
	}
	if len(store.users[0].Tokens[0].Prefix) >= len(out.Token) {
		t.Error("the prefix is the whole token")
	}
}

func TestTheDiskNeverHoldsThePassword(t *testing.T) {
	svc, store := newService(t)
	onboard(t, svc)

	raw, err := json.Marshal(store.users)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), goodPassword) {
		t.Fatalf("the stored record contains the password:\n%s", raw)
	}
}

// TestOnboardingIsClosedOnceThereIsAnAccount: an unauthenticated endpoint that
// creates an administrator is only safe while there is none.
func TestOnboardingIsClosedOnceThereIsAnAccount(t *testing.T) {
	svc, _ := newService(t)
	onboard(t, svc)

	_, err := svc.Onboarding(ctx(), auth.OnboardingInput{
		Name: "Someone else", Username: "other", Email: "other@example.test", Password: goodPassword,
	})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
}

func TestOnboardingRefusesAWeakPassword(t *testing.T) {
	svc, store := newService(t)
	_, err := svc.Onboarding(ctx(), auth.OnboardingInput{
		Name: "Vitor", Username: "vitor", Email: "v@example.test", Password: "short",
	})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
	if store.saves != 0 {
		t.Error("a rejected onboarding must not write")
	}
}

func TestLoginIssuesASessionToken(t *testing.T) {
	svc, _ := newService(t)
	onboard(t, svc)

	session, err := svc.Login(ctx(), auth.LoginInput{Identifier: "VITOR", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || session.UserID == "" {
		t.Fatalf("session = %+v", session)
	}
	if !session.ExpiresAt.After(refTime) {
		t.Error("a session with no future expiry is not a session")
	}

	// And it authenticates.
	user, err := svc.Authenticate(ctx(), session.Token)
	if err != nil || user.Username != "vitor" {
		t.Fatalf("user = %+v, err = %v", user, err)
	}
}

// TestAWrongPasswordAndAMissingAccountFailAlike: distinguishing them turns the
// login endpoint into a way to enumerate accounts.
func TestAWrongPasswordAndAMissingAccountFailAlike(t *testing.T) {
	svc, _ := newService(t)
	onboard(t, svc)

	_, wrongPassword := svc.Login(ctx(), auth.LoginInput{Identifier: "vitor", Password: "not the password at all"})
	_, noAccount := svc.Login(ctx(), auth.LoginInput{Identifier: "nobody", Password: goodPassword})

	if wrongPassword == nil || noAccount == nil {
		t.Fatal("both should fail")
	}
	if wrongPassword.Error() != noAccount.Error() {
		t.Fatalf("the two failures are distinguishable:\n  %v\n  %v", wrongPassword, noAccount)
	}
	if !errors.Is(wrongPassword, apperr.ErrUnauthorized) {
		t.Errorf("error = %v", wrongPassword)
	}
}

func TestAnUnknownTokenIsRefused(t *testing.T) {
	svc, _ := newService(t)
	onboard(t, svc)

	for _, bearer := range []string{"", "   ", "aos_not-a-real-token", "garbage"} {
		if _, err := svc.Authenticate(ctx(), bearer); !errors.Is(err, apperr.ErrUnauthorized) {
			t.Errorf("%q: error = %v", bearer, err)
		}
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	store := &fakeStore{}
	clock := &steppingClock{at: refTime}
	svc := auth.NewService(auth.Deps{Store: store, Clock: clock, IDs: &ids.Sequence{Prefix: "u"}})

	out := onboard(t, svc)
	user, err := svc.Authenticate(ctx(), out.Token)
	if err != nil {
		t.Fatal(err)
	}

	token, plain, err := svc.IssueToken(ctx(), auth.IssueTokenInput{
		UserID: user.ID, Name: "short-lived", ExpiresAt: ptr(refTime.Add(time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx(), plain); err != nil {
		t.Fatalf("the token should work before it expires: %v", err)
	}

	clock.advance(2 * time.Hour)
	if _, err := svc.Authenticate(ctx(), plain); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("an expired token authenticated: %v", err)
	}
	if token.ExpiresAt == nil {
		t.Error("the record should carry the expiry")
	}
}

func TestARevokedTokenIsRefusedButKeptAsEvidence(t *testing.T) {
	svc, store := newService(t)
	out := onboard(t, svc)
	user, err := svc.Authenticate(ctx(), out.Token)
	if err != nil {
		t.Fatal(err)
	}
	tokenID := store.users[0].Tokens[0].ID

	if err := svc.RevokeToken(ctx(), user.ID, tokenID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx(), out.Token); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("a revoked token authenticated: %v", err)
	}
	// The record survives: when it was created, last used and revoked are the
	// three facts someone investigating a leak needs.
	if len(store.users[0].Tokens) != 1 || store.users[0].Tokens[0].RevokedAt == nil {
		t.Fatalf("tokens = %+v", store.users[0].Tokens)
	}
}

// TestRotationKeepsTheOldTokenWorkingDuringTheGrace is the property that makes
// rotation usable: a running process keeps working while it is updated.
func TestRotationKeepsTheOldTokenWorkingDuringTheGrace(t *testing.T) {
	store := &fakeStore{}
	clock := &steppingClock{at: refTime}
	svc := auth.NewService(auth.Deps{Store: store, Clock: clock, IDs: &ids.Sequence{Prefix: "u"}})

	out := onboard(t, svc)
	user, err := svc.Authenticate(ctx(), out.Token)
	if err != nil {
		t.Fatal(err)
	}
	oldID := store.users[0].Tokens[0].ID

	_, replacement, err := svc.RotateToken(ctx(), user.ID, oldID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if replacement == out.Token {
		t.Fatal("rotation returned the same value")
	}
	for _, token := range []string{out.Token, replacement} {
		if _, err := svc.Authenticate(ctx(), token); err != nil {
			t.Errorf("during the grace period both should work: %v", err)
		}
	}

	clock.advance(2 * time.Hour)
	if _, err := svc.Authenticate(ctx(), out.Token); err == nil {
		t.Error("the old token outlived its grace period")
	}
	if _, err := svc.Authenticate(ctx(), replacement); err != nil {
		t.Errorf("the replacement stopped working: %v", err)
	}
}

func TestLastUseIsRecorded(t *testing.T) {
	svc, store := newService(t)
	out := onboard(t, svc)
	if store.users[0].Tokens[0].LastUsed != nil {
		t.Fatal("a token that was never presented has no last use")
	}
	if _, err := svc.Authenticate(ctx(), out.Token); err != nil {
		t.Fatal(err)
	}
	if store.users[0].Tokens[0].LastUsed == nil {
		t.Fatal("a token nobody can date is a token nobody will revoke")
	}
}

func TestChangePasswordVerifiesTheCurrentOne(t *testing.T) {
	svc, _ := newService(t)
	out := onboard(t, svc)
	user, err := svc.Authenticate(ctx(), out.Token)
	if err != nil {
		t.Fatal(err)
	}

	err = svc.ChangePassword(ctx(), auth.ChangePasswordInput{
		UserID: user.ID, Current: "wrong", New: "outra frase longa aqui 99",
	})
	if !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("error = %v", err)
	}
	if err := svc.ChangePassword(ctx(), auth.ChangePasswordInput{
		UserID: user.ID, Current: goodPassword, New: "outra frase longa aqui 99",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(ctx(), auth.LoginInput{
		Identifier: "vitor", Password: "outra frase longa aqui 99",
	}); err != nil {
		t.Fatalf("the new password does not work: %v", err)
	}
}

func TestChangePasswordAppliesThePolicy(t *testing.T) {
	svc, _ := newService(t)
	out := onboard(t, svc)
	user, _ := svc.Authenticate(ctx(), out.Token)

	err := svc.ChangePassword(ctx(), auth.ChangePasswordInput{
		UserID: user.ID, Current: goodPassword, New: "password123456",
	})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}

// TestTheLastAdministratorCannotBeRemovedOrDemoted keeps an installation from
// locking itself out: there would be nobody left able to undo it.
func TestTheLastAdministratorCannotBeRemovedOrDemoted(t *testing.T) {
	svc, store := newService(t)
	onboard(t, svc)
	id := store.users[0].ID

	if err := svc.SetRole(ctx(), id, auth.Member); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("demote: error = %v", err)
	}
	if err := svc.Delete(ctx(), id); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("delete: error = %v", err)
	}

	// With a second administrator, both are allowed.
	store.users = append(store.users, auth.User{ID: "u-second", Username: "second", Role: auth.Super})
	if err := svc.SetRole(ctx(), id, auth.Member); err != nil {
		t.Fatalf("demote with a second administrator: %v", err)
	}
}

// TestUsersCarryNothingThatAuthenticates: the listing is handed to callers that
// have no business holding a hash.
func TestUsersCarryNothingThatAuthenticates(t *testing.T) {
	svc, _ := newService(t)
	onboard(t, svc)

	list, err := svc.Users(ctx())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password", "hash", "token", "argon2"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Errorf("the public listing mentions %q:\n%s", forbidden, raw)
		}
	}
}

// TestGetReturnsThePublicProjection: the login/session HTTP surface needs to
// answer "who is this" without ever handing back a hash.
func TestGetReturnsThePublicProjection(t *testing.T) {
	svc, store := newService(t)
	onboard(t, svc)
	id := store.users[0].ID

	got, err := svc.Get(ctx(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Username != "vitor" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetOnAMissingAccountIsNotFound(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Get(ctx(), "nobody"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

// TestRevokeByTokenLogsOutOnlyTheSessionPresented: logging out one device must
// not silently end every other session the account holds.
func TestRevokeByTokenLogsOutOnlyTheSessionPresented(t *testing.T) {
	svc, _ := newService(t)
	onboard(t, svc)
	first, err := svc.Login(ctx(), auth.LoginInput{Identifier: "vitor", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Login(ctx(), auth.LoginInput{Identifier: "vitor", Password: goodPassword})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.RevokeByToken(ctx(), first.Token); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Authenticate(ctx(), first.Token); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("the revoked session should no longer authenticate: %v", err)
	}
	if _, err := svc.Authenticate(ctx(), second.Token); err != nil {
		t.Fatalf("the other session should still work: %v", err)
	}
}

// TestRevokeByTokenIsIdempotent: a caller that is already logged out, or was
// never logged in, gets the same quiet success — logout is not a place to
// leak whether a value was ever a real credential.
func TestRevokeByTokenIsIdempotent(t *testing.T) {
	svc, _ := newService(t)
	if err := svc.RevokeByToken(ctx(), ""); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := svc.RevokeByToken(ctx(), "not-a-real-token"); err != nil {
		t.Fatalf("garbage: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

// steppingClock lets a test move time forward on purpose.
type steppingClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *steppingClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}
