package build_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/build"
)

func TestCompatibleSameMinor(t *testing.T) {
	cases := []struct{ client, daemon string }{
		{"v0.9.0", "v0.9.1"},
		{"v0.9.0-fase7-132-g2b749c1-dirty", "v0.9.5-fase9-3-gabc1234"},
		{"0.9.0", "v0.9.0"},
	}
	for _, c := range cases {
		if err := build.Compatible(c.client, c.daemon); err != nil {
			t.Errorf("Compatible(%q, %q) = %v, want nil", c.client, c.daemon, err)
		}
	}
}

func TestCompatibleDifferentMinorRefuses(t *testing.T) {
	if err := build.Compatible("v0.9.0", "v0.10.0"); err == nil {
		t.Fatal("expected an error for a minor version mismatch")
	}
	if err := build.Compatible("v1.2.0", "v2.2.0"); err == nil {
		t.Fatal("expected an error for a major version mismatch")
	}
}

// "dev" is what Version holds in `go run` and in every test (see version.go)
// — a developer build has no real version to compare against, so refusing
// here would block every local workflow instead of just the ones that
// matter.
func TestCompatibleDevBuildsAreAlwaysCompatible(t *testing.T) {
	cases := []struct{ client, daemon string }{
		{"dev", "dev"},
		{"dev", "v0.9.0"},
		{"v0.9.0", "dev"},
		{"", "v0.9.0"},
	}
	for _, c := range cases {
		if err := build.Compatible(c.client, c.daemon); err != nil {
			t.Errorf("Compatible(%q, %q) = %v, want nil", c.client, c.daemon, err)
		}
	}
}
