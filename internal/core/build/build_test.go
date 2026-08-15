package build_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/build"
)

// TestBrandIsTheSingleSourceOfTheProductName is the guard behind ADR-0000:
// renaming the product must be one edit, so nothing may derive the name by
// hand. The constants below are what every other package reads.
func TestBrandIsTheSingleSourceOfTheProductName(t *testing.T) {
	if build.Name == "" || build.DisplayName == "" || build.ErrorPrefix == "" {
		t.Fatal("brand constants must not be empty")
	}
	if !strings.HasPrefix(build.StateDir, ".") {
		t.Errorf("state dir %q should be a dotted directory", build.StateDir)
	}
	if build.StateDir == ".fractal" {
		t.Fatal("the state directory must never be the original product's")
	}
	if build.Port != 5326 {
		t.Errorf("port = %d, want 5326", build.Port)
	}
}

func TestCurrentReportsThePlatform(t *testing.T) {
	info := build.Current()
	if !strings.Contains(info.Platform, "/") {
		t.Errorf("platform = %q, want GOOS/GOARCH", info.Platform)
	}
	if info.Go == "" {
		t.Error("go version is empty")
	}
	if info.Name != build.Name || info.Port != build.Port {
		t.Errorf("info does not carry the brand: %+v", info)
	}
	if !strings.Contains(info.String(), build.DisplayName) {
		t.Errorf("String() = %q", info.String())
	}
}

// TestDeveloperBuildsAreNotReleases: a dirty tree or an untagged commit must
// never claim to be a release, because the auto-updater and the MCP doctor
// branch on it.
func TestDeveloperBuildsAreNotReleases(t *testing.T) {
	if build.Current().IsRelease {
		t.Errorf("a build under `go test` reported itself as a release (version=%q)", build.Version)
	}
}
