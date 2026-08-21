package artifact_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
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
