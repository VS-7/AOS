package build

import (
	"fmt"
	"runtime"
	"strings"
)

// Version, Commit and Date are injected at link time by the Taskfile:
//
//	-X github.com/OWNER/aos/internal/core/build.Version={{.VERSION}}
//
// They keep their placeholder values in `go run` and in tests, which is how a
// developer build is told apart from a released one.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the machine-readable form of the build stamp. It is what `aos version
// --json` prints and what the gateway records so a client can detect that the
// registered binary and the running daemon are different builds (ADR-0011).
type Info struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	Go        string `json:"go"`
	Platform  string `json:"platform"`
	StateDir  string `json:"stateDir"`
	Port      int    `json:"port"`
	IsRelease bool   `json:"isRelease"`
}

// Current returns the build stamp of this binary.
func Current() Info {
	return Info{
		Name:      Name,
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		Go:        runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		StateDir:  StateDir,
		Port:      Port,
		IsRelease: isRelease(Version),
	}
}

// isRelease reports whether this binary was built from a clean, tagged commit.
// git describe emits "-dirty" for a working tree with changes and falls back to
// the bare commit hash when no tag is reachable; neither is a release.
func isRelease(version string) bool {
	return strings.HasPrefix(version, "v") && !strings.HasSuffix(version, "-dirty")
}

// String renders the one-line human form: "AOS 0.1.0 (abc1234, darwin/arm64)".
func (i Info) String() string {
	return fmt.Sprintf("%s %s (%s, %s)", DisplayName, i.Version, i.Commit, i.Platform)
}
