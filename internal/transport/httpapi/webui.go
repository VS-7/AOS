package httpapi

import (
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// The web interface, served by the daemon.
//
// Until this existed the interface lived in exactly one place — embedded in
// the desktop binary, served to its own webview over the `wails://` scheme —
// and a daemon on a server answered the API and nothing else. Which is to say
// there was no way to *use* the system on a machine you do not sit in front
// of, only to call it.
//
// Everything else that browser use needs was already here and had been for a
// while: the interface asks for `/api/...` relative to whatever origin served
// it (frontend's lib/daemon-origin.ts), bearerOf accepts a session cookie, and
// guardExposure refuses a non-loopback bind while security is off. The missing
// piece was literally handing over the files.
//
// They arrive pre-compressed. The bundle is 53 MB and gzips to 14, which is
// the difference between a server binary somebody will download and one they
// will not — and compressing once at build time rather than per request is
// also the right shape for the thing this is now: a page fetched over a
// network rather than off the local disk.

// spaFallback is served for any path the bundle has no file for. A single-page
// application routes /tasks/42 in the browser; the server has never heard of
// it and must answer with the document that will.
const spaFallback = "index.html"

// serveInterface answers a request from the embedded bundle.
//
// It reports whether it handled the request. False means "not mine" and leaves
// the caller to produce its own answer — which is how /api/nope keeps getting
// a JSON refusal rather than an HTML page.
func (s *Server) serveInterface(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.Interface == nil {
		return false
	}
	// A missing API route is an API answer. Without this the interface's own
	// index.html would be returned with a 200 for every mistyped endpoint,
	// which is the failure mode the desktop spent a release with: a client
	// parsing HTML as JSON and reporting nothing useful.
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = spaFallback
	}
	if !fs.ValidPath(name) {
		// path.Clean has already resolved "..", so this is a name the
		// filesystem would refuse rather than an attempt to escape. Answering
		// with the document is what a browser asked a nonsense URL deserves.
		name = spaFallback
	}

	if s.writeAsset(w, r, name) {
		return true
	}
	// Not a file in the bundle: a client-side route. The document decides.
	return s.writeAsset(w, r, spaFallback)
}

// writeAsset serves one file, preferring the pre-compressed copy beside it.
func (s *Server) writeAsset(w http.ResponseWriter, r *http.Request, name string) bool {
	body, encoding, ok := s.readAsset(r, name)
	if !ok {
		return false
	}
	defer func() { _ = body.Close() }()

	header := w.Header()
	header.Set("Content-Type", contentTypeOf(name))
	// Correct even when this particular response is uncompressed: the same URL
	// answers differently depending on the request's Accept-Encoding, and a
	// cache that does not know that will hand gzip to a client that cannot
	// read it.
	header.Set("Vary", "Accept-Encoding")
	if encoding != "" {
		header.Set("Content-Encoding", encoding)
	}
	// The bundle's own file names carry a content hash, so a name that
	// resolves can never change meaning. index.html carries none and is the
	// document that names the current hashes, so it must never be held.
	if name == spaFallback {
		header.Set("Cache-Control", "no-store")
	} else {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	// Served to a browser over a network now, not to a webview off the local
	// disk. A bundle whose types are guessed is a bundle where an uploaded
	// file can be talked into executing.
	header.Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return true
	}
	_, _ = io.Copy(w, body)
	return true
}

// readAsset opens `name`, or its pre-compressed copy, in whichever form this
// client can read.
//
// The returned encoding is what to declare in Content-Encoding, empty for
// identity.
func (s *Server) readAsset(r *http.Request, name string) (io.ReadCloser, string, bool) {
	if compressed, err := s.cfg.Interface.Open(name + ".gz"); err == nil {
		if acceptsGzip(r) {
			return compressed, "gzip", true
		}
		// Something that does not speak gzip — curl without --compressed, a
		// health check, an old proxy. Decompressing here costs this one
		// request and keeps the bundle honest for everything else.
		unzipped, err := gzip.NewReader(compressed)
		if err != nil {
			_ = compressed.Close()
			return nil, "", false
		}
		return pairedCloser{Reader: unzipped, closers: []io.Closer{unzipped, compressed}}, "", true
	}

	// Not everything compresses. The build leaves a file uncompressed when
	// gzip made it no smaller — already-compressed images, mostly.
	if raw, err := s.cfg.Interface.Open(name); err == nil {
		if info, err := fs.Stat(s.cfg.Interface, name); err == nil && info.IsDir() {
			_ = raw.Close()
			return nil, "", false
		}
		return raw, "", true
	}
	return nil, "", false
}

// pairedCloser closes the decompressor and the file underneath it. Closing
// only the gzip reader leaves the embedded file open, which for an embed.FS is
// harmless and for a directory-backed one is a leaked descriptor per request.
type pairedCloser struct {
	io.Reader
	closers []io.Closer
}

func (p pairedCloser) Close() error {
	var err error
	for _, c := range p.closers {
		if closeErr := c.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(name, "gzip") {
			return true
		}
	}
	return false
}

// contentTypeOf answers from the name the file has in the bundle, never from
// its bytes: the compressed copy's bytes are gzip whatever the file is, and
// sniffing them would label every script as application/gzip.
func contentTypeOf(name string) string {
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	switch path.Ext(name) {
	// The two the standard table has been observed to miss on a bare
	// container, which is exactly where this daemon runs.
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json"
	case ".html":
		return "text/html; charset=utf-8"
	}
	return "application/octet-stream"
}
