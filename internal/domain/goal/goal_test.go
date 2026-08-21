package goal_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/goal"
)

var refTime = time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

func ctx() context.Context { return context.Background() }

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not *apperr.Error: %v", err, err)
	}
	return app.Code
}

// ---- fakeRepository: an in-memory goal.Repository ----

type fakeRepository struct {
	mu    sync.Mutex
	goals map[string]goal.Goal
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{goals: map[string]goal.Goal{}}
}

func (r *fakeRepository) Get(_ context.Context, key collections.Key) (*goal.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.goals[key["id"]]
	if !ok {
		return nil, fmt.Errorf("fakeRepository: no goal %q", key["id"])
	}
	out := g
	return &out, nil
}

func (r *fakeRepository) List(_ context.Context, _ collections.Query) ([]goal.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]goal.Goal, 0, len(r.goals))
	for _, g := range r.goals {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeRepository) Create(_ context.Context, g *goal.Goal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.goals[g.ID]; exists {
		return fmt.Errorf("fakeRepository: goal %q already exists", g.ID)
	}
	r.goals[g.ID] = *g
	return nil
}

func (r *fakeRepository) Update(_ context.Context, g *goal.Goal, _ collections.Version) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.goals[g.ID]; !exists {
		return fmt.Errorf("fakeRepository: no goal %q", g.ID)
	}
	r.goals[g.ID] = *g
	return nil
}

func (r *fakeRepository) Delete(_ context.Context, key collections.Key) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.goals, key["id"])
	return nil
}

// ---- fakeTasks: records every ClearGoal call ----

type fakeTasks struct {
	mu      sync.Mutex
	cleared []string
}

func (f *fakeTasks) ClearGoal(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared = append(f.cleared, id)
	return nil
}

func newService(repo *fakeRepository, tasks *fakeTasks) *goal.Service {
	return goal.NewService(goal.Deps{Repo: repo, Tasks: tasks, Clock: clockx.Fixed{At: refTime}})
}

// TestRoundTrip proves a goal survives Create, Get, Update and Delete
// through the service, over an in-memory repository.
func TestRoundTrip(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})

	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Launch V1", Measure: "v1 released"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "launch-v1" {
		t.Fatalf("ID = %q, want %q (derived from title)", created.ID, "launch-v1")
	}
	if created.Status != goal.StatusActive {
		t.Fatalf("Status = %q, want active by default", created.Status)
	}
	if !created.CreatedAt.Equal(refTime) || !created.UpdatedAt.Equal(refTime) {
		t.Fatalf("timestamps not set from the injected clock")
	}

	got, err := svc.Get(ctx(), goal.GetInput{ID: created.ID})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Launch V1" {
		t.Fatalf("Title = %q, want %q", got.Title, "Launch V1")
	}

	achieved := goal.StatusAchieved
	updated, err := svc.Update(ctx(), goal.UpdateInput{ID: created.ID, Status: &achieved})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != goal.StatusAchieved {
		t.Fatalf("Status after Update = %q, want achieved", updated.Status)
	}

	if _, err := svc.Delete(ctx(), goal.DeleteInput{ID: created.ID}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx(), goal.GetInput{ID: created.ID}); err == nil {
		t.Fatalf("Get after Delete: want error, got none")
	} else if code := codeOf(t, err); code != "AOS_GOAL_NOT_FOUND" {
		t.Fatalf("code = %q, want GOAL_NOT_FOUND", code)
	}
}

// TestActiveReturnsOnlyActive proves Active filters to status active alone,
// which is what feeds the prompt's workspace inventory.
func TestActiveReturnsOnlyActive(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})

	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "Active One"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "Achieved One", Status: goal.StatusAchieved}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "Paused One", Status: goal.StatusPaused}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	active, err := svc.Active(ctx())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("Active returned %d goals, want 1: %+v", len(active), active)
	}
	if active[0].Title != "Active One" {
		t.Fatalf("Active[0].Title = %q, want %q", active[0].Title, "Active One")
	}
}

// TestCreateInstalledBySkillCarriesSkillField proves a goal a skill installs
// keeps that provenance, which is what lets it be found and removed on
// uninstall.
func TestCreateInstalledBySkillCarriesSkillField(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})

	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Bundled Goal", Skill: "crm"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Skill != "crm" {
		t.Fatalf("Skill = %q, want %q", created.Skill, "crm")
	}

	got, err := svc.Get(ctx(), goal.GetInput{ID: created.ID})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Skill != "crm" {
		t.Fatalf("Skill after Get = %q, want %q", got.Skill, "crm")
	}
}

// TestDeleteDisassociatesTasksWithoutRemovingThem proves Delete asks Tasks
// to clear its id, and nothing else — the tasks themselves are Tasks' own
// responsibility, not this package's, to remove.
func TestDeleteDisassociatesTasksWithoutRemovingThem(t *testing.T) {
	repo := newFakeRepository()
	tasks := &fakeTasks{}
	svc := newService(repo, tasks)

	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Has Tasks"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Delete(ctx(), goal.DeleteInput{ID: created.ID}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(tasks.cleared) != 1 || tasks.cleared[0] != created.ID {
		t.Fatalf("tasks.cleared = %v, want [%q]", tasks.cleared, created.ID)
	}
}

// TestCreateRejectsBlankTitle proves a title is required — nothing to derive
// an id from otherwise.
func TestCreateRejectsBlankTitle(t *testing.T) {
	svc := newService(newFakeRepository(), &fakeTasks{})
	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "   "}); err == nil {
		t.Fatalf("Create with blank title: want error, got none")
	} else if code := codeOf(t, err); code != "AOS_GOAL_TITLE_REQUIRED" {
		t.Fatalf("code = %q, want GOAL_TITLE_REQUIRED", code)
	}
}

// TestUpdateRejectsInvalidStatus proves a status outside the four-member
// union is refused rather than silently stored.
func TestUpdateRejectsInvalidStatus(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})

	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Some Goal"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	bogus := goal.Status("bogus")
	if _, err := svc.Update(ctx(), goal.UpdateInput{ID: created.ID, Status: &bogus}); err == nil {
		t.Fatalf("Update with invalid status: want error, got none")
	} else if code := codeOf(t, err); code != "AOS_GOAL_STATUS_INVALID" {
		t.Fatalf("code = %q, want GOAL_STATUS_INVALID", code)
	}
}

// TestListFiltersByProjectAndText proves List's filters compose: a text
// match narrows further within a project match.
func TestListFiltersByProjectAndText(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})

	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "Launch V1", Project: "fractal-os"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "Reduce Latency", Project: "fractal-os"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx(), goal.CreateInput{Title: "Other Project Goal", Project: "other"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := svc.List(ctx(), goal.ListInput{Query: goal.Query{Project: "fractal-os", Text: "launch"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(found) != 1 || found[0].Title != "Launch V1" {
		t.Fatalf("List = %+v, want just Launch V1", found)
	}
}

// failRepository wraps a real Repository and forces one named method to
// fail, so a test can exercise errReadFailed/errWriteFailed without a real
// storage error to provoke it.
type failRepository struct {
	*fakeRepository
	fail string
	err  error
}

func (r failRepository) List(ctx context.Context, q collections.Query) ([]goal.Goal, error) {
	if r.fail == "List" {
		return nil, r.err
	}
	return r.fakeRepository.List(ctx, q)
}

func (r failRepository) Update(ctx context.Context, g *goal.Goal, expect collections.Version) error {
	if r.fail == "Update" {
		return r.err
	}
	return r.fakeRepository.Update(ctx, g, expect)
}

func (r failRepository) Delete(ctx context.Context, key collections.Key) error {
	if r.fail == "Delete" {
		return r.err
	}
	return r.fakeRepository.Delete(ctx, key)
}

func TestListWrapsARepositoryFailure(t *testing.T) {
	repo := failRepository{fakeRepository: newFakeRepository(), fail: "List", err: errors.New("disk gone")}
	svc := goal.NewService(goal.Deps{Repo: repo, Tasks: &fakeTasks{}, Clock: clockx.Fixed{At: refTime}})
	_, err := svc.List(ctx(), goal.ListInput{})
	if code := codeOf(t, err); code != "AOS_GOAL_READ_FAILED" {
		t.Fatalf("code = %q, want GOAL_READ_FAILED", code)
	}
}

func TestUpdateWrapsARepositoryFailure(t *testing.T) {
	real := newFakeRepository()
	svc := newService(real, &fakeTasks{})
	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Doomed update"})
	if err != nil {
		t.Fatal(err)
	}
	failing := goal.NewService(goal.Deps{
		Repo:  failRepository{fakeRepository: real, fail: "Update", err: errors.New("disk gone")},
		Tasks: &fakeTasks{}, Clock: clockx.Fixed{At: refTime},
	})
	newTitle := "won't stick"
	_, err = failing.Update(ctx(), goal.UpdateInput{ID: created.ID, Title: &newTitle})
	if code := codeOf(t, err); code != "AOS_GOAL_WRITE_FAILED" {
		t.Fatalf("code = %q, want GOAL_WRITE_FAILED", code)
	}
}

func TestDeleteWrapsARepositoryFailure(t *testing.T) {
	real := newFakeRepository()
	svc := newService(real, &fakeTasks{})
	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Doomed delete"})
	if err != nil {
		t.Fatal(err)
	}
	failing := goal.NewService(goal.Deps{
		Repo:  failRepository{fakeRepository: real, fail: "Delete", err: errors.New("disk gone")},
		Tasks: &fakeTasks{}, Clock: clockx.Fixed{At: refTime},
	})
	_, err = failing.Delete(ctx(), goal.DeleteInput{ID: created.ID})
	if code := codeOf(t, err); code != "AOS_GOAL_WRITE_FAILED" {
		t.Fatalf("code = %q, want GOAL_WRITE_FAILED", code)
	}
}

// failTasks is a goal.Tasks whose ClearGoal always fails, to exercise
// Delete's other errWriteFailed call site.
type failTasks struct{ err error }

func (f failTasks) ClearGoal(context.Context, string) error { return f.err }

func TestDeleteWrapsATasksUnlinkFailure(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})
	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Doomed unlink"})
	if err != nil {
		t.Fatal(err)
	}
	failing := goal.NewService(goal.Deps{
		Repo: repo, Tasks: failTasks{err: errors.New("task service down")}, Clock: clockx.Fixed{At: refTime},
	})
	_, err = failing.Delete(ctx(), goal.DeleteInput{ID: created.ID})
	if code := codeOf(t, err); code != "AOS_GOAL_WRITE_FAILED" {
		t.Fatalf("code = %q, want GOAL_WRITE_FAILED", code)
	}
}

func TestUpdateChangesEveryOptionalField(t *testing.T) {
	repo := newFakeRepository()
	svc := newService(repo, &fakeTasks{})
	created, err := svc.Create(ctx(), goal.CreateInput{Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}

	newDesc := "a new description"
	newProject := "new-project"
	newDue := refTime.Add(48 * time.Hour)
	newMeasure := "shipped to prod"
	newContent := "# Notes\nUpdated."
	updated, err := svc.Update(ctx(), goal.UpdateInput{
		ID: created.ID, Description: &newDesc, Project: &newProject,
		DueAt: &newDue, Measure: &newMeasure, Content: &newContent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != newDesc || updated.Project != newProject || updated.Measure != newMeasure || updated.Content != newContent {
		t.Fatalf("got %+v", updated)
	}
	if updated.DueAt == nil || !updated.DueAt.Equal(newDue) {
		t.Fatalf("DueAt = %v, want %v", updated.DueAt, newDue)
	}
}
