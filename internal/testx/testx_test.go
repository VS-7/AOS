package testx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/internal/testx"
	"github.com/OWNER/aos/internal/testx/fixture"
)

// TestMarshalStableIsStable: Go randomises map iteration, so a golden built
// from a map would fail at random without this.
func TestMarshalStableIsStable(t *testing.T) {
	value := map[string]any{
		"zeta": 1, "alpha": 2, "mu": 3, "beta": map[string]any{"b": 1, "a": 2},
	}
	first, err := testx.MarshalStable(value)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		again, err := testx.MarshalStable(value)
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(first) {
			t.Fatalf("run %d differs:\n%s\n---\n%s", i, first, again)
		}
	}
}

// TestMarshalStableDoesNotEscapeHTML keeps a prompt containing "<" readable in
// review instead of turning it into <.
func TestMarshalStableDoesNotEscapeHTML(t *testing.T) {
	got, err := testx.MarshalStable(map[string]string{"xml": `<context trust="trusted">`})
	if err != nil {
		t.Fatal(err)
	}
	if want := `<context trust=\"trusted\">`; !contains(string(got), want) {
		t.Fatalf("got %s", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > 0 && indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// TestFixtureIsByteIdentical is what makes every golden derived from a fixture
// trustworthy.
func TestFixtureIsByteIdentical(t *testing.T) {
	a := fixture.Workspace(t, fixture.Typical)
	b := fixture.Workspace(t, fixture.Typical)

	filesA := readTree(t, a.StateDir())
	filesB := readTree(t, b.StateDir())

	if len(filesA) != len(filesB) {
		t.Fatalf("%d files then %d", len(filesA), len(filesB))
	}
	for rel, content := range filesA {
		other, ok := filesB[rel]
		if !ok {
			t.Errorf("%s is missing from the second fixture", rel)
			continue
		}
		if content != other {
			t.Errorf("%s differs between two builds of the same fixture", rel)
		}
	}
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out[filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFixtureSizes(t *testing.T) {
	minimal := fixture.Workspace(t, fixture.Minimal)
	if got := len(readTree(t, minimal.StateDir())); got != minimal.Total() {
		t.Errorf("minimal wrote %d files, expected %d", got, minimal.Total())
	}
	typical := fixture.Workspace(t, fixture.Typical)
	if got := len(readTree(t, typical.StateDir())); got != typical.Total() {
		t.Errorf("typical wrote %d files, expected %d", got, typical.Total())
	}
}

func TestGoldenAssertMatches(t *testing.T) {
	// The helper is exercised against a golden committed alongside this test.
	testx.AssertString(t, "hello", "hello, golden\n")
}
