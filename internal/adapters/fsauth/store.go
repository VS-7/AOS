// Package fsauth stores the accounts of the installation in ~/.aos/users.json.
package fsauth

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sync"

	"github.com/OWNER/aos/internal/core/apperr"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/domain/auth"
)

// Store reads and replaces the account file.
//
// The whole file is read and written at once because that is what it is: a
// handful of records whose invariants — one administrator at least, one account
// per username — hold across the set and not within a record.
type Store struct {
	path string
	mu   sync.Mutex
}

// New builds a store over a file path.
func New(path string) *Store { return &Store{path: path} }

// FromPaths builds a store over the installation layout.
func FromPaths(p corecfg.Paths) *Store { return New(p.Users()) }

// Load returns the accounts, or none when the file does not exist yet. A fresh
// installation has no accounts, which is the state onboarding exists for and
// not an error.
func (s *Store) Load(ctx context.Context) ([]auth.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errUnreadable(s.path, err)
	}
	var users []auth.User
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, errMalformed(s.path, err)
	}
	return users, nil
}

// Save replaces the file, atomically and with 0600.
//
// The permission is not incidental: the file holds password hashes and token
// hashes, and the original ships it as 0644 (defect #10). WriteSecret is the
// single place that decides what a secret file looks like on disk.
func (s *Store) Save(ctx context.Context, users []auth.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if users == nil {
		users = []auth.User{}
	}
	raw, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := corecfg.WriteSecret(s.path, raw); err != nil {
		return errUnwritable(s.path, err)
	}
	return nil
}

func errUnreadable(path string, cause error) error {
	return apperr.New("AUTH_STORE_UNREADABLE").
		Causer("fsauth.Store.Load").
		Msgf("cannot read the account file at %q", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errMalformed(path string, cause error) error {
	return apperr.New("AUTH_STORE_MALFORMED").
		Causer("fsauth.Store.Load").
		Msgf("%q is not a valid account file", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnwritable(path string, cause error) error {
	return apperr.New("AUTH_STORE_UNWRITABLE").
		Causer("fsauth.Store.Save").
		Msgf("cannot write the account file at %q", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
