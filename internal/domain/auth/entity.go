// Package auth owns accounts, sessions and API tokens.
//
// It is the boundary an agent does not cross. There are no tools here and there
// never will be: an agent operates the domain, not the identity that authorises
// it. That is inherited from the original and it is one of the things the
// original got right.
package auth

import (
	"strings"
	"time"
)

// Role is the instance-level role, distinct from the workspace-level one. A
// guard compares both: being an owner of a workspace does not make you an
// administrator of the installation.
type Role string

const (
	// Super administers the installation and every workspace in it.
	Super Role = "super"
	// Member has whatever access their workspace memberships grant.
	Member Role = "member"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r == Super || r == Member }

// User is one account.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`

	// PasswordHash is argon2id, never plain text. The parameters are the
	// original's, which are sound.
	PasswordHash string `json:"password" secret:"true"`

	Role Role `json:"role"`

	Tokens []Token `json:"tokens"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Token is an API credential, stored hashed.
//
// The original writes the token in clear into users.json and again into the MCP
// client configuration. Here the disk holds only the hash: the value is
// returned once, at creation, and cannot be recovered afterwards. What is lost
// is the ability to re-read a token you mislaid; what is gained is that a
// readable state file is not a set of live credentials.
type Token struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	Hash string `json:"hash" secret:"true"`

	// Prefix is the leading, non-secret part of the value, kept so a person can
	// tell two tokens apart in a list without either being reconstructible.
	Prefix string `json:"prefix"`

	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	LastUsed  *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// Active reports whether a token may still authenticate at the given time.
func (t Token) Active(now time.Time) bool {
	if t.RevokedAt != nil && !now.Before(*t.RevokedAt) {
		return false
	}
	if t.ExpiresAt != nil && !now.Before(*t.ExpiresAt) {
		return false
	}
	return true
}

// Session is what a successful login yields.
type Session struct {
	UserID    string    `json:"userId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Public is the projection safe to hand to any caller: no hash, no token, no
// anything that would let the holder act as this user.
type Public struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     Role   `json:"role"`
}

// ToPublic strips everything that authenticates.
func (u User) ToPublic() Public {
	return Public{ID: u.ID, Name: u.Name, Username: u.Username, Email: u.Email, Role: u.Role}
}

// IsSuper reports whether the account administers the installation.
func (u User) IsSuper() bool { return u.Role == Super }

// normalizeUsername lowercases and trims. Usernames are compared, and a
// comparison that depends on capitalisation lets two accounts claim one name.
func normalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normalizeEmail does the same for addresses, for the same reason.
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
