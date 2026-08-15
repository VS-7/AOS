package auth

import (
	"context"
	"time"
)

// Store persists the accounts of the installation.
//
// It is the whole file, read and replaced, because that is what users.json is:
// a handful of records that change rarely and must be consistent with each
// other. A per-record store would let two writes leave two administrators where
// the invariant says one.
type Store interface {
	Load(ctx context.Context) ([]User, error)
	Save(ctx context.Context, users []User) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// IDs hands out account and token identifiers.
type IDs interface{ New() string }

// Secrets generates the random part of a token. It is a port so a test can
// predict the value it will have to present back.
type Secrets interface {
	// NewToken returns a fresh credential in plain text. It is the only moment
	// the value exists outside the caller's hands.
	NewToken() (string, error)
}
