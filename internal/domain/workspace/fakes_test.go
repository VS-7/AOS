package workspace_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// The fakes this package's tests run on. They are here rather than in
// internal/domain/fakes because no other aggregate consumes these ports, and a
// fake shared by one caller is just indirection.

// fakeStore is the workspace registry in memory.
type fakeStore struct {
	mu      sync.Mutex
	items   map[string]workspace.Workspace
	saves   int
	saveErr error
	listErr error
}

func newStore() *fakeStore { return &fakeStore{items: map[string]workspace.Workspace{}} }

func (s *fakeStore) Get(_ context.Context, id string) (*workspace.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.items[id]
	if !ok {
		return nil, apperr.New("TEST_NOT_FOUND").Status(apperr.StatusNotFound).Msgf("no workspace %q", id)
	}
	return &w, nil
}

func (s *fakeStore) List(context.Context) ([]workspace.Workspace, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]workspace.Workspace, 0, len(s.items))
	for _, w := range s.items {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *fakeStore) Save(_ context.Context, w *workspace.Workspace) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves++
	s.items[w.ID] = *w
	return nil
}

func (s *fakeStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

// fakeGit records what was asked of it.
type fakeGit struct {
	repos    map[string]bool
	origin   string
	initErr  error
	checkErr error
	inits    int
}

func newGit() *fakeGit { return &fakeGit{repos: map[string]bool{}} }

func (g *fakeGit) IsRepository(_ context.Context, dir string) (bool, error) {
	if g.checkErr != nil {
		return false, g.checkErr
	}
	return g.repos[dir], nil
}

func (g *fakeGit) Init(_ context.Context, dir string) error {
	if g.initErr != nil {
		return g.initErr
	}
	g.inits++
	g.repos[dir] = true
	return nil
}

func (g *fakeGit) OriginURL(_ context.Context, _ string) (string, error) {
	if g.origin == "" {
		return "", errors.New("no origin remote")
	}
	return g.origin, nil
}

// fakeSeeder records the orchestrator that was asked for, so a test can assert
// on the instruction document without a real agent repository.
type fakeSeeder struct {
	// seeded is the most recent seed, for the tests that assert on it directly.
	seeded  workspace.OrchestratorSeed
	byRoot  map[string]workspace.OrchestratorSeed
	present map[string]string
	err     error
	calls   int
}

func newSeeder() *fakeSeeder {
	return &fakeSeeder{byRoot: map[string]workspace.OrchestratorSeed{}, present: map[string]string{}}
}

func (s *fakeSeeder) FindOrchestrator(_ context.Context, root string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	id, ok := s.present[root]
	return id, ok, nil
}

func (s *fakeSeeder) SeedOrchestrator(_ context.Context, in workspace.OrchestratorSeed) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.calls++
	s.seeded = in
	s.byRoot[in.Root] = in
	s.present[in.Root] = in.ID
	return in.ID, nil
}

// fakeSurveyor answers the inventory question from a fixed table.
type fakeSurveyor struct {
	byRoot map[string][]workspace.CollectionSummary
	err    error
}

func (s fakeSurveyor) Survey(_ context.Context, root string) ([]workspace.CollectionSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byRoot[root], nil
}

// hasLine reports whether text contains a line equal to want, so an assertion
// on a generated document does not depend on the lines around it.
func hasLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// writeFile seeds a file the user is presumed to have written.
func writeFile(t *testing.T, fs *fakes.Files, path, content string) {
	t.Helper()
	if err := fs.WriteFile(context.Background(), path, content); err != nil {
		t.Fatal(err)
	}
}

// appendFile adds to a file the user is presumed to have edited by hand.
func appendFile(t *testing.T, fs *fakes.Files, path, extra string) {
	t.Helper()
	current, _ := fs.File(path)
	writeFile(t, fs, path, current+extra)
}
