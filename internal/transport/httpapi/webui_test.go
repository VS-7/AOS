package httpapi_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/OWNER/aos/internal/transport/httpapi"
)

// bundle is a frontend bundle in the shape the build produces: a document, a
// content-hashed script that compressed, and an image that did not.
func bundle(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"index.html.gz":             {Data: gz(t, "<!doctype html><div id=root></div>")},
		"assets/index-abc123.js.gz": {Data: gz(t, "export const answer = 42;")},
		"assets/logo-def456.png":    {Data: []byte("\x89PNG\r\n\x1a\n not really")},
	}
}

func gz(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := io.WriteString(w, body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func withInterface(t *testing.T, files fstest.MapFS) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(httpapi.New(httpapi.Config{
		Registry:        registry(t),
		SecurityEnabled: func() bool { return false },
		Interface:       files,
	}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

// get issues a request without the transport's automatic gzip, so a test can
// say for itself what it accepts and see what came back on the wire.
func get(t *testing.T, srv *httptest.Server, path, acceptEncoding string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	res, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func bodyOf(t *testing.T, res *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestTheDaemonServesTheInterface is what this exists for.
//
// Until it did, the interface lived only inside the desktop binary and a
// daemon on a server answered the API and nothing else — there was no way to
// use the system on a machine you do not sit in front of.
func TestTheDaemonServesTheInterface(t *testing.T) {
	srv := withInterface(t, bundle(t))

	res := get(t, srv, "/", "gzip")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("content type = %q, want html", ct)
	}
}

// TestAnUnknownRouteGetsTheDocumentThatKnowsIt. A single-page application
// routes /tasks/42 in the browser; the server has never heard of it and must
// answer with the document rather than a 404, or every deep link and every
// refresh is broken.
func TestAnUnknownRouteGetsTheDocumentThatKnowsIt(t *testing.T) {
	srv := withInterface(t, bundle(t))

	res := get(t, srv, "/tasks/42", "gzip")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /tasks/42 = %d, want the document", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("content type = %q, want html", ct)
	}
}

// TestAMissingAPIRouteIsStillAnAPIAnswer is the defect the fallback would
// otherwise introduce, and the desktop already spent a release on its twin:
// a client parsing an HTML document as JSON reports nothing anyone can act on.
func TestAMissingAPIRouteIsStillAnAPIAnswer(t *testing.T) {
	srv := withInterface(t, bundle(t))

	res := get(t, srv, "/api/nope", "gzip")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /api/nope = %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct == "text/html; charset=utf-8" {
		t.Error("a missing API route answered with the interface")
	}
}

// TestTheBundleIsServedPreCompressed. The bundle is 53 MB and gzips to 14,
// which is what makes a single-file server binary something somebody will
// download — and compressing once at build time rather than per request is
// the right shape for a page fetched over a network.
func TestTheBundleIsServedPreCompressed(t *testing.T) {
	srv := withInterface(t, bundle(t))

	res := get(t, srv, "/assets/index-abc123.js", "gzip, deflate")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if enc := res.Header.Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("content encoding = %q, want gzip", enc)
	}
	if v := res.Header.Get("Vary"); v != "Accept-Encoding" {
		t.Errorf("Vary = %q — a cache that does not know this hands gzip to a client that cannot read it", v)
	}

	unzipped, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(unzipped)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "export const answer = 42;" {
		t.Errorf("body = %q", body)
	}
}

// TestAClientThatCannotReadGzipGetsTheFileAnyway — curl without --compressed,
// a health check, an old proxy. The bundle only exists compressed, so the
// alternative to decompressing here is handing them bytes they cannot read.
func TestAClientThatCannotReadGzipGetsTheFileAnyway(t *testing.T) {
	srv := withInterface(t, bundle(t))

	res := get(t, srv, "/assets/index-abc123.js", "identity")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("content encoding = %q, want none", enc)
	}
	if got := bodyOf(t, res); got != "export const answer = 42;" {
		t.Errorf("body = %q, want the decompressed file", got)
	}
}

// TestAFileThatDidNotCompressIsServedAsItIs. Not everything shrinks; the build
// leaves those alone rather than storing a .gz that is bigger than the file.
func TestAFileThatDidNotCompressIsServedAsItIs(t *testing.T) {
	srv := withInterface(t, bundle(t))

	res := get(t, srv, "/assets/logo-def456.png", "gzip")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Errorf("content encoding = %q, want none", enc)
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("content type = %q, want image/png", ct)
	}
}

// TestTheDocumentIsNeverCachedAndTheHashedAssetsAlwaysAre.
//
// The bundle's file names carry a content hash, so a name that resolves can
// never change meaning; index.html carries none and is what names the current
// hashes. Holding it is how a browser ends up asking for the previous
// release's assets after a deploy.
func TestTheDocumentIsNeverCachedAndTheHashedAssetsAlwaysAre(t *testing.T) {
	srv := withInterface(t, bundle(t))

	if got := get(t, srv, "/", "gzip").Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("index Cache-Control = %q, want no-store", got)
	}
	got := get(t, srv, "/assets/index-abc123.js", "gzip").Header.Get("Cache-Control")
	if got != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q, want immutable", got)
	}
}

// TestNothingEscapesTheBundle. The paths arrive from a browser and a browser
// is not the only thing that can send one.
func TestNothingEscapesTheBundle(t *testing.T) {
	srv := withInterface(t, bundle(t))

	for _, path := range []string{
		"/../go.mod",
		"/assets/../../go.mod",
		"/%2e%2e/go.mod",
	} {
		res := get(t, srv, path, "gzip")
		body := bodyOf(t, res)
		if bytes.Contains([]byte(body), []byte("module github.com/OWNER/aos")) {
			t.Errorf("%s escaped the bundle", path)
		}
	}
}

// TestADaemonWithNoInterfaceIsUnchanged. The daemon the desktop supervises
// carries no bundle — that window has its own copy and loads it off its own
// scheme — and it must go on answering the way it always did.
func TestADaemonWithNoInterfaceIsUnchanged(t *testing.T) {
	srv := httptest.NewServer(httpapi.New(httpapi.Config{
		Registry:        registry(t),
		SecurityEnabled: func() bool { return false },
	}).Handler())
	t.Cleanup(srv.Close)

	res := get(t, srv, "/", "gzip")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("GET / = %d, want 404 from a daemon with no interface", res.StatusCode)
	}
}
