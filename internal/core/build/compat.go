package build

import (
	"fmt"
	"regexp"
	"strconv"
)

// versionPrefix pulls the leading vMAJOR.MINOR[.PATCH] off a
// git-describe-shaped version string — "v0.9.0-fase7-132-g2b749c1-dirty"
// yields ("0", "9", "0"), the same string CI and a developer's own machine
// actually produce (see Version's own doc comment). A release build is
// expected to look the same shape with a clean tag ahead of "-dirty".
//
// Whatever follows the numbers has to start a suffix: "v1.2x" is not
// version 1.2 with some decoration, it is not a version.
var versionPrefix = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?(?:[-+].*)?$`)

// SemVer is a version string read as the numbers releases are ordered by.
type SemVer struct {
	Major, Minor, Patch int
}

// ParseVersion reads the release numbers out of a stamped version. It refuses
// what carries none — "dev", a bare commit hash, an empty string — rather than
// reading them as 0.0.0, because a build with no version is not older than
// every release, it is not comparable with any of them.
func ParseVersion(version string) (SemVer, bool) {
	m := versionPrefix.FindStringSubmatch(version)
	if m == nil {
		return SemVer{}, false
	}
	var v SemVer
	var err error
	if v.Major, err = strconv.Atoi(m[1]); err != nil {
		return SemVer{}, false
	}
	if v.Minor, err = strconv.Atoi(m[2]); err != nil {
		return SemVer{}, false
	}
	if m[3] != "" {
		if v.Patch, err = strconv.Atoi(m[3]); err != nil {
			return SemVer{}, false
		}
	}
	return v, true
}

// Compare orders two versions by their release numbers alone: -1 when v is
// older than o, 0 when they name the same release, 1 when v is newer.
//
// Suffixes do not take part, deliberately. Every suffix this tree produces
// names the same release seen from somewhere else — "-fase9" is a phase
// label, "-3-gabc1234" is three commits past the tag, "-dirty" is a locally
// packaged tree — so v0.15.2-fase9-dirty and v0.15.2-fase9 are one release,
// and offering the second to the first as an update would never stop.
func (v SemVer) Compare(o SemVer) int {
	for _, d := range [...]int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
	}
	return 0
}

// Compatible reports whether a client and a daemon at these versions can
// talk. Same minor is required — docs/08 - Entrega/Auto-Update.md's own
// design: "the CLI verifies this on the first call to the daemon and fails
// with a CTA, instead of a confusing protocol failure later."
//
// Either side failing to parse (most commonly "dev", the placeholder Version
// keeps in `go run` and in tests — see version.go) is treated as compatible
// rather than refused: a developer build has no real version to compare
// against, and blocking every local dev workflow on that would defeat the
// whole point of the placeholder.
func Compatible(client, daemon string) error {
	cMajor, cMinor, cOK := parseMajorMinor(client)
	dMajor, dMinor, dOK := parseMajorMinor(daemon)
	if !cOK || !dOK {
		return nil
	}
	if cMajor != dMajor || cMinor != dMinor {
		return errIncompatible(client, daemon)
	}
	return nil
}

func parseMajorMinor(version string) (major, minor int, ok bool) {
	v, ok := ParseVersion(version)
	return v.Major, v.Minor, ok
}

func errIncompatible(client, daemon string) error {
	return fmt.Errorf(
		"%s client %s and daemon %s are on different minor versions and cannot talk — update whichever is older",
		DisplayName, client, daemon,
	)
}
