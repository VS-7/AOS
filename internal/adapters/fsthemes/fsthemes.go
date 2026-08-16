// Package fsthemes stores the theme presets a user installs.
//
// One YAML file per theme under ~/.aos/themes/, which is the same shape the
// built-in ones have inside the binary. That is deliberate: a preset somebody
// likes can be copied out and sent to somebody else, and a built-in one can be
// copied in and edited, without either being converted first.
package fsthemes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/domain/theme"
)

// Store reads and writes presets in a directory.
type Store struct {
	dir string
	mu  sync.Mutex
}

// New builds a store over a directory.
func New(dir string) *Store { return &Store{dir: dir} }

// List reads every preset.
//
// A file that does not parse is skipped with the rest still returned: one
// hand-edited theme should not make the picker empty.
func (s *Store) List(_ context.Context) ([]theme.Theme, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []theme.Theme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		found, err := s.read(strings.TrimSuffix(e.Name(), ".yaml"))
		if err != nil || found == nil {
			continue
		}
		out = append(out, *found)
	}
	return out, nil
}

// Get reads one preset.
func (s *Store) Get(_ context.Context, id string) (*theme.Theme, error) {
	found, err := s.read(id)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, os.ErrNotExist
	}
	return found, nil
}

// Save writes one preset atomically.
func (s *Store) Save(_ context.Context, t theme.Theme) error {
	raw, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	path, err := s.pathOf(t.ID)
	if err != nil {
		return err
	}
	return atomicfs.WriteFile(path, raw, 0o644)
}

// Delete removes one preset. Removing one that is not there is not an error.
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.pathOf(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) read(id string) (*theme.Theme, error) {
	path, err := s.pathOf(id)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path) //nolint:gosec // the name is checked to be a single path element
	if os.IsNotExist(err) {
		return nil, nil //nolint:nilnil // absence is an answer
	}
	if err != nil {
		return nil, err
	}
	var t theme.Theme
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	// The file name is the identifier. A file whose front matter disagrees with
	// its name would be addressable under one and stored under the other.
	t.ID = strings.ToLower(strings.TrimSpace(filepath.Base(path)))
	t.ID = strings.TrimSuffix(t.ID, ".yaml")
	t.Builtin = false
	return &t, nil
}

// pathOf refuses an identifier that is not a single path element, so an install
// cannot write outside the theme directory.
func (s *Store) pathOf(id string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(id))
	if clean == "" || clean != filepath.Base(clean) || strings.ContainsAny(clean, `/\`) {
		return "", os.ErrInvalid
	}
	return filepath.Join(s.dir, clean+".yaml"), nil
}
