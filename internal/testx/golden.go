// Package testx holds the test helpers shared across the tree: golden files,
// stable serialisation and workspace fixtures.
//
// These exist for the artefacts that have no natural assertion. Nobody can
// test that "the prompt is good"; what can be tested is that it changed, and
// that the change was reviewed.
package testx

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files instead of comparing")

// Assert compares got with testdata/<name>.golden.
//
// With -update it rewrites the file, so refreshing a golden is a deliberate act
// that shows up as a reviewable diff in the commit — never a silent adjustment
// inside a test run. The workflow is: run with -update, read the git diff,
// decide whether the change was intended.
func Assert(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s — run the test with -update and review the diff: %v", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("%s does not match the golden file.\n--- want ---\n%s\n--- got ---\n%s\n"+
			"If the change is intended, rerun with -update and review the diff.",
			path, firstLines(want, 40), firstLines(got, 40))
	}
}

// AssertString is Assert for text.
func AssertString(t *testing.T, name, got string) {
	t.Helper()
	Assert(t, name, []byte(got))
}

func firstLines(b []byte, n int) string {
	lines := bytes.SplitN(b, []byte("\n"), n+1)
	if len(lines) > n {
		lines = append(lines[:n], []byte("… (truncated)"))
	}
	return string(bytes.Join(lines, []byte("\n")))
}

// Updating reports whether the run is refreshing golden files, so a test that
// needs to skip a comparison can tell.
func Updating() bool { return *update }
