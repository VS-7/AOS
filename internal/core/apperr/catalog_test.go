package apperr_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/apperr/scan"
	"github.com/OWNER/aos/internal/domain/config"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func scanned(t *testing.T) []apperr.Entry {
	t.Helper()
	entries, err := scan.Scan(moduleRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// TestCatalogMatchesSource covers two of the three invariants at once: every
// code used in the tree is in the catalog, and every catalog entry is reachable
// from a call site. Both hold by construction when the committed catalog equals
// a fresh scan — which is what this asserts.
func TestCatalogMatchesSource(t *testing.T) {
	got := scanned(t)
	if len(got) != len(apperr.Catalog) {
		t.Fatalf("catalog is stale: source has %d codes, catalog has %d — run `task gen-catalog`",
			len(got), len(apperr.Catalog))
	}
	for i := range got {
		if got[i].Code != apperr.Catalog[i].Code || got[i].File != apperr.Catalog[i].File {
			t.Fatalf("catalog is stale at %d: source has %s (%s), catalog has %s (%s) — run `task gen-catalog`",
				i, got[i].Code, got[i].File, apperr.Catalog[i].Code, apperr.Catalog[i].File)
		}
	}
}

// TestEveryErrorHasCauser is the third invariant: an error without a causer is
// untraceable for a consumer that does not read stack traces — which is every
// LLM consuming this API.
func TestEveryErrorHasCauser(t *testing.T) {
	for _, e := range scanned(t) {
		if e.Causer == "" {
			t.Errorf("%s:%d: %s has no Causer", e.File, e.Line, e.Code)
		}
	}
}

// TestActionableErrorsHaveCTA: a 4xx means the caller can do something about
// it, so the error must say what. 5xx is exempt — there is no useful action in
// front of an internal failure, and suggesting one would be a lie.
func TestActionableErrorsHaveCTA(t *testing.T) {
	for _, e := range scanned(t) {
		if e.Status < 400 || e.Status >= 500 {
			continue
		}
		if !e.CTA {
			t.Errorf("%s:%d: %s is %d and carries no CTA", e.File, e.Line, e.Code, e.Status)
		}
	}
}

// TestNoSecretsInIssue walks the catalog and fails if any error puts a field
// tagged secret:"true" into its Issue map, which is serialised to the agent and
// to logs (ADR-0010).
func TestNoSecretsInIssue(t *testing.T) {
	banned := map[string]bool{}
	for _, path := range config.SecretPaths() {
		parts := strings.Split(path, ".")
		banned[strings.ToLower(parts[len(parts)-1])] = true
	}
	// Names that are secrets wherever they appear, not only in config.
	for _, extra := range []string{"password", "secret", "token", "apitoken", "key", "credential"} {
		banned[extra] = true
	}

	for _, e := range scanned(t) {
		for _, issue := range e.Issues {
			if banned[strings.ToLower(issue)] {
				t.Errorf("%s:%d: %s puts %q in Issue, which reaches agents and logs",
					e.File, e.Line, e.Code, issue)
			}
		}
	}
}

// TestEveryErrorUnwrapsToOneSentinel: callers branch on behaviour with
// errors.Is instead of parsing codes, which requires exactly one match.
func TestEveryErrorUnwrapsToOneSentinel(t *testing.T) {
	statuses := []int{0, 400, 401, 403, 404, 409, 422, 429, 500, 503, 504}
	for _, status := range statuses {
		err := apperr.New("SOME_CONDITION").Causer("test").Status(status)
		matches := 0
		for _, s := range apperr.Sentinels {
			if isSentinel(err, s) {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("status %d: matched %d sentinels, want exactly 1", status, matches)
		}
	}
}

// isSentinel inspects the direct unwrap set rather than using errors.Is, which
// would also follow the wrapped cause. The invariant is about which sentinel
// the error itself declares.
func isSentinel(err error, sentinel error) bool {
	type unwrapper interface{ Unwrap() []error }
	u, ok := err.(unwrapper)
	if !ok {
		return false
	}
	for _, e := range u.Unwrap() {
		if errors.Is(e, sentinel) {
			return true
		}
	}
	return false
}
