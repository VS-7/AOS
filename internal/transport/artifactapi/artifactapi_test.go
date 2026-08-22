package artifactapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/transport/artifactapi"
)

// fakeArtifacts serves one artifact and decides authorization from a plain
// func — internal/domain/artifact's own suite already proves Authorize's
// visibility logic; what this package needs proven is that it builds the
// AccessRequest correctly and reacts correctly to the result, not that
// switch again.
type fakeArtifacts struct {
	artifact  *artifact.Artifact
	getErr    error
	authorize func(artifact.AccessRequest) error
}

func (f *fakeArtifacts) Get(_ context.Context, in artifact.GetInput) (*artifact.Artifact, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.artifact == nil || in.ID != f.artifact.ID {
		return nil, apperr.New("ARTIFACT_NOT_FOUND").Status(apperr.StatusNotFound)
	}
	return f.artifact, nil
}

func (f *fakeArtifacts) Authorize(_ context.Context, _ *artifact.Artifact, req artifact.AccessRequest) error {
	return f.authorize(req)
}

// fakeFiles resolves inside a real temp directory — http.ServeFile needs
// actual files on disk, and a fake in-memory http.FileSystem would be more
// machinery than this layer's own containment logic is worth testing twice
// (internal/adapters/artifactfiles' own tests prove pathx.ResolveInside
// itself refuses traversal; this fake mirrors that shape without depending
// on it).
type fakeFiles struct{ root string }

func (f fakeFiles) Resolve(id, p string) (string, error) {
	dir := filepath.Join(f.root, id)
	target := filepath.Join(dir, filepath.FromSlash(p))
	rel, err := filepath.Rel(dir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("outside the artifact's own directory")
	}
	return target, nil
}

type fakeAuth struct {
	// valid maps an accepted bearer to a user. Anything else is refused.
	valid map[string]bool
}

func (f fakeAuth) Authenticate(_ context.Context, bearer string) (*auth.User, error) {
	if f.valid[bearer] {
		return &auth.User{ID: "u1"}, nil
	}
	return nil, apperr.New("AUTH_INVALID_TOKEN").Status(apperr.StatusUnauthorized)
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func alwaysAllow(artifact.AccessRequest) error { return nil }

func TestServesTheEntrypointWhenNoSubPathIsGiven(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "<h1>hello</h1>")
	a := &artifact.Artifact{ID: "demo", Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "<h1>hello</h1>" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestServesAFileUnderTheEntrypoint(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "root")
	writeFile(t, root, "demo/assets/app.js", "console.log(1)")
	a := &artifact.Artifact{ID: "demo", Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/assets/app.js", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "console.log(1)" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("Content-Type = %q, want javascript", ct)
	}
}

func TestUnknownArtifactIs404(t *testing.T) {
	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{getErr: apperr.New("ARTIFACT_NOT_FOUND").Status(apperr.StatusNotFound)},
		Files:     fakeFiles{root: t.TempDir()},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/nope/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAuthorizeRefusalIsSurfacedWithItsOwnStatus(t *testing.T) {
	a := &artifact.Artifact{ID: "demo", Visibility: artifact.Private}
	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: func(artifact.AccessRequest) error {
			return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusForbidden)
		}},
		Files: fakeFiles{root: t.TempDir()},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestPathTraversalIsRefused proves the transport reacts correctly to a
// Files.Resolve refusal — the actual containment logic is
// internal/adapters/artifactfiles' own, proven against the real filesystem
// there.
func TestPathTraversalIsRefused(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "root")
	writeFile(t, root, "secret.txt", "should never be reachable")
	a := &artifact.Artifact{ID: "demo", Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/../../secret.txt", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a traversal attempt", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "should never be reachable") {
		t.Fatal("traversal reached a file outside the artifact's own directory")
	}
}

// TestADirectoryIsNeverListed: http.ServeFile lists a directory with no
// index.html by default — this route must refuse instead, so an artifact's
// file layout is never disclosed.
func TestADirectoryIsNeverListed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "root")
	writeFile(t, root, "demo/assets/app.js", "x")
	a := &artifact.Artifact{ID: "demo", Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/assets", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a directory request", rec.Code)
	}
}

func TestCSPAndSecurityHeadersArePresent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "hi")
	a := &artifact.Artifact{ID: "demo", Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	h.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no Content-Security-Policy header")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("CSP = %q, want same-origin default-src", csp)
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Fatalf("CSP = %q, must not allow unsafe-inline", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("X-Content-Type-Options missing")
	}
	if rec.Header().Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Fatal("Cross-Origin-Resource-Policy missing")
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("Referrer-Policy missing")
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("this route must never set a session cookie")
	}
}

func TestAnUnknownExtensionIsServedAsAnAttachment(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/data.weird-extension", "binary-ish content")
	a := &artifact.Artifact{ID: "demo", Entrypoint: "data.weird-extension"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment" {
		t.Fatalf("Content-Disposition = %q, want attachment", got)
	}
}

// --- authentication: header only, never a cookie, and the password comes
// from the query string instead --------------------------------------------

func TestAuthenticatedRequestNeedsTheBearerHeaderNotACookie(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "hi")
	a := &artifact.Artifact{ID: "demo", Visibility: artifact.Private, Entrypoint: "index.html"}

	var gotAuthenticated bool
	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: func(req artifact.AccessRequest) error {
			gotAuthenticated = req.Authenticated
			if !req.Authenticated {
				return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusForbidden)
			}
			return nil
		}},
		Files:           fakeFiles{root: root},
		Auth:            fakeAuth{valid: map[string]bool{"good-token": true}},
		SecurityEnabled: func() bool { return true },
	})

	// A session cookie alone must not authenticate this route.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	req.AddCookie(&http.Cookie{Name: "sessionToken", Value: "good-token"})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a cookie alone authenticated the request: status = %d", rec.Code)
	}
	if gotAuthenticated {
		t.Fatal("Authenticated was true from a cookie alone")
	}

	// The same credential, presented as a header, does authenticate it.
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !gotAuthenticated {
		t.Fatal("Authenticated was false with a valid bearer header")
	}
}

func TestAnInvalidBearerHeaderDoesNotAuthenticate(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "hi")
	a := &artifact.Artifact{ID: "demo", Visibility: artifact.Private, Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: func(req artifact.AccessRequest) error {
			if !req.Authenticated {
				return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusForbidden)
			}
			return nil
		}},
		Files:           fakeFiles{root: root},
		Auth:            fakeAuth{valid: map[string]bool{"good-token": true}},
		SecurityEnabled: func() bool { return true },
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestWithSecurityDisabledEveryRequestIsTreatedAsAuthenticated(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "hi")
	a := &artifact.Artifact{ID: "demo", Visibility: artifact.Private, Entrypoint: "index.html"}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: func(req artifact.AccessRequest) error {
			if !req.Authenticated {
				return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusForbidden)
			}
			return nil
		}},
		Files:           fakeFiles{root: root},
		Auth:            fakeAuth{},
		SecurityEnabled: func() bool { return false },
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with security disabled", rec.Code)
	}
}

func TestThePasswordComesFromTheQueryString(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "hi")
	a := &artifact.Artifact{ID: "demo", Visibility: artifact.ByPassword, PasswordHash: "irrelevant-to-this-fake", Entrypoint: "index.html"}

	var gotPassword string
	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: func(req artifact.AccessRequest) error {
			gotPassword = req.Password
			if req.Password != "letmein" {
				return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusForbidden)
			}
			return nil
		}},
		Files: fakeFiles{root: root},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/?password=letmein", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotPassword != "letmein" {
		t.Fatalf("gotPassword = %q", gotPassword)
	}
}
