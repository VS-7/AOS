package collections_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
)

// TestErrorVocabularyIsShared: the engine owns the codes so that a second
// adapter cannot invent its own and break every caller that branches on
// behaviour. Each condition must map to one sentinel and, when actionable,
// carry a CTA.
func TestErrorVocabularyIsShared(t *testing.T) {
	key := collections.Key{"agent": "luara", "id": "m1"}
	cases := []struct {
		name     string
		err      error
		sentinel error
		wantCTA  bool
	}{
		{"not found", collections.NotFoundError("memories", key), apperr.ErrNotFound, true},
		{"already exists", collections.AlreadyExistsError("memories", key), apperr.ErrConflict, true},
		{
			"conflict",
			collections.ConflictError("memories", "a.md",
				collections.Version{Size: 1}, collections.Version{Size: 2}),
			apperr.ErrConflict, true,
		},
		{"io", collections.IOError("read", "a.md", errors.New("disk")), apperr.ErrInternal, false},
		{"outside root", collections.OutsideRootError("../x", "/root"), apperr.ErrForbidden, true},
		{"not owned", collections.NotOwnedError("memories", "a.md"), apperr.ErrInternal, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !errors.Is(c.err, c.sentinel) {
				t.Errorf("does not unwrap to the expected sentinel: %v", c.err)
			}
			e, ok := apperr.As(c.err)
			if !ok {
				t.Fatalf("not an app error: %v", c.err)
			}
			if e.CauserName == "" {
				t.Error("no causer")
			}
			if c.wantCTA && len(e.Actions) == 0 {
				t.Error("an actionable error must carry a CTA")
			}
			if !strings.HasPrefix(e.Code, "AOS_COLLECTION_") {
				t.Errorf("code = %q", e.Code)
			}
		})
	}
}

// TestTheRecordKeyIsNotCalledKey: "key" is the name of a provider credential in
// the configuration, and an Issue field named that way trips the secret scan.
func TestTheRecordKeyIsNotCalledKey(t *testing.T) {
	e, _ := apperr.As(collections.NotFoundError("memories", collections.Key{"id": "m1"}))
	if _, bad := e.Issues["key"]; bad {
		t.Error(`the record identifier must not be reported under "key"`)
	}
	if got := e.Issues["record"]; got != "id=m1" {
		t.Errorf("record = %v", got)
	}
}

func TestFieldOfReadsByJSONName(t *testing.T) {
	v := &probe{Title: "t", Confidence: 0.9}
	if got, ok := collections.FieldOf(v, "title"); !ok || got != "t" {
		t.Errorf("title = %v %v", got, ok)
	}
	if got, ok := collections.FieldOf(v, "confidence"); !ok || got != 0.9 {
		t.Errorf("confidence = %v %v", got, ok)
	}
	if _, ok := collections.FieldOf(v, "nope"); ok {
		t.Error("an unknown field must report ok=false")
	}
}

func TestModelWritePatternFailsWithoutOne(t *testing.T) {
	m := collections.Model[probe]{
		Name:     "wildcards-only",
		Patterns: []*collections.Pattern{collections.MustCompile(".aos/skills/*/x/{id}.md")},
	}
	if _, err := m.WritePattern(); err == nil {
		t.Fatal("a collection with only wildcard patterns cannot be written to")
	}
}

// TestInterProcessLockUsesTheLockDirectory: the advisory lock file must never
// land in the user's Git repository next to the record.
func TestInterProcessLockUsesTheLockDirectory(t *testing.T) {
	repoDir := t.TempDir()
	lockDir := filepath.Join(t.TempDir(), "locks")
	l := collections.NewPathLock(lockDir)
	record := filepath.Join(repoDir, "m.md")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := l.With(ctx, record, func() error { return nil }); err != nil {
				t.Errorf("lock: %v", err)
			}
		}()
	}
	wg.Wait()

	if entries, err := readNames(repoDir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("the lock polluted the repository: %v", entries)
	}
	locks, err := readNames(lockDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 {
		t.Fatalf("expected exactly one lock file, got %v", locks)
	}
}

func readNames(dir string) ([]string, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, filepath.Base(e))
	}
	return out, nil
}
