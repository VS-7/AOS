package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/build"
)

// TokenPrefix is what every credential starts with, so that a leaked string is
// recognisable as one — by a scanner, and by the person who spots it in a log.
var TokenPrefix = build.Name + "_"

// prefixLen is how much of a token is kept in the clear for identification. It
// is short enough to be useless on its own and long enough to distinguish the
// tokens one account holds.
const prefixLen = 8

// DefaultSessionTTL is how long a login lasts.
const DefaultSessionTTL = 30 * 24 * time.Hour

// Service is the identity aggregate.
type Service struct {
	store   Store
	clock   Clock
	ids     IDs
	secrets Secrets
}

// Deps is what the service is built from.
type Deps struct {
	Store   Store
	Clock   Clock
	IDs     IDs
	Secrets Secrets
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	secrets := d.Secrets
	if secrets == nil {
		secrets = randomSecrets{}
	}
	return &Service{store: d.Store, clock: d.Clock, ids: d.IDs, secrets: secrets}
}

// OnboardingInput creates the first account of an installation.
type OnboardingInput struct {
	Name     string
	Username string
	Email    string
	Password string
}

// OnboardingOutput carries the account and the one plain-text token it will
// ever have handed out.
type OnboardingOutput struct {
	User  Public
	Token string
}

// Onboarding creates the first administrator.
//
// It refuses once an account exists. An unauthenticated endpoint that creates
// an administrator is only safe while there is no administrator to create — and
// a daemon that was later exposed beyond loopback would otherwise be one
// request away from a second one.
func (s *Service) Onboarding(ctx context.Context, in OnboardingInput) (OnboardingOutput, error) {
	users, err := s.store.Load(ctx)
	if err != nil {
		return OnboardingOutput{}, errStoreFailed("Onboarding", err)
	}
	if len(users) > 0 {
		return OnboardingOutput{}, errAlreadyOnboarded()
	}
	if err := ValidatePassword(in.Password); err != nil {
		return OnboardingOutput{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return OnboardingOutput{}, err
	}
	now := s.clock.Now()
	user := User{
		ID:           s.ids.New(),
		Name:         strings.TrimSpace(in.Name),
		Username:     normalizeUsername(in.Username),
		Email:        normalizeEmail(in.Email),
		PasswordHash: hash,
		Role:         Super,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	plain, token, err := s.mintToken("initial", nil)
	if err != nil {
		return OnboardingOutput{}, err
	}
	user.Tokens = append(user.Tokens, token)

	if err := s.store.Save(ctx, []User{user}); err != nil {
		return OnboardingOutput{}, errStoreFailed("Onboarding", err)
	}
	return OnboardingOutput{User: user.ToPublic(), Token: plain}, nil
}

// LoginInput is a username or email, and a password.
type LoginInput struct {
	Identifier string
	Password   string
}

// Login verifies a password and issues a session token.
//
// It always performs a password verification, even when no account matched, so
// that the time it takes does not reveal whether the account exists.
func (s *Service) Login(ctx context.Context, in LoginInput) (Session, error) {
	users, err := s.store.Load(ctx)
	if err != nil {
		return Session{}, errStoreFailed("Login", err)
	}

	identifier := normalizeUsername(in.Identifier)
	found := -1
	for i, u := range users {
		if u.Username == identifier || u.Email == identifier {
			found = i
			break
		}
	}

	// The decoy is a real hash of a real password, so the work done on the
	// miss path matches the work done on the hit path.
	candidate := decoyHash
	if found >= 0 {
		candidate = users[found].PasswordHash
	}
	ok := VerifyPassword(candidate, in.Password)
	if found < 0 || !ok {
		return Session{}, errInvalidCredentials()
	}

	plain, token, err := s.mintToken("session", ptr(s.clock.Now().Add(DefaultSessionTTL)))
	if err != nil {
		return Session{}, err
	}
	users[found].Tokens = append(users[found].Tokens, token)
	users[found].UpdatedAt = s.clock.Now()
	if err := s.store.Save(ctx, users); err != nil {
		return Session{}, errStoreFailed("Login", err)
	}
	return Session{UserID: users[found].ID, Token: plain, ExpiresAt: *token.ExpiresAt}, nil
}

// Authenticate resolves a bearer credential to an account.
//
// The token is hashed and compared in constant time, and the hash is computed
// whether or not a candidate exists, so a caller cannot learn from the timing
// whether a token is merely wrong or is one character off.
func (s *Service) Authenticate(ctx context.Context, bearer string) (*User, error) {
	presented := strings.TrimSpace(bearer)
	if presented == "" {
		return nil, errUnauthenticated()
	}
	users, err := s.store.Load(ctx)
	if err != nil {
		return nil, errStoreFailed("Authenticate", err)
	}

	want := hashToken(presented)
	now := s.clock.Now()

	var match *User
	var matchToken int
	for i := range users {
		for j := range users[i].Tokens {
			t := users[i].Tokens[j]
			if subtle.ConstantTimeCompare([]byte(t.Hash), []byte(want)) != 1 {
				continue
			}
			if !t.Active(now) {
				continue
			}
			match, matchToken = &users[i], j
		}
	}
	if match == nil {
		return nil, errUnauthenticated()
	}

	// Recording last use is a write on a read path. It is done because a token
	// nobody can tell the age of is a token nobody will ever revoke, and it is
	// best-effort because failing an authenticated request over an audit field
	// would be the wrong trade.
	match.Tokens[matchToken].LastUsed = &now
	if err := s.store.Save(ctx, users); err != nil {
		out := *match
		return &out, nil
	}
	out := *match
	return &out, nil
}

// IssueTokenInput names a new credential for an account.
type IssueTokenInput struct {
	UserID    string
	Name      string
	ExpiresAt *time.Time
}

// IssueToken mints a credential and returns its plain value once.
func (s *Service) IssueToken(ctx context.Context, in IssueTokenInput) (Token, string, error) {
	users, err := s.store.Load(ctx)
	if err != nil {
		return Token{}, "", errStoreFailed("IssueToken", err)
	}
	idx := indexOf(users, in.UserID)
	if idx < 0 {
		return Token{}, "", errUserNotFound(in.UserID)
	}

	plain, token, err := s.mintToken(in.Name, in.ExpiresAt)
	if err != nil {
		return Token{}, "", err
	}
	users[idx].Tokens = append(users[idx].Tokens, token)
	users[idx].UpdatedAt = s.clock.Now()
	if err := s.store.Save(ctx, users); err != nil {
		return Token{}, "", errStoreFailed("IssueToken", err)
	}
	return token, plain, nil
}

// RevokeToken marks a credential as no longer valid, keeping the record.
//
// The record stays because a revoked token is evidence: when it was created,
// when it was last used, and when it stopped working are the three facts
// someone investigating a leak needs.
func (s *Service) RevokeToken(ctx context.Context, userID, tokenID string) error {
	users, err := s.store.Load(ctx)
	if err != nil {
		return errStoreFailed("RevokeToken", err)
	}
	idx := indexOf(users, userID)
	if idx < 0 {
		return errUserNotFound(userID)
	}
	now := s.clock.Now()
	for j := range users[idx].Tokens {
		if users[idx].Tokens[j].ID != tokenID {
			continue
		}
		users[idx].Tokens[j].RevokedAt = &now
		users[idx].UpdatedAt = now
		if err := s.store.Save(ctx, users); err != nil {
			return errStoreFailed("RevokeToken", err)
		}
		return nil
	}
	return errTokenNotFound(tokenID)
}

// RotateToken issues a replacement and expires the old one after a grace
// period, so that a running process keeps working while it is updated.
func (s *Service) RotateToken(ctx context.Context, userID, tokenID string, grace time.Duration) (Token, string, error) {
	users, err := s.store.Load(ctx)
	if err != nil {
		return Token{}, "", errStoreFailed("RotateToken", err)
	}
	idx := indexOf(users, userID)
	if idx < 0 {
		return Token{}, "", errUserNotFound(userID)
	}
	old := -1
	for j := range users[idx].Tokens {
		if users[idx].Tokens[j].ID == tokenID {
			old = j
		}
	}
	if old < 0 {
		return Token{}, "", errTokenNotFound(tokenID)
	}

	now := s.clock.Now()
	plain, token, err := s.mintToken(users[idx].Tokens[old].Name, users[idx].Tokens[old].ExpiresAt)
	if err != nil {
		return Token{}, "", err
	}
	deadline := now.Add(grace)
	users[idx].Tokens[old].ExpiresAt = &deadline
	users[idx].Tokens = append(users[idx].Tokens, token)
	users[idx].UpdatedAt = now
	if err := s.store.Save(ctx, users); err != nil {
		return Token{}, "", errStoreFailed("RotateToken", err)
	}
	return token, plain, nil
}

// ChangePasswordInput replaces an account's password.
type ChangePasswordInput struct {
	UserID  string
	Current string
	New     string
}

// ChangePassword verifies the current password before setting the new one.
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	users, err := s.store.Load(ctx)
	if err != nil {
		return errStoreFailed("ChangePassword", err)
	}
	idx := indexOf(users, in.UserID)
	if idx < 0 {
		return errUserNotFound(in.UserID)
	}
	if !VerifyPassword(users[idx].PasswordHash, in.Current) {
		return errInvalidCredentials()
	}
	if err := ValidatePassword(in.New); err != nil {
		return err
	}
	hash, err := HashPassword(in.New)
	if err != nil {
		return err
	}
	users[idx].PasswordHash = hash
	users[idx].UpdatedAt = s.clock.Now()
	if err := s.store.Save(ctx, users); err != nil {
		return errStoreFailed("ChangePassword", err)
	}
	return nil
}

// Users lists the accounts, without anything that authenticates.
func (s *Service) Users(ctx context.Context) ([]Public, error) {
	users, err := s.store.Load(ctx)
	if err != nil {
		return nil, errStoreFailed("Users", err)
	}
	out := make([]Public, 0, len(users))
	for _, u := range users {
		out = append(out, u.ToPublic())
	}
	return out, nil
}

// SetRole changes an account's instance role.
func (s *Service) SetRole(ctx context.Context, userID string, role Role) error {
	if !role.Valid() {
		return errInvalidRole(string(role))
	}
	users, err := s.store.Load(ctx)
	if err != nil {
		return errStoreFailed("SetRole", err)
	}
	idx := indexOf(users, userID)
	if idx < 0 {
		return errUserNotFound(userID)
	}
	if users[idx].Role == Super && role != Super && countSupers(users) == 1 {
		return errLastSuperProtected("demoting")
	}
	users[idx].Role = role
	users[idx].UpdatedAt = s.clock.Now()
	if err := s.store.Save(ctx, users); err != nil {
		return errStoreFailed("SetRole", err)
	}
	return nil
}

// Delete removes an account.
func (s *Service) Delete(ctx context.Context, userID string) error {
	users, err := s.store.Load(ctx)
	if err != nil {
		return errStoreFailed("Delete", err)
	}
	idx := indexOf(users, userID)
	if idx < 0 {
		return errUserNotFound(userID)
	}
	if users[idx].Role == Super && countSupers(users) == 1 {
		return errLastSuperProtected("removing")
	}
	users = append(users[:idx], users[idx+1:]...)
	if err := s.store.Save(ctx, users); err != nil {
		return errStoreFailed("Delete", err)
	}
	return nil
}

// mintToken produces the plain value and the record that will be stored.
func (s *Service) mintToken(name string, expires *time.Time) (string, Token, error) {
	plain, err := s.secrets.NewToken()
	if err != nil {
		return "", Token{}, err
	}
	if !strings.HasPrefix(plain, TokenPrefix) {
		plain = TokenPrefix + plain
	}
	prefix := plain
	if len(prefix) > len(TokenPrefix)+prefixLen {
		prefix = plain[:len(TokenPrefix)+prefixLen]
	}
	return plain, Token{
		ID:        s.ids.New(),
		Name:      name,
		Hash:      hashToken(plain),
		Prefix:    prefix,
		ExpiresAt: expires,
		CreatedAt: s.clock.Now(),
	}, nil
}

// hashToken is SHA-256 rather than argon2id, and deliberately.
//
// A token is 256 bits of entropy this system generated; it is not guessable, so
// the slow hash that protects a human-chosen password buys nothing here and
// would put a 64 MiB allocation on the path of every authenticated request.
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// randomSecrets is the production generator: 32 bytes from the system CSPRNG.
type randomSecrets struct{}

func (randomSecrets) NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", errors.New("the system entropy source is unavailable: " + err.Error())
	}
	return TokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// decoyHash is a real argon2id hash, used so that a login attempt against an
// account that does not exist costs the same as one against an account that
// does. It is computed once, at first use.
var decoyHash = func() string {
	h, err := HashPassword("a password that is not anybody's, for constant time")
	if err != nil {
		return ""
	}
	return h
}()

func indexOf(users []User, id string) int {
	for i := range users {
		if users[i].ID == id {
			return i
		}
	}
	return -1
}

func countSupers(users []User) int {
	n := 0
	for _, u := range users {
		if u.Role == Super {
			n++
		}
	}
	return n
}

func ptr[T any](v T) *T { return &v }
