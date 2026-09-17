package contentpolicy_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/aos/internal/transport/fileapi/contentpolicy"
)

func TestOnlyPassiveMediaGoesOutUnsandboxed(t *testing.T) {
	cases := []struct {
		contentType string
		sandboxed   bool
	}{
		{"text/html; charset=utf-8", true},
		{"application/xhtml+xml", true},
		{"image/svg+xml", true},
		{"application/xml", true},
		{"text/xml; charset=utf-8", true},
		{"application/rss+xml", true},
		{"text/plain; charset=utf-8", true},
		{"text/javascript; charset=utf-8", true},
		{"application/octet-stream", true},
		{"", true},
		{"not a media type", true},
		{"image/png", false},
		{"IMAGE/JPEG", false},
		{"image/webp", false},
		{"audio/mpeg", false},
		{"video/mp4", false},
		{"application/pdf", false},
	}
	for _, tc := range cases {
		h := http.Header{}
		if tc.contentType != "" {
			h.Set("Content-Type", tc.contentType)
		}
		contentpolicy.Apply(h)

		want := ""
		if tc.sandboxed {
			want = contentpolicy.Sandbox
		}
		if got := h.Get("Content-Security-Policy"); got != want {
			t.Errorf("%q: Content-Security-Policy = %q, want %q", tc.contentType, got, want)
		}
		if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%q: X-Content-Type-Options = %q, want nosniff", tc.contentType, got)
		}
	}
}

// A policy already on the answer — the one a daemon relayed, say — does not
// survive for a document: the sandbox replaces it rather than joining it.
func TestTheSandboxReplacesAnyPolicyAlreadyThere(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "text/html")
	h.Set("Content-Security-Policy", "script-src *")

	contentpolicy.Apply(h)

	if got := h.Values("Content-Security-Policy"); len(got) != 1 || got[0] != contentpolicy.Sandbox {
		t.Fatalf("Content-Security-Policy = %q, want only %q", got, contentpolicy.Sandbox)
	}
}
