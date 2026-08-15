// Package fsconfig stores the installation configuration in ~/.aos/config.json.
package fsconfig

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sync"
	"time"

	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/domain/config"
)

// Store reads and replaces the configuration file.
//
// It rereads the file whenever its modification time changed, which reproduces
// the original's stateless service — hand edits are picked up without a restart
// — without paying a disk read on every single call.
type Store struct {
	path string

	mu      sync.Mutex
	cached  config.Config
	modTime time.Time
	size    int64
	loaded  bool
}

// New builds a store over the given file path.
func New(path string) *Store { return &Store{path: path} }

// FromPaths builds a store over the installation layout.
func FromPaths(p corecfg.Paths) *Store { return New(p.Config()) }

// Load returns the stored configuration, or the defaults when the file does not
// exist yet.
func (s *Store) Load(ctx context.Context) (config.Config, error) {
	if err := ctx.Err(); err != nil {
		return config.Config{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return config.Default(), nil
	case err != nil:
		return config.Config{}, err
	}

	if s.loaded && info.ModTime().Equal(s.modTime) && info.Size() == s.size {
		return s.cached, nil
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return config.Config{}, err
	}
	c := config.Default()
	if err := json.Unmarshal(raw, &c); err != nil {
		return config.Config{}, err
	}
	s.cached, s.modTime, s.size, s.loaded = c, info.ModTime(), info.Size(), true
	return c, nil
}

// Save writes the configuration atomically with 0600, because the file carries
// the API token, the session secret and every provider key (ADR-0010).
func (s *Store) Save(ctx context.Context, c config.Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := corecfg.WriteSecret(s.path, raw); err != nil {
		return err
	}
	s.loaded = false // force a reread, so the cache reflects what is on disk
	return nil
}
