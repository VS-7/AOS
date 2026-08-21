package project_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/project"
)

func newService(t *testing.T) (*project.Service, *fakes.Repo[project.Project]) {
	t.Helper()
	repo := fakes.NewRepo[project.Project]("projects")
	svc := project.NewService(project.Deps{
		Repo:  repo,
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	return svc, repo
}

func TestRoundTrip(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, project.CreateInput{Name: "My App", Description: "primary product"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "my-app" {
		t.Fatalf("id = %q, want a slug of the name", created.ID)
	}
	if created.Status != project.Active {
		t.Fatalf("status = %q, want the default", created.Status)
	}

	got, err := svc.Get(ctx, project.GetInput{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "My App" {
		t.Fatalf("name = %q", got.Name)
	}

	desc := "renamed"
	if _, err := svc.Update(ctx, project.UpdateInput{ID: created.ID, Description: &desc}); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Get(ctx, project.GetInput{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "renamed" {
		t.Fatalf("description = %q", got.Description)
	}

	if _, err := svc.Delete(ctx, project.DeleteInput{ID: created.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, project.GetInput{ID: created.ID}); err == nil {
		t.Fatal("expected the project to be gone")
	}
}

// fakeUnlinker records every project id it was asked to drop, so a test can
// assert Delete reaches every registered Unlinker rather than just the repo.
type fakeUnlinker struct{ unlinked []string }

func (f *fakeUnlinker) UnlinkProject(_ context.Context, id string) error {
	f.unlinked = append(f.unlinked, id)
	return nil
}

func TestDeleteUnlinksWithoutRemovingTheReferencingWork(t *testing.T) {
	repo := fakes.NewRepo[project.Project]("projects")
	tasks := &fakeUnlinker{}
	goals := &fakeUnlinker{}
	svc := project.NewService(project.Deps{
		Repo:      repo,
		Unlinkers: []project.Unlinker{tasks, goals},
		Clock:     clockx.Fixed{At: time.Now()},
	})
	ctx := context.Background()

	created, err := svc.Create(ctx, project.CreateInput{Name: "Doomed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(ctx, project.DeleteInput{ID: created.ID}); err != nil {
		t.Fatal(err)
	}

	for name, u := range map[string]*fakeUnlinker{"tasks": tasks, "goals": goals} {
		if len(u.unlinked) != 1 || u.unlinked[0] != created.ID {
			t.Fatalf("%s was not asked to unlink %q: got %v", name, created.ID, u.unlinked)
		}
	}
	// Delete only removes the project's own record — an Unlinker dropping
	// its own reference is the whole mechanism; nothing here re-deletes a
	// task or a goal, which is the property this test exists to pin down.
}

func TestCreateRequiresAName(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(context.Background(), project.CreateInput{})
	requireCode(t, err, "PROJECT_NAME_REQUIRED")
}

func TestSourceMustBeAbsolute(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(context.Background(), project.CreateInput{Name: "x", Source: "relative/path"})
	requireCode(t, err, "PROJECT_SOURCE_INVALID")
}

func TestSourceMustExist(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(context.Background(), project.CreateInput{
		Name: "x", Source: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	requireCode(t, err, "PROJECT_SOURCE_INVALID")
}

func TestSourceMustBeADirectory(t *testing.T) {
	svc, _ := newService(t)
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(context.Background(), project.CreateInput{Name: "x", Source: file})
	requireCode(t, err, "PROJECT_SOURCE_INVALID")
}

func TestSourceThatIsAnExistingDirectoryIsAccepted(t *testing.T) {
	svc, _ := newService(t)
	dir := t.TempDir()
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "x", Source: dir})
	if err != nil {
		t.Fatal(err)
	}
	if created.Source != dir {
		t.Fatalf("source = %q", created.Source)
	}
}

func TestListFiltersByStatus(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, project.CreateInput{Name: "Active One", Status: project.Active}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, project.CreateInput{Name: "Archived One", Status: project.Archived}); err != nil {
		t.Fatal(err)
	}

	active, err := svc.List(ctx, project.ListInput{Query: project.Query{Status: project.Active}})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Name != "Active One" {
		t.Fatalf("active = %+v", active)
	}

	all, err := svc.List(ctx, project.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all = %+v", all)
	}
}

func TestGetNotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Get(context.Background(), project.GetInput{ID: "missing"})
	requireCode(t, err, "PROJECT_NOT_FOUND")
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, want *apperr.Error", err)
	}
	if got := app.Code; got != code && got != "AOS_"+code {
		t.Fatalf("code = %q, want %q", got, code)
	}
}
