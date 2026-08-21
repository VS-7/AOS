package instruction_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/instruction"
)

func newService(t *testing.T) (*instruction.Service, string) {
	t.Helper()
	root := t.TempDir()
	model, err := collections.ModelOf[instruction.Instruction]("instructions")
	if err != nil {
		t.Fatalf("ModelOf: %v", err)
	}
	repo := fscollections.New(root, model)
	svc := instruction.NewService(instruction.Deps{
		Repo:  repo,
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	return svc, root
}

func ctx() context.Context { return context.Background() }

// --- round trip --------------------------------------------------------

func TestRoundTripCreateGetUpdateDelete(t *testing.T) {
	svc, _ := newService(t)

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
	svc, _ := newService(t)
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
	svc, _ := newService(t)
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
	svc, _ := newService(t)
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
	svc, _ := newService(t)
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
	svc, _ := newService(t)
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
	svc, _ := newService(t)
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
	svc, _ := newService(t)
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
