package project_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/project"
)

func newService(t *testing.T) (*project.Service, *fakes.Repo[project.Project]) {
	t.Helper()
	repo := fakes.NewRepo[project.Project]("projects")
	svc := project.NewService(project.Deps{
		Repo:  repo,
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		Stat:  fakePathStat{},
	})
	return svc, repo
}

// fakePathStat is an in-memory project.PathStat: dirs and files are declared
// by name, nothing else exists. It lets TestSourceMust* pin the three
// validateSource outcomes (not found, not a directory, a real directory)
// without a real filesystem, per internal/architecture's
// TestDomainTestsDoNotTouchIO.
type fakePathStat struct {
	dirs  map[string]bool
	files map[string]bool
}

func (f fakePathStat) Stat(path string) (os.FileInfo, error) {
	if f.dirs[path] {
		return fakeFileInfo{name: path, dir: true}, nil
	}
	if f.files[path] {
		return fakeFileInfo{name: path, dir: false}, nil
	}
	return nil, os.ErrNotExist
}

type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) Name() string     { return f.name }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool      { return f.dir }
func (f fakeFileInfo) Sys() any         { return nil }

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
		Name: "x", Source: "/does/not/exist",
	})
	requireCode(t, err, "PROJECT_SOURCE_INVALID")
}

func TestSourceMustBeADirectory(t *testing.T) {
	file := "/some/afile"
	svc := project.NewService(project.Deps{
		Repo:  fakes.NewRepo[project.Project]("projects"),
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		Stat:  fakePathStat{files: map[string]bool{file: true}},
	})
	_, err := svc.Create(context.Background(), project.CreateInput{Name: "x", Source: file})
	requireCode(t, err, "PROJECT_SOURCE_INVALID")
}

func TestSourceThatIsAnExistingDirectoryIsAccepted(t *testing.T) {
	dir := "/some/dir"
	svc := project.NewService(project.Deps{
		Repo:  fakes.NewRepo[project.Project]("projects"),
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		Stat:  fakePathStat{dirs: map[string]bool{dir: true}},
	})
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

// failRepository wraps a real Repository and forces one named method to
// fail, so a test can exercise errReadFailed/errWriteFailed without a real
// storage error to provoke it.
type failRepository struct {
	*fakes.Repo[project.Project]
	fail string
	err  error
}

func (r failRepository) List(ctx context.Context, q collections.Query) ([]project.Project, error) {
	if r.fail == "List" {
		return nil, r.err
	}
	return r.Repo.List(ctx, q)
}

func (r failRepository) Update(ctx context.Context, v *project.Project, expect collections.Version) error {
	if r.fail == "Update" {
		return r.err
	}
	return r.Repo.Update(ctx, v, expect)
}

func (r failRepository) Delete(ctx context.Context, key collections.Key) error {
	if r.fail == "Delete" {
		return r.err
	}
	return r.Repo.Delete(ctx, key)
}

func TestListWrapsARepositoryFailure(t *testing.T) {
	repo := failRepository{Repo: fakes.NewRepo[project.Project]("projects"), fail: "List", err: errors.New("disk gone")}
	svc := project.NewService(project.Deps{Repo: repo, Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{}})
	_, err := svc.List(context.Background(), project.ListInput{})
	requireCode(t, err, "PROJECT_READ_FAILED")
}

func TestUpdateWrapsARepositoryFailure(t *testing.T) {
	real := fakes.NewRepo[project.Project]("projects")
	svc := project.NewService(project.Deps{Repo: real, Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{}})
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "Doomed update"})
	if err != nil {
		t.Fatal(err)
	}
	failing := project.NewService(project.Deps{
		Repo:  failRepository{Repo: real, fail: "Update", err: errors.New("disk gone")},
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{},
	})
	newName := "won't stick"
	_, err = failing.Update(context.Background(), project.UpdateInput{ID: created.ID, Name: &newName})
	requireCode(t, err, "PROJECT_WRITE_FAILED")
}

func TestDeleteWrapsARepositoryFailure(t *testing.T) {
	real := fakes.NewRepo[project.Project]("projects")
	svc := project.NewService(project.Deps{Repo: real, Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{}})
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "Doomed delete"})
	if err != nil {
		t.Fatal(err)
	}
	failing := project.NewService(project.Deps{
		Repo:  failRepository{Repo: real, fail: "Delete", err: errors.New("disk gone")},
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{},
	})
	_, err = failing.Delete(context.Background(), project.DeleteInput{ID: created.ID})
	requireCode(t, err, "PROJECT_WRITE_FAILED")
}

func TestDeleteWrapsAnUnlinkerFailure(t *testing.T) {
	repo := fakes.NewRepo[project.Project]("projects")
	svc := project.NewService(project.Deps{Repo: repo, Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{}})
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "Doomed unlink"})
	if err != nil {
		t.Fatal(err)
	}

	failing := project.NewService(project.Deps{
		Repo:      repo,
		Unlinkers: []project.Unlinker{&fakeUnlinker{}, failUnlinker{err: errors.New("tasks down")}},
		Clock:     clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{},
	})
	_, err = failing.Delete(context.Background(), project.DeleteInput{ID: created.ID})
	requireCode(t, err, "PROJECT_WRITE_FAILED")
}

type failUnlinker struct{ err error }

func (f failUnlinker) UnlinkProject(context.Context, string) error { return f.err }

func TestUpdateChangesEveryOptionalField(t *testing.T) {
	svc, _ := newService(t)
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}

	newDesc := "a new description"
	newColor := "#ff0000"
	newIcon := "rocket"
	updated, err := svc.Update(context.Background(), project.UpdateInput{
		ID: created.ID, Description: &newDesc, Color: &newColor, Icon: &newIcon,
		Paths: []string{"src/**/*.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != newDesc || updated.Color != newColor || updated.Icon != newIcon {
		t.Fatalf("got %+v", updated)
	}
	if len(updated.Paths) != 1 || updated.Paths[0] != "src/**/*.ts" {
		t.Fatalf("Paths = %v", updated.Paths)
	}
}

func TestUpdateWithAnInvalidStatusLeavesItUnchanged(t *testing.T) {
	svc, _ := newService(t)
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "Original", Status: project.Active})
	if err != nil {
		t.Fatal(err)
	}
	bogus := project.Status("bogus")
	updated, err := svc.Update(context.Background(), project.UpdateInput{ID: created.ID, Status: &bogus})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != project.Active {
		t.Fatalf("Status = %q, an invalid status should leave it unchanged", updated.Status)
	}
}

func TestStatusValidRejectsAnUnknownValue(t *testing.T) {
	if project.Status("bogus").Valid() {
		t.Fatal("an unknown status validated")
	}
	for _, s := range project.Statuses {
		if !s.Valid() {
			t.Fatalf("%q, a declared status, did not validate", s)
		}
	}
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

// A project named with accents used to get an id with the accented letters
// dropped and a hyphen in their place ("Ação Rápida Área" → a-o-r-pida-rea),
// while a goal of the same title got acao-rapida-area: two slug rules for one
// kind of identity. Both derive it through internal/core/slug now.
func TestCreateDerivesTheIDTheWayEveryOtherNativeDoes(t *testing.T) {
	svc, _ := newService(t)
	created, err := svc.Create(context.Background(), project.CreateInput{Name: "Ação Rápida Área"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "acao-rapida-area" {
		t.Fatalf("id = %q, want acao-rapida-area", created.ID)
	}
}

// A name made only of symbols has nothing to derive an id from. It reached
// the repository with an empty key and came back as a 500 "could not save",
// with a call to action that called it a bug.
func TestCreateRefusesANameWithNothingToDeriveAnIDFrom(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Create(context.Background(), project.CreateInput{Name: "!!!"})
	requireCode(t, err, "PROJECT_NAME_REQUIRED")
	if status := apperr.StatusOf(err); status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}
}

// Creating a second project under a name the first already took is the
// person's to fix — choose another name — not a failed write to retry.
func TestCreateRefusesADuplicateAsAConflict(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, project.CreateInput{Name: "API de teste"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, project.CreateInput{Name: "API DE TESTE"})
	requireCode(t, err, "PROJECT_ALREADY_EXISTS")
	if status := apperr.StatusOf(err); status != 409 {
		t.Fatalf("status = %d, want 409", status)
	}
}

// Clearing a field is an empty string, not an omitted one: the form that
// empties Description and Source must be able to say so.
func TestUpdateClearsDescriptionSourceAndContentWithEmptyStrings(t *testing.T) {
	dir := "/some/dir"
	svc := project.NewService(project.Deps{
		Repo:  fakes.NewRepo[project.Project]("projects"),
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		Stat:  fakePathStat{dirs: map[string]bool{dir: true}},
	})
	ctx := context.Background()
	created, err := svc.Create(ctx, project.CreateInput{Name: "Bound", Description: "d", Source: dir, Content: "c"})
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	updated, err := svc.Update(ctx, project.UpdateInput{ID: created.ID, Description: &empty, Source: &empty, Content: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "" || updated.Source != "" || updated.Content != "" {
		t.Fatalf("got %+v, want description, source and content cleared", updated)
	}
}

// blindRepository never finds a record, the way a repository looks to a
// Create racing another writer for the same id: the lookup misses, and the
// write is what reports the collision.
type blindRepository struct{ *fakes.Repo[project.Project] }

func (blindRepository) Get(context.Context, collections.Key) (*project.Project, error) {
	return nil, errors.New("not found")
}

func TestCreateReportsARacedDuplicateAsAConflict(t *testing.T) {
	repo := blindRepository{fakes.NewRepo[project.Project]("projects")}
	svc := project.NewService(project.Deps{Repo: repo, Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, Stat: fakePathStat{}})
	ctx := context.Background()
	if _, err := svc.Create(ctx, project.CreateInput{Name: "Twice"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, project.CreateInput{Name: "Twice"})
	requireCode(t, err, "PROJECT_ALREADY_EXISTS")
}

// An agent reads what a command takes from its jsonschema, and nothing else.
// Update honours an empty string as "clear this field"
// (TestUpdateClearsDescriptionSourceAndContentWithEmptyStrings above), but
// the docs said only "Omit to leave unchanged" — so there was no way to learn
// how to unbind a project's source without reading the Go.
func TestUpdateInputDocumentsThatAnEmptyStringClears(t *testing.T) {
	fields := reflect.VisibleFields(reflect.TypeOf(project.UpdateInput{}))
	for _, name := range []string{"Description", "Source", "Content"} {
		field, ok := findField(fields, name)
		if !ok {
			t.Fatalf("UpdateInput has no %s field", name)
		}
		doc := field.Tag.Get("jsonschema")
		if !strings.Contains(doc, "Empty string") {
			t.Errorf("%s: %q says nothing about clearing it with an empty string", name, doc)
		}
	}
}

func findField(fields []reflect.StructField, name string) (reflect.StructField, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return reflect.StructField{}, false
}
