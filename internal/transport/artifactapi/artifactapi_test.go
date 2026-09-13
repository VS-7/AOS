package artifactapi_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

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

// The route follows the workspace the request names, the same routing every
// command gets. It served only the workspace the daemon started in, so an
// artifact from any other one answered AOS_ARTIFACT_NOT_FOUND.
func TestTheArtifactComesFromTheWorkspaceTheRequestNames(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/index.html", "from the second workspace")
	scoped := &fakeArtifacts{artifact: &artifact.Artifact{ID: "demo", Entrypoint: "index.html"}, authorize: alwaysAllow}

	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{authorize: alwaysAllow}, // the daemon's own: has no such artifact
		Files:     fakeFiles{root: t.TempDir()},
		Scope: func(context.Context) (artifactapi.Artifacts, artifactapi.Files, error) {
			return scoped, fakeFiles{root: root}, nil
		},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "from the second workspace" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}

func TestAWorkspaceThatCannotBeResolvedIsTheAnswer(t *testing.T) {
	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{authorize: alwaysAllow},
		Files:     fakeFiles{root: t.TempDir()},
		Scope: func(context.Context) (artifactapi.Artifacts, artifactapi.Files, error) {
			return nil, nil, apperr.New("WORKSPACE_UNAVAILABLE").Status(apperr.StatusServiceUnavailable)
		},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want the scope's own failure", rec.Code)
	}
}

// --- the origin an artifact runs in ------------------------------------------

// An artifact is served from the API's own origin, and a browser signed in to
// that origin attaches its session cookie to every same-origin request. So a
// by_password artifact whose page loaded `probe.js?password=pw`, opened at its
// share link, ran that script as the API's origin: fetch("/api/auth/session")
// answered 200 with the user, /api/tasks/list answered, and the page's own
// localStorage was readable. Framed by a browser tab with allow-same-origin,
// it reached window.parent too. The frame's sandbox cannot be the whole
// answer — a share link is opened top-level, with no frame at all — so the
// answer itself carries the sandbox: the document gets an opaque origin
// wherever it is opened, and nothing of the API's origin is its own.
func TestEveryArtifactAnswerRunsInAnOpaqueOrigin(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"index.html", "probe.js", "logo.svg", "data.weird"} {
		writeFile(t, root, "demo/"+name, "x")
	}
	for _, visibility := range []artifact.Visibility{artifact.Private, artifact.Workspace, artifact.ByPassword} {
		a := &artifact.Artifact{ID: "demo", Visibility: visibility, Entrypoint: "index.html"}
		h := artifactapi.New(artifactapi.Config{
			Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
			Files:     fakeFiles{root: root},
		})
		for _, path := range []string{"/artifacts/demo/", "/artifacts/demo/probe.js", "/artifacts/demo/logo.svg", "/artifacts/demo/data.weird"} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path+"?password=pw", nil))
			csp := rec.Header().Get("Content-Security-Policy")
			sandbox := ""
			for _, directive := range strings.Split(csp, ";") {
				if strings.HasPrefix(strings.TrimSpace(directive), "sandbox") {
					sandbox = strings.TrimSpace(directive)
				}
			}
			if sandbox == "" {
				t.Errorf("%s %s: no sandbox in CSP %q; its script runs as the API's origin", visibility, path, csp)
				continue
			}
			if strings.Contains(sandbox, "allow-same-origin") || strings.Contains(sandbox, "allow-top-navigation") {
				t.Errorf("%s %s: sandbox %q hands the document an origin or the top window", visibility, path, sandbox)
			}
			if !strings.Contains(sandbox, "allow-scripts") {
				t.Errorf("%s %s: sandbox %q stops the artifact's own script", visibility, path, sandbox)
			}
		}
	}
}

// An opaque page loading its own stylesheet, script or image makes a
// cross-origin request, and a same-origin resource policy cancels it; its
// fetch of its own data is a CORS request. A by_password artifact is read with
// the password the request itself presents — whoever holds it can read the
// file from anywhere already — so its files answer any origin, without
// credentials. What is read on any other ground (a bearer, or security
// switched off) still answers this origin only: a page elsewhere must not be
// able to embed it.
func TestOnlyAPasswordReadArtifactAnswersItsOwnOpaquePage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "demo/data.json", `{"rows":3}`)

	shared := &artifact.Artifact{ID: "demo", Visibility: artifact.ByPassword, PasswordHash: "h", Entrypoint: "index.html"}
	h := artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: shared, authorize: func(req artifact.AccessRequest) error {
			if req.Password != "pw" {
				return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusUnauthorized)
			}
			return nil
		}},
		Files: fakeFiles{root: root},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/data.json?password=pw", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if corp := rec.Header().Get("Cross-Origin-Resource-Policy"); corp != "cross-origin" {
		t.Errorf("a password-read file answers Cross-Origin-Resource-Policy %q; its own opaque page cannot load it", corp)
	}
	if acao := rec.Header().Get("Access-Control-Allow-Origin"); acao != "*" {
		t.Errorf("a password-read file answers Access-Control-Allow-Origin %q; its own opaque page cannot fetch it", acao)
	}
	if creds := rec.Header().Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q; nothing here is read with a credential the browser attaches", creds)
	}

	refused := httptest.NewRecorder()
	h.ServeHTTP(refused, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/data.json", nil))
	if refused.Code != http.StatusUnauthorized || refused.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("without the password: status %d, ACAO %q", refused.Code, refused.Header().Get("Access-Control-Allow-Origin"))
	}

	for name, cfg := range map[string]artifactapi.Config{
		"bearer": {
			Auth:            fakeAuth{valid: map[string]bool{"good-token": true}},
			SecurityEnabled: func() bool { return true },
		},
		"security off": {
			Auth:            fakeAuth{},
			SecurityEnabled: func() bool { return false },
		},
	} {
		private := &artifact.Artifact{ID: "demo", Visibility: artifact.Private, Entrypoint: "index.html"}
		cfg.Artifacts = &fakeArtifacts{artifact: private, authorize: func(req artifact.AccessRequest) error {
			if !req.Authenticated {
				return apperr.New("ARTIFACT_UNAUTHORIZED").Status(apperr.StatusUnauthorized)
			}
			return nil
		}}
		cfg.Files = fakeFiles{root: root}
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/artifacts/demo/data.json", nil)
		req.Header.Set("Authorization", "Bearer good-token")
		artifactapi.New(cfg).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", name, rec.Code)
		}
		if corp := rec.Header().Get("Cross-Origin-Resource-Policy"); corp != "same-origin" {
			t.Errorf("%s: Cross-Origin-Resource-Policy = %q, want same-origin", name, corp)
		}
		if acao := rec.Header().Get("Access-Control-Allow-Origin"); acao != "" {
			t.Errorf("%s: Access-Control-Allow-Origin = %q, want none", name, acao)
		}
	}
}

// WebKit stops counting an opaque page's own URL as connect-src 'self' — its
// scripts, styles and images still match, its fetch does not — so a shared
// artifact's fetch of its own data.json was refused before it was sent. The
// policy names the artifact's own directory, absolutely, and nothing wider:
// /api stays out of reach there too. The address is the one the request was
// made to, and a Host that could smuggle a directive widens nothing.
func TestAnArtifactMayConnectToItsOwnDirectoryAndNothingWider(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "sales report/index.html", "hi")
	a := &artifact.Artifact{ID: "sales report", Visibility: artifact.ByPassword, PasswordHash: "h", Entrypoint: "index.html"}
	mux := chi.NewRouter()
	mux.Mount("/v", artifactapi.New(artifactapi.Config{
		Artifacts: &fakeArtifacts{artifact: a, authorize: alwaysAllow},
		Files:     fakeFiles{root: root},
	}))

	connectSrc := func(host string, tls bool, forwarded string) string {
		t.Helper()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v/artifacts/sales%20report/?password=pw", nil)
		req.Host = host
		if tls {
			req.TLS = &tlsState
		}
		if forwarded != "" {
			req.Header.Set("X-Forwarded-Proto", forwarded)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		for _, directive := range strings.Split(rec.Header().Get("Content-Security-Policy"), ";") {
			if d := strings.TrimSpace(directive); strings.HasPrefix(d, "connect-src") {
				return d
			}
		}
		return ""
	}

	if got, want := connectSrc("127.0.0.1:5326", false, ""), "connect-src 'self' http://127.0.0.1:5326/v/artifacts/sales%20report/"; got != want {
		t.Errorf("connect-src = %q, want %q", got, want)
	}
	if got, want := connectSrc("aos.example.com", false, "https"), "connect-src 'self' https://aos.example.com/v/artifacts/sales%20report/"; got != want {
		t.Errorf("behind a tunnel: connect-src = %q, want %q", got, want)
	}
	if got, want := connectSrc("aos.example.com", true, ""), "connect-src 'self' https://aos.example.com/v/artifacts/sales%20report/"; got != want {
		t.Errorf("over TLS: connect-src = %q, want %q", got, want)
	}
	for _, host := range []string{"evil;script-src", "a,b", "", "x/y"} {
		if got := connectSrc(host, false, "javascript"); got != "connect-src 'self'" {
			t.Errorf("Host %q: connect-src = %q", host, got)
		}
	}
}

var tlsState tls.ConnectionState
