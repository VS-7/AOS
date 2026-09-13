package build_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/build"
)

// The shapes this tree actually stamps: a release tag, the same tag from a
// locally packaged tree (Taskfile's --dirty), a build some commits past a tag
// (git describe), and the placeholders a build without a tag carries.
func TestParseVersionReadsTheReleaseNumbers(t *testing.T) {
	cases := []struct {
		in                  string
		major, minor, patch int
	}{
		{"v0.15.2-fase9", 0, 15, 2},
		{"v0.15.0-fase9-dirty", 0, 15, 0},
		{"v0.9.0-fase7-132-g2b749c1-dirty", 0, 9, 0},
		{"0.16.1", 0, 16, 1},
		{"v1.2", 1, 2, 0},
	}
	for _, c := range cases {
		v, ok := build.ParseVersion(c.in)
		if !ok {
			t.Errorf("ParseVersion(%q) did not parse", c.in)
			continue
		}
		if v.Major != c.major || v.Minor != c.minor || v.Patch != c.patch {
			t.Errorf("ParseVersion(%q) = %d.%d.%d, want %d.%d.%d", c.in, v.Major, v.Minor, v.Patch, c.major, c.minor, c.patch)
		}
	}
}

func TestParseVersionRefusesWhatIsNotAVersion(t *testing.T) {
	for _, in := range []string{"dev", "", "2b749c1", "vX.1.0", "release-0.1"} {
		if _, ok := build.ParseVersion(in); ok {
			t.Errorf("ParseVersion(%q) parsed, want refused", in)
		}
	}
}

// Ordering is by the release numbers alone. A suffix names the same release
// seen from somewhere else — a dirty tree, a phase label, commits past the
// tag — and is never a reason to offer that release again.
func TestVersionCompareOrdersByTheReleaseNumbersOnly(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.15.0-fase9-dirty", "v0.15.2-fase9", -1},
		{"v0.15.2-fase9-dirty", "v0.15.2-fase9", 0},
		{"v0.15.2-fase9-3-gabc1234", "v0.15.2-fase9", 0},
		{"v0.15.2-fase9", "v0.1.0", 1},
		{"v0.9.9", "v0.10.0", -1},
		{"v1.0.0", "v0.99.99", 1},
	}
	for _, c := range cases {
		a, _ := build.ParseVersion(c.a)
		b, _ := build.ParseVersion(c.b)
		if got := a.Compare(b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
