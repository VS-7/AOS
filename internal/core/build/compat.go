package build

import (
	"fmt"
	"regexp"
	"strconv"
)

// versionPrefix pulls the leading vMAJOR.MINOR off a git-describe-shaped
// version string — "v0.9.0-fase7-132-g2b749c1-dirty" yields ("0", "9"), the
// same string CI and a developer's own machine actually produce (see
// Version's own doc comment). A release build is expected to look the same
// shape with a clean tag ahead of "-dirty".
var versionPrefix = regexp.MustCompile(`^v?(\d+)\.(\d+)`)

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
	m := versionPrefix.FindStringSubmatch(version)
	if m == nil {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func errIncompatible(client, daemon string) error {
	return fmt.Errorf(
		"%s client %s and daemon %s are on different minor versions and cannot talk — update whichever is older",
		DisplayName, client, daemon,
	)
}
