package artifact_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/fakes"
)

var at = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func ctx() context.Context { return context.Background() }

// fakeFiles is the in-memory artifact.Files a domain test writes to. It
// records the entrypoint it was asked to scaffold, so a test can assert on
// what Create actually requested without touching a real disk —
// internal/architecture's TestDomainTestsDoNotTouchIO forbids that here; the
// real implementation, internal/adapters/artifactfiles, has its own test
// against a real temp directory.
type fakeFiles struct {
	mu      sync.Mutex
	ensured map[string]string // id -> entrypoint actually in place
	removed map[string]bool
	ensureN map[string]int // id -> how many times Ensure was called
}

func newFakeFiles() *fakeFiles {
	return &fakeFiles{ensured: map[string]string{}, removed: map[string]bool{}, ensureN: map[string]int{}}
}

func (f *fakeFiles) Ensure(_ context.Context, id, entrypoint string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entrypoint == "" {
		entrypoint = "index.html"
	}
	f.ensured[id] = entrypoint
	f.ensureN[id]++
	return entrypoint, nil
}

func (f *fakeFiles) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed[id] = true
	return nil
}

func newService(t *testing.T) (*artifact.Service, *fakes.Repo[artifact.Artifact], *fakeFiles) {
	t.Helper()
	repo := fakes.NewRepo[artifact.Artifact]("artifacts")
	files := newFakeFiles()
	svc := artifact.NewService(artifact.Deps{
		Repo: repo, Files: files, Hasher: artifact.Argon2Hasher{},
		Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "art"},
	})
	return svc, repo, files
}

// --- round-trip CRUD ---------------------------------------------------

func TestCreateGetUpdateDeleteRoundTrips(t *testing.T) {
	svc, _, _ := newService(t)

	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Sales dashboard"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("no id assigned")
	}

	got, err := svc.Get(ctx(), artifact.GetInput{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Sales dashboard" {
		t.Fatalf("got %+v", got)
	}

	newName := "Q1 dashboard"
	updated, err := svc.Update(ctx(), artifact.UpdateInput{ID: created.ID, Name: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != newName {
		t.Fatalf("update did not stick: %+v", updated)
	}

	list, err := svc.List(ctx(), artifact.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 artifact, got %d", len(list))
	}

	if _, err := svc.Delete(ctx(), artifact.DeleteInput{ID: created.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx(), artifact.GetInput{ID: created.ID}); err == nil {
		t.Fatal("artifact still readable after delete")
	}
}

// --- scaffold on empty entrypoint --------------------------------------

func TestCreateWithNoEntrypointScaffoldsTheDefault(t *testing.T) {
	svc, _, files := newService(t)

	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "No entrypoint given"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Entrypoint != "index.html" {
		t.Fatalf("want the scaffolded default, got %q", created.Entrypoint)
	}
	if got := files.ensured[created.ID]; got != "index.html" {
		t.Fatalf("Files.Ensure was not asked for the default entrypoint: %q", got)
	}
}

func TestCreateWithAnExplicitEntrypointIsNotOverridden(t *testing.T) {
	svc, _, files := newService(t)

	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Has its own root", Entrypoint: "app.html"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Entrypoint != "app.html" {
		t.Fatalf("got %q", created.Entrypoint)
	}
	if got := files.ensured[created.ID]; got != "app.html" {
		t.Fatalf("Files.Ensure got the wrong entrypoint: %q", got)
	}
}

// --- password: set, verify, and persistence across a fresh Service -----

func TestSetPasswordThenAuthorizeAcceptsTheRightPasswordAndRejectsTheWrongOne(t *testing.T) {
	svc, _, _ := newService(t)
	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Shared link", Visibility: artifact.ByPassword})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetPassword(ctx(), artifact.SetPasswordInput{ID: created.ID, Password: "correct-horse-battery-staple"}); err != nil {
		t.Fatal(err)
	}

	stored, err := svc.Get(ctx(), artifact.GetInput{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Authorize(ctx(), stored, artifact.AccessRequest{Password: "correct-horse-battery-staple"}); err != nil {
		t.Fatalf("the right password was rejected: %v", err)
	}
	if err := svc.Authorize(ctx(), stored, artifact.AccessRequest{Password: "wrong"}); err == nil {
		t.Fatal("the wrong password was accepted")
	}
}

// TestPasswordSurvivesAFreshServiceOverTheSameStore is the regression for
// defect #19: the original derives its by_password secret fresh on every
// boot and never writes it down, so a link shared before a restart stops
// working after one. Here, the hash is persisted in the repository, and a
// second *Service built over the same backing store — standing in for the
// daemon restarting, with the fake repo standing in for disk that survives
// it — must still accept the password the first Service set.
func TestPasswordSurvivesAFreshServiceOverTheSameStore(t *testing.T) {
	repo := fakes.NewRepo[artifact.Artifact]("artifacts")
	files := newFakeFiles()
	svc1 := artifact.NewService(artifact.Deps{
		Repo: repo, Files: files, Hasher: artifact.Argon2Hasher{},
		Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "art"},
	})
	created, err := svc1.Create(ctx(), artifact.CreateInput{Name: "Survives a restart", Visibility: artifact.ByPassword})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc1.SetPassword(ctx(), artifact.SetPasswordInput{ID: created.ID, Password: "still-works-after-restart"}); err != nil {
		t.Fatal(err)
	}

	// A fresh Service, same repo: simulates the process restarting without
	// losing what was on disk.
	svc2 := artifact.NewService(artifact.Deps{
		Repo: repo, Files: files, Hasher: artifact.Argon2Hasher{},
		Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "art"},
	})
	after, err := svc2.Get(ctx(), artifact.GetInput{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc2.Authorize(ctx(), after, artifact.AccessRequest{Password: "still-works-after-restart"}); err != nil {
		t.Fatalf("password did not survive: %v", err)
	}
}

// --- the three visibilities --------------------------------------------

func TestPrivateRequiresAuthentication(t *testing.T) {
	svc, _, _ := newService(t)
	a, err := svc.Create(ctx(), artifact.CreateInput{Name: "Private", Visibility: artifact.Private})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Authorize(ctx(), a, artifact.AccessRequest{Authenticated: false}); err == nil {
		t.Fatal("unauthenticated request was allowed against a private artifact")
	}
	if err := svc.Authorize(ctx(), a, artifact.AccessRequest{Authenticated: true}); err != nil {
		t.Fatalf("authenticated request was refused: %v", err)
	}
}

func TestWorkspaceRequiresAuthentication(t *testing.T) {
	svc, _, _ := newService(t)
	a, err := svc.Create(ctx(), artifact.CreateInput{Name: "Workspace-wide", Visibility: artifact.Workspace})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Authorize(ctx(), a, artifact.AccessRequest{Authenticated: false}); err == nil {
		t.Fatal("unauthenticated request was allowed against a workspace artifact")
	}
	if err := svc.Authorize(ctx(), a, artifact.AccessRequest{Authenticated: true}); err != nil {
		t.Fatalf("authenticated member was refused: %v", err)
	}
}

func TestByPasswordWithNoPasswordSetIsRefused(t *testing.T) {
	svc, _, _ := newService(t)
	a, err := svc.Create(ctx(), artifact.CreateInput{Name: "No password yet", Visibility: artifact.ByPassword})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Authorize(ctx(), a, artifact.AccessRequest{Password: "anything"})
	if err == nil {
		t.Fatal("an artifact with no password set granted access")
	}
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("ARTIFACT_PASSWORD_REQUIRED").Code {
		t.Fatalf("want ARTIFACT_PASSWORD_REQUIRED, got %v", err)
	}
}

func TestInvalidVisibilityIsRefused(t *testing.T) {
	svc, _, _ := newService(t)
	_, err := svc.Create(ctx(), artifact.CreateInput{Name: "Bad visibility", Visibility: artifact.Visibility("public")})
	if err == nil {
		t.Fatal("an unknown visibility was accepted")
	}
}

// failRepo wraps a real Repository and forces one named method to fail, so a
// test can exercise the write/read-failure wrapping (errReadFailed,
// errWriteFailed) without a real filesystem error to provoke it.
type failRepo struct {
	artifact.Repository
	fail string
	err  error
}

func (f failRepo) List(ctx context.Context, q collections.Query) ([]artifact.Artifact, error) {
	if f.fail == "List" {
		return nil, f.err
	}
	return f.Repository.List(ctx, q)
}

func (f failRepo) Update(ctx context.Context, v *artifact.Artifact, expect collections.Version) error {
	if f.fail == "Update" {
		return f.err
	}
	return f.Repository.Update(ctx, v, expect)
}

func (f failRepo) Delete(ctx context.Context, key collections.Key) error {
	if f.fail == "Delete" {
		return f.err
	}
	return f.Repository.Delete(ctx, key)
}

func TestListWrapsARepositoryFailure(t *testing.T) {
	repo := failRepo{Repository: fakes.NewRepo[artifact.Artifact]("artifacts"), fail: "List", err: errors.New("disk gone")}
	svc := artifact.NewService(artifact.Deps{Repo: repo, Files: newFakeFiles(), Hasher: artifact.Argon2Hasher{}, Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"}})
	_, err := svc.List(ctx(), artifact.ListInput{})
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("ARTIFACT_READ_FAILED").Code {
		t.Fatalf("want ARTIFACT_READ_FAILED, got %v", err)
	}
}

func TestUpdateWrapsARepositoryFailure(t *testing.T) {
	real := fakes.NewRepo[artifact.Artifact]("artifacts")
	svc := artifact.NewService(artifact.Deps{Repo: real, Files: newFakeFiles(), Hasher: artifact.Argon2Hasher{}, Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"}})
	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Doomed update"})
	if err != nil {
		t.Fatal(err)
	}

	failing := artifact.NewService(artifact.Deps{
		Repo:  failRepo{Repository: real, fail: "Update", err: errors.New("disk gone")},
		Files: newFakeFiles(), Hasher: artifact.Argon2Hasher{}, Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"},
	})
	newName := "won't stick"
	_, err = failing.Update(ctx(), artifact.UpdateInput{ID: created.ID, Name: &newName})
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("ARTIFACT_WRITE_FAILED").Code {
		t.Fatalf("want ARTIFACT_WRITE_FAILED, got %v", err)
	}
}

func TestDeleteWrapsARepositoryFailure(t *testing.T) {
	real := fakes.NewRepo[artifact.Artifact]("artifacts")
	svc := artifact.NewService(artifact.Deps{Repo: real, Files: newFakeFiles(), Hasher: artifact.Argon2Hasher{}, Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"}})
	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Doomed delete"})
	if err != nil {
		t.Fatal(err)
	}

	failing := artifact.NewService(artifact.Deps{
		Repo:  failRepo{Repository: real, fail: "Delete", err: errors.New("disk gone")},
		Files: newFakeFiles(), Hasher: artifact.Argon2Hasher{}, Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"},
	})
	_, err = failing.Delete(ctx(), artifact.DeleteInput{ID: created.ID})
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("ARTIFACT_WRITE_FAILED").Code {
		t.Fatalf("want ARTIFACT_WRITE_FAILED, got %v", err)
	}
}

func TestUpdateChangesEveryOptionalField(t *testing.T) {
	svc, _, _ := newService(t)
	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Original", Entrypoint: "index.html", Visibility: artifact.Private})
	if err != nil {
		t.Fatal(err)
	}

	newDesc := "a new description"
	newEntry := "landing.html"
	newVis := artifact.Workspace
	updated, err := svc.Update(ctx(), artifact.UpdateInput{
		ID: created.ID, Description: &newDesc, Entrypoint: &newEntry, Visibility: &newVis,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != newDesc || updated.Entrypoint != newEntry || updated.Visibility != newVis {
		t.Fatalf("got %+v", updated)
	}
}

// failFiles is an artifact.Files whose Ensure always fails, to exercise
// Create's errScaffoldFailed wrapping.
type failFiles struct{ err error }

func (f failFiles) Ensure(context.Context, string, string) (string, error) { return "", f.err }
func (f failFiles) Remove(context.Context, string) error                   { return nil }

func TestCreateWrapsAScaffoldFailure(t *testing.T) {
	svc := artifact.NewService(artifact.Deps{
		Repo: fakes.NewRepo[artifact.Artifact]("artifacts"), Files: failFiles{err: errors.New("disk full")},
		Hasher: artifact.Argon2Hasher{}, Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"},
	})
	_, err := svc.Create(ctx(), artifact.CreateInput{Name: "Doomed scaffold"})
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("ARTIFACT_SCAFFOLD_FAILED").Code {
		t.Fatalf("want ARTIFACT_SCAFFOLD_FAILED, got %v", err)
	}
}

// failHasher is an artifact.PasswordHasher whose Hash always fails, to
// exercise SetPassword's errHashFailed wrapping.
type failHasher struct{ err error }

func (f failHasher) Hash(string) (string, error)         { return "", f.err }
func (f failHasher) Verify(string, string) (bool, error) { return false, nil }

func TestSetPasswordWrapsAHashFailure(t *testing.T) {
	repo := fakes.NewRepo[artifact.Artifact]("artifacts")
	svc := artifact.NewService(artifact.Deps{
		Repo: repo, Files: newFakeFiles(), Hasher: artifact.Argon2Hasher{},
		Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"},
	})
	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Doomed password", Visibility: artifact.ByPassword})
	if err != nil {
		t.Fatal(err)
	}

	failing := artifact.NewService(artifact.Deps{
		Repo: repo, Files: newFakeFiles(), Hasher: failHasher{err: errors.New("kdf exploded")},
		Clock: clockx.Fixed{At: at}, IDs: &ids.Sequence{Prefix: "a"},
	})
	_, err = failing.SetPassword(ctx(), artifact.SetPasswordInput{ID: created.ID, Password: "whatever"})
	got, ok := apperr.As(err)
	if !ok || got.Code != apperr.New("ARTIFACT_HASH_FAILED").Code {
		t.Fatalf("want ARTIFACT_HASH_FAILED, got %v", err)
	}
}

func TestVerifyRejectsMalformedStoredHashes(t *testing.T) {
	for name, stored := range map[string]string{
		"empty":              "",
		"wrong field count":  "$argon2id$v=19$m=1,t=1,p=1$onlyfivefields",
		"wrong algorithm":    "$bcrypt$v=19$m=65536,t=2,p=1$c2FsdA$a2V5",
		"wrong version":      "$argon2id$v=1$m=65536,t=2,p=1$c2FsdA$a2V5",
		"unparseable params": "$argon2id$v=19$not-params$c2FsdA$a2V5",
	} {
		t.Run(name, func(t *testing.T) {
			ok, err := artifact.Argon2Hasher{}.Verify("anything", stored)
			if err != nil {
				t.Fatalf("Verify returned an error for a malformed hash, want (false, nil): %v", err)
			}
			if ok {
				t.Fatal("a malformed stored hash verified successfully")
			}
		})
	}
}

func TestUpdateRejectsAnInvalidVisibility(t *testing.T) {
	svc, _, _ := newService(t)
	created, err := svc.Create(ctx(), artifact.CreateInput{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	bad := artifact.Visibility("public")
	_, err = svc.Update(ctx(), artifact.UpdateInput{ID: created.ID, Visibility: &bad})
	if err == nil {
		t.Fatal("an unknown visibility was accepted on update")
	}
}
