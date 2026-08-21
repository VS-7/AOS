package instruction_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/instruction"
)

// ---- fakeRepository: an in-memory instruction.Repository ----

type fakeRepository struct {
	mu           sync.Mutex
	instructions map[string]instruction.Instruction
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{instructions: map[string]instruction.Instruction{}}
}

func (r *fakeRepository) Get(_ context.Context, key collections.Key) (*instruction.Instruction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i, ok := r.instructions[key["id"]]
	if !ok {
		return nil, fmt.Errorf("fakeRepository: no instruction %q", key["id"])
	}
	out := i
	return &out, nil
}

func (r *fakeRepository) List(_ context.Context, _ collections.Query) ([]instruction.Instruction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]instruction.Instruction, 0, len(r.instructions))
	for _, i := range r.instructions {
		out = append(out, i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeRepository) Create(_ context.Context, v *instruction.Instruction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.instructions[v.ID]; exists {
		return fmt.Errorf("fakeRepository: instruction %q already exists", v.ID)
	}
	r.instructions[v.ID] = *v
	return nil
}

func (r *fakeRepository) Update(_ context.Context, v *instruction.Instruction, _ collections.Version) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.instructions[v.ID]; !exists {
		return fmt.Errorf("fakeRepository: no instruction %q", v.ID)
	}
	r.instructions[v.ID] = *v
	return nil
}

func (r *fakeRepository) Delete(_ context.Context, key collections.Key) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.instructions, key["id"])
	return nil
}

func newService(t *testing.T) *instruction.Service {
	t.Helper()
	return instruction.NewService(instruction.Deps{
		Repo:  newFakeRepository(),
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
}

func ctx() context.Context { return context.Background() }

// --- round trip --------------------------------------------------------

func TestRoundTripCreateGetUpdateDelete(t *testing.T) {
	svc := newService(t)

	created, err := svc.Create(ctx(), instruction.CreateInput{
		Name: "Feature Protocol", Type: "standards",
		Content: "# Usage\n\nEvery new feature ships with a test.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "feature-protocol" {
		t.Fatalf("id = %q, want the slugified name", created.ID)
	}
	if !created.Active {
		t.Fatal("a newly created instruction must start active")
	}

	got, err := svc.Get(ctx(), instruction.GetInput{ID: created.ID})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// The engine writes Content as a Markdown file body, which always ends in
	// exactly one trailing newline on disk — the same reason every text file
	// in this repository does. Compare with it trimmed rather than assuming
	// byte-for-byte equality with what was written.
	if strings.TrimRight(got.Content, "\n") != strings.TrimRight(created.Content, "\n") {
		t.Fatalf("content = %q, want %q", got.Content, created.Content)
	}

	newContent := "# Usage\n\nUpdated."
	updated, err := svc.Update(ctx(), instruction.UpdateInput{ID: created.ID, Content: &newContent})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if strings.TrimRight(updated.Content, "\n") != newContent {
		t.Fatalf("content after update = %q", updated.Content)
	}
	if !updated.UpdatedAt.After(created.CreatedAt) && updated.UpdatedAt != created.CreatedAt {
		t.Fatalf("UpdatedAt did not advance: %v vs %v", updated.UpdatedAt, created.CreatedAt)
	}

	if _, err := svc.Delete(ctx(), instruction.DeleteInput{ID: created.ID}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx(), instruction.GetInput{ID: created.ID}); err == nil {
		t.Fatal("Get after Delete succeeded")
	}

	// Idempotent: deleting what is already gone succeeds.
	if _, err := svc.Delete(ctx(), instruction.DeleteInput{ID: created.ID}); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}

func TestCreateWithoutNameFails(t *testing.T) {
	svc := newService(t)
	_, err := svc.Create(ctx(), instruction.CreateInput{})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_INSTRUCTION_NAME_REQUIRED" {
		t.Fatalf("code = %q", app.Code)
	}
}

func TestCreateRejectsADuplicateID(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), instruction.CreateInput{Name: "Feature Protocol"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := svc.Create(ctx(), instruction.CreateInput{Name: "Feature Protocol"})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_INSTRUCTION_ALREADY_EXISTS" {
		t.Fatalf("code = %q", app.Code)
	}
}

func TestGetOfAnUnknownIDIsNotFound(t *testing.T) {
	svc := newService(t)
	_, err := svc.Get(ctx(), instruction.GetInput{ID: "does-not-exist"})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if app.Code != "AOS_INSTRUCTION_NOT_FOUND" {
		t.Fatalf("code = %q", app.Code)
	}
}

// --- Applicable: the query the prompt assembler runs -------------------

func TestApplicableWithNoPathsIsAlwaysWorkspaceWide(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), instruction.CreateInput{Name: "Global Rule"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Applicable(ctx(), []string{"anything/at/all.go"})
	if err != nil {
		t.Fatalf("Applicable: %v", err)
	}
	if len(got) != 1 || got[0].ID != "global-rule" {
		t.Fatalf("got %+v, want the unscoped instruction to always apply", got)
	}
}

func TestApplicableMatchesGlobPaths(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), instruction.CreateInput{
		Name: "Go Standards", Paths: []string{"internal/domain/**/*.go"},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	matching, err := svc.Applicable(ctx(), []string{"internal/domain/instruction/service.go"})
	if err != nil {
		t.Fatalf("Applicable: %v", err)
	}
	if len(matching) != 1 {
		t.Fatalf("got %d matches, want 1 for a matching path", len(matching))
	}

	notMatching, err := svc.Applicable(ctx(), []string{"frontend/src/main.tsx"})
	if err != nil {
		t.Fatalf("Applicable: %v", err)
	}
	if len(notMatching) != 0 {
		t.Fatalf("got %d matches, want 0 for a non-matching path", len(notMatching))
	}
}

func TestApplicableNeverReturnsAnInactiveInstruction(t *testing.T) {
	svc := newService(t)
	created, err := svc.Create(ctx(), instruction.CreateInput{Name: "Retired Rule"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	inactive := false
	if _, err := svc.Update(ctx(), instruction.UpdateInput{ID: created.ID, Active: &inactive}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := svc.Applicable(ctx(), []string{"anything.go"})
	if err != nil {
		t.Fatalf("Applicable: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want an inactive instruction to never apply", got)
	}
}

// --- List ----------------------------------------------------------------

func TestListFiltersBySkill(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), instruction.CreateInput{Name: "Browser Rule", Skill: "browser"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx(), instruction.CreateInput{Name: "General Rule"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	out, err := svc.List(ctx(), instruction.ListInput{Skill: "browser"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if out.Total != 1 || out.Instructions[0].Skill != "browser" {
		t.Fatalf("got %+v, want only the browser-scoped instruction", out)
	}
}

func TestListFiltersByQueryAcrossNameDescriptionAndContent(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(ctx(), instruction.CreateInput{
		Name: "Alpha", Description: "nothing special", Content: "mentions a secret keyword: banana",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx(), instruction.CreateInput{Name: "Beta", Description: "unrelated"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	out, err := svc.List(ctx(), instruction.ListInput{Query: "banana"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if out.Total != 1 || out.Instructions[0].Name != "Alpha" {
		t.Fatalf("got %+v, want only the instruction whose content matches", out)
	}
}

// --- repository failures --------------------------------------------------

// failRepository wraps a real Repository and forces one named method to
// fail, so a test can exercise errReadFailed/errWriteFailed without a real
// storage error to provoke it.
type failRepository struct {
	*fakeRepository
	fail string
	err  error
}

func (r failRepository) List(ctx context.Context, q collections.Query) ([]instruction.Instruction, error) {
	if r.fail == "List" {
		return nil, r.err
	}
	return r.fakeRepository.List(ctx, q)
}

func (r failRepository) Update(ctx context.Context, v *instruction.Instruction, expect collections.Version) error {
	if r.fail == "Update" {
		return r.err
	}
	return r.fakeRepository.Update(ctx, v, expect)
}

func (r failRepository) Delete(ctx context.Context, key collections.Key) error {
	if r.fail == "Delete" {
		return r.err
	}
	return r.fakeRepository.Delete(ctx, key)
}

func newServiceOver(repo instruction.Repository) *instruction.Service {
	return instruction.NewService(instruction.Deps{
		Repo: repo, Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
}

func TestListWrapsARepositoryFailure(t *testing.T) {
	svc := newServiceOver(failRepository{fakeRepository: newFakeRepository(), fail: "List", err: errors.New("disk gone")})
	_, err := svc.List(ctx(), instruction.ListInput{})
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_INSTRUCTION_READ_FAILED" {
		t.Fatalf("want AOS_INSTRUCTION_READ_FAILED, got %v", err)
	}
}

func TestUpdateWrapsARepositoryFailure(t *testing.T) {
	real := newFakeRepository()
	svc := newServiceOver(real)
	created, err := svc.Create(ctx(), instruction.CreateInput{Name: "Doomed update"})
	if err != nil {
		t.Fatal(err)
	}
	failing := newServiceOver(failRepository{fakeRepository: real, fail: "Update", err: errors.New("disk gone")})
	newName := "won't stick"
	_, err = failing.Update(ctx(), instruction.UpdateInput{ID: created.ID, Name: &newName})
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_INSTRUCTION_WRITE_FAILED" {
		t.Fatalf("want AOS_INSTRUCTION_WRITE_FAILED, got %v", err)
	}
}

func TestDeleteWrapsARepositoryFailure(t *testing.T) {
	real := newFakeRepository()
	svc := newServiceOver(real)
	created, err := svc.Create(ctx(), instruction.CreateInput{Name: "Doomed delete"})
	if err != nil {
		t.Fatal(err)
	}
	failing := newServiceOver(failRepository{fakeRepository: real, fail: "Delete", err: errors.New("disk gone")})
	_, err = failing.Delete(ctx(), instruction.DeleteInput{ID: created.ID})
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_INSTRUCTION_WRITE_FAILED" {
		t.Fatalf("want AOS_INSTRUCTION_WRITE_FAILED, got %v", err)
	}
}

func TestUpdateChangesEveryOptionalField(t *testing.T) {
	svc := newService(t)
	created, err := svc.Create(ctx(), instruction.CreateInput{Name: "Original", Type: "standards"})
	if err != nil {
		t.Fatal(err)
	}

	newName := "Renamed"
	newType := "patterns"
	newDesc := "a new description"
	updated, err := svc.Update(ctx(), instruction.UpdateInput{
		ID: created.ID, Name: &newName, Type: &newType, Description: &newDesc,
		Paths: []string{"internal/**/*.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != newName || updated.Type != newType || updated.Description != newDesc {
		t.Fatalf("got %+v", updated)
	}
	if len(updated.Paths) != 1 || updated.Paths[0] != "internal/**/*.go" {
		t.Fatalf("Paths = %v", updated.Paths)
	}
}
