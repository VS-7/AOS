package fscollections_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/testsuite"
)

// record carries every placeholder the native collections use, so one type
// exercises all thirteen models through the real engine.
type record struct {
	ID      string `yaml:"-" json:"id"      collection:"path"`
	Agent   string `yaml:"-" json:"agent"   collection:"path"`
	Skill   string `yaml:"-" json:"skill"   collection:"path"`
	Type    string `yaml:"-" json:"type"    collection:"path"`
	TaskID  string `yaml:"-" json:"taskId"  collection:"path"`
	Task    string `yaml:"-" json:"task"    collection:"path"`
	Routine string `yaml:"-" json:"routine" collection:"path"`

	Title      string    `yaml:"title"          json:"title"`
	Category   string    `yaml:"category"       json:"category"`
	Status     string    `yaml:"status"         json:"status"`
	Tags       []string  `yaml:"tags,omitempty" json:"tags,omitempty"`
	Confidence float64   `yaml:"confidence"     json:"confidence"`
	CreatedAt  time.Time `yaml:"createdAt"      json:"createdAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

func modelFor(t *testing.T, name string) collections.Model[record] {
	t.Helper()
	m, err := collections.ModelOf[record](name)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// keyFor fills exactly the placeholders the collection's write pattern needs.
func keyFor(t *testing.T, m collections.Model[record], suffix string) collections.Key {
	t.Helper()
	p, err := m.WritePattern()
	if err != nil {
		t.Fatal(err)
	}
	k := collections.Key{}
	for _, f := range p.Fields() {
		k[f] = f + "-" + suffix
	}
	return k
}

func applyKey(v *record, k collections.Key) {
	for name, value := range k {
		switch name {
		case "id":
			v.ID = value
		case "agent":
			v.Agent = value
		case "skill":
			v.Skill = value
		case "type":
			v.Type = value
		case "taskId":
			v.TaskID = value
		case "task":
			v.Task = value
		case "routine":
			v.Routine = value
		}
	}
}

func newRepo(t *testing.T, name string) *fscollections.Repo[record] {
	t.Helper()
	return fscollections.New(t.TempDir(), modelFor(t, name))
}

// TestRoundTripForEveryNativeCollection is the phase's headline claim: create,
// read, update and delete a record of each of the thirteen models, on the real
// filesystem, through the real engine.
func TestRoundTripForEveryNativeCollection(t *testing.T) {
	ctx := context.Background()
	for _, desc := range collections.Natives() {
		t.Run(desc.Name, func(t *testing.T) {
			m := modelFor(t, desc.Name)
			repo := fscollections.New(t.TempDir(), m)
			key := keyFor(t, m, "1")

			want := &record{
				Title:      "título com acento",
				Category:   "preference",
				Status:     "active",
				Tags:       []string{"a", "b"},
				Confidence: 0.9,
				CreatedAt:  refTime,
				Content:    "  corpo indentado\n\nsegunda linha 日本語\n",
			}
			applyKey(want, key)

			if err := repo.Create(ctx, want); err != nil {
				t.Fatalf("create: %v", err)
			}

			got, err := repo.Get(ctx, key)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.Title != want.Title || got.Category != want.Category || got.Status != want.Status {
				t.Errorf("front matter lost data: %+v", got)
			}
			if got.Confidence != want.Confidence || !got.CreatedAt.Equal(refTime) {
				t.Errorf("typed fields lost data: %+v", got)
			}
			if len(got.Tags) != 2 {
				t.Errorf("tags = %v", got.Tags)
			}
			if collections.KeyOf(got).String() != key.String() {
				t.Errorf("key = %s, want %s", collections.KeyOf(got), key)
			}
			if desc.Format == collections.FormatMarkdown && got.Content != want.Content {
				t.Errorf("body = %q, want %q", got.Content, want.Content)
			}

			got.Title = "renamed"
			if err := repo.Update(ctx, got, collections.Version{}); err != nil {
				t.Fatalf("update: %v", err)
			}
			after, err := repo.Get(ctx, key)
			if err != nil {
				t.Fatal(err)
			}
			if after.Title != "renamed" {
				t.Errorf("update did not persist: %q", after.Title)
			}

			list, err := repo.List(ctx, collections.Query{})
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 1 {
				t.Fatalf("list returned %d records, want 1", len(list))
			}

			if err := repo.Delete(ctx, key); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if _, err := repo.Get(ctx, key); !errors.Is(err, apperr.ErrNotFound) {
				t.Fatalf("get after delete = %v", err)
			}
		})
	}
}

// TestRepositoryContract runs the port contract against the filesystem
// implementation. The fake runs the same suite.
func TestRepositoryContract(t *testing.T) {
	testsuite.RunRepositoryContract(t, testsuite.RepositoryContract[record]{
		New: func(t *testing.T) collections.Repository[record] {
			return newRepo(t, "memories")
		},
		Sample: func(i int) *record {
			return &record{
				Agent: "luara", ID: fmt.Sprintf("m-%02d", i),
				Title: fmt.Sprintf("memory %02d", i), Category: fmt.Sprintf("cat-%02d", i),
				Status: "active", CreatedAt: refTime.Add(time.Duration(i) * time.Hour),
				Content: "body\n",
			}
		},
		KeyOf:   collections.KeyOf[record],
		Mutate:  func(v *record) { v.Status = "deprecated" },
		Changed: func(v *record) bool { return v.Status == "deprecated" },
		Filter:  func(v *record) (string, any) { return "category", v.Category },
	})
}

func TestWriteIsAtomicAndLeavesNoTemporaryFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := fscollections.New(root, modelFor(t, "memories"))
	v := &record{Agent: "luara", ID: "m1", Title: "t", Content: "body\n"}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		v.Title = fmt.Sprintf("t%d", i)
		if err := repo.Update(ctx, v, collections.Version{}); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(root, ".aos", "agents", "luara", "memories")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("temporary file survived: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly the record file, got %v", entries)
	}
}

// TestFiftyConcurrentWritersNeverCorruptARecord is the scenario the lock and
// the atomic write exist for: twenty job workers and N parallel instances of
// the same agent share one memory graph.
func TestFiftyConcurrentWritersNeverCorruptARecord(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	base := &record{Agent: "luara", ID: "m1", Title: "t", Content: "body\n"}
	if err := repo.Create(ctx, base); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := &record{
				Agent: "luara", ID: "m1",
				Title:   fmt.Sprintf("title-%02d", i),
				Content: strings.Repeat("x", 512) + "\n",
			}
			if err := repo.Update(ctx, v, collections.Version{}); err != nil {
				t.Errorf("update %d: %v", i, err)
			}
		}(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := repo.Get(ctx, collections.Key{"agent": "luara", "id": "m1"})
			if err != nil {
				t.Errorf("get: %v", err)
				return
			}
			if !strings.HasPrefix(got.Title, "title-") && got.Title != "t" {
				t.Errorf("read a record that was never written whole: %q", got.Title)
			}
		}()
	}
	wg.Wait()
}

// TestConflictIsDetected covers the two-agent-universes case: two parallel
// selves editing the same memory must not lose an edit in silence.
func TestConflictIsDetected(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	v := &record{Agent: "luara", ID: "m1", Title: "original", Content: "body\n"}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}
	stale, err := repo.Version(ctx, collections.Key{"agent": "luara", "id": "m1"})
	if err != nil {
		t.Fatal(err)
	}

	other := &record{Agent: "luara", ID: "m1", Title: "written by the other self", Content: "body\n"}
	if err := repo.Update(ctx, other, collections.Version{}); err != nil {
		t.Fatal(err)
	}

	v.Title = "written by this self"
	err = repo.Update(ctx, v, stale)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v, want a conflict", err)
	}
	e, _ := apperr.As(err)
	if e == nil || len(e.Actions) == 0 {
		t.Fatal("the conflict must tell the caller to reload and reapply")
	}

	// The other self's write survived: a rejected update changes nothing.
	got, err := repo.Get(ctx, collections.Key{"agent": "luara", "id": "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "written by the other self" {
		t.Fatalf("title = %q", got.Title)
	}
}

// TestCascadeDeleteRemovesTheWholeDirectory: deleting a task takes its todos,
// comments and runs with it.
func TestCascadeDeleteRemovesTheWholeDirectory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	tasks := fscollections.New(root, modelFor(t, "tasks"))
	todos := fscollections.New(root, modelFor(t, "todos"))

	if err := tasks.Create(ctx, &record{ID: "t-1", Title: "ship phase 1", Content: "body\n"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := todos.Create(ctx, &record{TaskID: "t-1", ID: fmt.Sprintf("td-%d", i), Title: "step"}); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := todos.List(ctx, collections.Query{}); len(got) != 3 {
		t.Fatalf("%d todos before delete", len(got))
	}

	if err := tasks.Delete(ctx, collections.Key{"id": "t-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".aos", "tasks", "t-1")); !os.IsNotExist(err) {
		t.Fatal("the task directory survived the delete")
	}
	got, err := todos.List(ctx, collections.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%d todos survived the cascade", len(got))
	}
}

// TestNonCascadingDeleteLeavesSiblingsAlone: a memory is a file, not a
// directory, and deleting it must not touch the agent's other memories.
func TestNonCascadingDeleteLeavesSiblingsAlone(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	for i := 0; i < 3; i++ {
		if err := repo.Create(ctx, &record{Agent: "luara", ID: fmt.Sprintf("m%d", i), Title: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.Delete(ctx, collections.Key{"agent": "luara", "id": "m1"}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.List(ctx, collections.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d memories left, want 2", len(got))
	}
}

func TestHooksRunInTheRightOrder(t *testing.T) {
	ctx := context.Background()
	m := modelFor(t, "memories")

	var events []string
	m.OnCreated = func(_ context.Context, v *record) error {
		events = append(events, "created")
		v.Agent = strings.ToLower(v.Agent) // the original's normalisation
		v.CreatedAt = refTime
		return nil
	}
	m.OnUpdated = func(_ context.Context, old, v *record) error {
		events = append(events, "updated:"+old.Title)
		v.CreatedAt = old.CreatedAt // updating must not rewrite creation time
		return nil
	}
	m.OnDeleted = func(_ context.Context, v *record) error {
		events = append(events, "deleted:"+v.Title)
		return nil
	}

	root := t.TempDir()
	repo := fscollections.New(root, m)
	v := &record{Agent: "LUARA", ID: "m1", Title: "first", Content: "body\n"}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}

	// The hook normalised the agent id before the path was built, so the
	// record must be on disk under the lowercase name.
	if _, err := os.Stat(filepath.Join(root, ".aos", "agents", "luara", "memories", "m1.memory.md")); err != nil {
		t.Fatalf("the record is not where the normalised key says: %v", err)
	}

	v.Title = "second"
	if err := repo.Update(ctx, v, collections.Version{}); err != nil {
		t.Fatal(err)
	}
	if !v.CreatedAt.Equal(refTime) {
		t.Error("the update hook did not preserve createdAt")
	}
	if err := repo.Delete(ctx, collections.Key{"agent": "luara", "id": "m1"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"created", "updated:first", "deleted:second"}
	if len(events) != len(want) {
		t.Fatalf("hooks fired %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("hook %d = %q, want %q", i, events[i], want[i])
		}
	}
}

func TestAFailingCreateHookAbortsTheWrite(t *testing.T) {
	ctx := context.Background()
	m := modelFor(t, "memories")
	sentinel := errors.New("refused by the hook")
	m.OnCreated = func(context.Context, *record) error { return sentinel }

	root := t.TempDir()
	repo := fscollections.New(root, m)
	if err := repo.Create(ctx, &record{Agent: "luara", ID: "m1"}); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".aos", "agents")); !os.IsNotExist(err) {
		t.Fatal("a refused create still touched the disk")
	}
}

// TestSkillScopedRecordsAreReadable: a memory shipped inside a skill belongs to
// the same collection and is listed with the rest.
func TestSkillScopedRecordsAreReadable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := fscollections.New(root, modelFor(t, "memories"))

	if err := repo.Create(ctx, &record{Agent: "luara", ID: "own", Title: "own memory"}); err != nil {
		t.Fatal(err)
	}
	// A skill installs its own agent's memory directly on disk.
	shipped := filepath.Join(root, ".aos", "skills", "github-flow", "agents", "reviewer", "memories", "packed.memory.md")
	if err := os.MkdirAll(filepath.Dir(shipped), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shipped, []byte("---\ntitle: shipped by the skill\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := repo.List(ctx, collections.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d memories, want 2 (one own, one from the skill)", len(got))
	}

	fromSkill, err := repo.Get(ctx, collections.Key{"agent": "reviewer", "id": "packed"})
	if err != nil {
		t.Fatalf("a skill-scoped record must be readable: %v", err)
	}
	if fromSkill.Title != "shipped by the skill" {
		t.Errorf("title = %q", fromSkill.Title)
	}
}

func TestListFiltersByKeyAndField(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	for _, agent := range []string{"luara", "reviewer"} {
		for i := 0; i < 3; i++ {
			v := &record{
				Agent: agent, ID: fmt.Sprintf("m%d", i),
				Category: []string{"decision", "preference", "fact"}[i],
				Status:   "active",
				Tags:     []string{"shared", agent},
			}
			if err := repo.Create(ctx, v); err != nil {
				t.Fatal(err)
			}
		}
	}

	byAgent, err := repo.List(ctx, collections.Query{Key: collections.Key{"agent": "luara"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(byAgent) != 3 {
		t.Fatalf("%d records for one agent, want 3", len(byAgent))
	}

	byCategory, err := repo.List(ctx, collections.Query{Filters: map[string]any{"category": "decision"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(byCategory) != 2 {
		t.Fatalf("%d records with category=decision, want 2", len(byCategory))
	}

	// A slice field matches when it contains the value: filtering by one tag
	// is the common case.
	byTag, err := repo.List(ctx, collections.Query{Filters: map[string]any{"tags": "reviewer"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTag) != 3 {
		t.Fatalf("%d records tagged reviewer, want 3", len(byTag))
	}
}

func TestListOrdersByFieldWithAStableTieBreak(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	for i := 0; i < 6; i++ {
		v := &record{
			Agent: "luara", ID: fmt.Sprintf("m%d", i),
			CreatedAt:  refTime.Add(time.Duration(i%3) * time.Hour), // deliberate ties
			Confidence: float64(i%3) / 10,
		}
		if err := repo.Create(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.List(ctx, collections.Query{OrderBy: "createdAt"})
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		again, err := repo.List(ctx, collections.Query{OrderBy: "createdAt"})
		if err != nil {
			t.Fatal(err)
		}
		for i := range again {
			if again[i].ID != first[i].ID {
				t.Fatalf("ties are not broken deterministically: %v then %v", ids(first), ids(again))
			}
		}
	}
	for i := 1; i < len(first); i++ {
		if first[i].CreatedAt.Before(first[i-1].CreatedAt) {
			t.Fatalf("not ordered by createdAt: %v", first)
		}
	}
}

func ids(rs []record) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

// TestListDoesNotLoadBodies is the memory decision made visible: a workspace
// inventory needs names, not tens of kilobytes of task descriptions.
func TestListDoesNotLoadBodies(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	if err := repo.Create(ctx, &record{Agent: "luara", ID: "m1", Title: "t", Content: "a long body\n"}); err != nil {
		t.Fatal(err)
	}

	lean, err := repo.List(ctx, collections.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if lean[0].Content != "" {
		t.Errorf("List loaded the body: %q", lean[0].Content)
	}
	if lean[0].Title != "t" {
		t.Errorf("List dropped the front matter too: %+v", lean[0])
	}

	full, err := repo.List(ctx, collections.Query{IncludeContent: true})
	if err != nil {
		t.Fatal(err)
	}
	if full[0].Content != "a long body\n" {
		t.Errorf("IncludeContent did not load the body: %q", full[0].Content)
	}
}

func TestGetAlwaysLoadsTheBody(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	if err := repo.Create(ctx, &record{Agent: "luara", ID: "m1", Content: "the body\n"}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, collections.Key{"agent": "luara", "id": "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "the body\n" {
		t.Fatalf("content = %q", got.Content)
	}
}

// TestAnExternalEditIsSeenImmediately: the index is a cache, never the truth.
// A git checkout or an editor save must not be invisible.
func TestAnExternalEditIsSeenImmediately(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := fscollections.New(root, modelFor(t, "memories"))
	if err := repo.Create(ctx, &record{Agent: "luara", ID: "m1", Title: "before"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.List(ctx, collections.Query{}); err != nil { // warm the index
		t.Fatal(err)
	}

	path := filepath.Join(root, ".aos", "agents", "luara", "memories", "m1.memory.md")
	if err := os.WriteFile(path, []byte("---\ntitle: edited by hand\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := repo.List(ctx, collections.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "edited by hand" {
		t.Fatalf("title = %q — the cache served a stale record", got[0].Title)
	}
}

func TestRefreshCountsEveryRecord(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	for i := 0; i < 25; i++ {
		if err := repo.Create(ctx, &record{Agent: "luara", ID: fmt.Sprintf("m%02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := repo.Refresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 25 {
		t.Fatalf("Refresh counted %d records, want 25", n)
	}
}

// TestKeysCannotEscapeTheWorkspace: a record key names one path element, and
// resolution happens before the containment check.
func TestKeysCannotEscapeTheWorkspace(t *testing.T) {
	ctx := context.Background()
	repo := newRepo(t, "memories")
	err := repo.Create(ctx, &record{Agent: "luara", ID: "../../../../etc/passwd"})
	if err == nil {
		t.Fatal("a traversing key must be refused")
	}
	if !errors.Is(err, apperr.ErrInvalid) && !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("error = %v", err)
	}
}

func TestEventsArePublishedForEveryWrite(t *testing.T) {
	ctx := context.Background()
	bus := &recordingBus{}
	repo := fscollections.New(t.TempDir(), modelFor(t, "memories"),
		fscollections.WithPublisher[record](bus))

	v := &record{Agent: "luara", ID: "m1", Title: "t"}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}
	v.Title = "t2"
	if err := repo.Update(ctx, v, collections.Version{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, collections.Key{"agent": "luara", "id": "m1"}); err != nil {
		t.Fatal(err)
	}

	got := bus.ops()
	want := []string{"create", "update", "delete"}
	if len(got) != 3 {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %q, want %q", i, got[i], want[i])
		}
	}
	if bus.last().Collection != "memories" || bus.last().Key["id"] != "m1" {
		t.Errorf("event does not identify the record: %+v", bus.last())
	}
}

type recordingBus struct {
	mu     sync.Mutex
	events []collections.Changed
}

func (b *recordingBus) Publish(_ context.Context, ev collections.Changed) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, ev)
}

func (b *recordingBus) ops() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.events))
	for i, e := range b.events {
		out[i] = e.Op
	}
	return out
}

func (b *recordingBus) last() collections.Changed {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return collections.Changed{}
	}
	return b.events[len(b.events)-1]
}

func TestWalkIgnoresNoiseDirectories(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := fscollections.New(root, modelFor(t, "memories"))
	if err := repo.Create(ctx, &record{Agent: "luara", ID: "m1"}); err != nil {
		t.Fatal(err)
	}
	// A .git directory that happens to mirror the pattern must not be read.
	noise := filepath.Join(root, ".aos", "agents", ".git", "memories")
	if err := os.MkdirAll(noise, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noise, "x.memory.md"), []byte("---\ntitle: noise\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := repo.List(ctx, collections.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("%d records, want 1 — the walk read something it should ignore", len(got))
	}
}
