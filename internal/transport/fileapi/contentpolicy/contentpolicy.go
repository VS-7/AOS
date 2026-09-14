// Package contentpolicy is the guard on a workspace file served as its own
// bytes: the headers that keep it from running as the origin that serves it.
//
// Two origins serve those bytes — the daemon's /api/file/content, and the
// desktop window's asset host, which forwards that route with the window's
// credential — and both apply this, so neither trusts the other to have. It is
// a package of its own, with nothing but the standard library under it,
// because the window may not link the file domain fileapi is built on.
package contentpolicy

import (
	"mime"
	"net/http"
	"strings"
)

// Sandbox is the Content-Security-Policy a workspace document is answered
// under: an opaque origin, with no script, form, popup or navigation of its own.
const Sandbox = "sandbox"

// Apply sets on h, whose Content-Type is already the one the answer goes out
// with, the headers that keep a workspace file inert.
//
// A workspace file is whatever an agent chose to write. Answered from the API's
// origin as text/html, a script in it ran as that origin: in a browser signed
// in to the daemon, fetch("/api/auth/session") read the person back, and in the
// window a same-origin call to /wails/runtime ran any command they can. An SVG
// or an XML file is a document that runs script as well, and a type a browser
// does not recognise may be sniffed into one.
//
// So every answer is nosniff, and every answer that is not plain media is
// sandboxed without allow-same-origin or allow-scripts — which also rules out
// a script in a preview that frames it, since a frame's own sandbox can only
// narrow this one. The allowlist is media rather than a list of dangerous types
// on purpose: a type nobody thought of stays inert. Images, audio, video and
// PDF are left unsandboxed because they run no script as a document, and
// because a sandboxed PDF does not render at all.
func Apply(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	if !media(h.Get("Content-Type")) {
		h.Set("Content-Security-Policy", Sandbox)
	}
}

// media reports whether contentType is one a browser renders as an image, a
// sound, a video or a PDF. An XML-based image (image/svg+xml) is a document.
func media(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	if mediaType == "application/pdf" {
		return true
	}
	kind, subtype, _ := strings.Cut(mediaType, "/")
	switch kind {
	case "image", "audio", "video":
		return !strings.Contains(subtype, "xml")
	}
	return false
}
