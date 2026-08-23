// Package update keeps aos, aosd and aos-desktop on one verified version.
//
// docs/08 - Entrega/Auto-Update.md's own framing is the reason this exists:
// the reverse engineering of the original found three coexisting versions
// on one machine — CLI 0.1.314 via nvm, app 0.1.400, CLI 0.1.401 via
// Homebrew — because three artifacts updated over independent channels.
// That is not an installation mistake; it is what happens when nothing
// coordinates them. This package is that coordination: one release, one
// signature check, one apply, for all three binaries together.
package update

import "time"

// Channel selects which release stream Check reads from.
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
)

// Release is one published version, as the channel manifest describes it —
// see port.go's ReleaseSource for the manifest shape this is read from.
type Release struct {
	Version     string    `json:"version"`
	Channel     Channel   `json:"channel"`
	Notes       string    `json:"notes,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`

	// Assets is one entry per (binary, platform) pair actually published for
	// this release — not every platform necessarily has every binary yet.
	Assets []Asset `json:"assets"`

	// ChecksumsURL points at a text file listing "<sha256>  <asset filename>"
	// per line, one file covering every asset of this release. SignatureURL
	// points at the Ed25519 signature (relsig.Sign) over that same file's
	// bytes — see Verify's own reasoning in service.go for why the signature
	// covers the checksums file rather than each asset individually.
	ChecksumsURL string `json:"checksumsUrl"`
	SignatureURL string `json:"signatureUrl"`
}

// Asset is one downloadable artifact of a Release.
type Asset struct {
	// Binary is one of "aos", "aosd", "aos-desktop".
	Binary string `json:"binary"`
	// Platform is "GOOS/GOARCH", e.g. "darwin/arm64" — runtime.GOOS+"/"+runtime.GOARCH
	// on the machine that is meant to run it.
	Platform string `json:"platform"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	// Filename is the checksums file's own name for this asset — what its
	// line in ChecksumsURL's file is keyed by. Not necessarily the last path
	// segment of URL, so it is carried explicitly rather than derived.
	Filename string `json:"filename"`
}

// Staged is what Download leaves ready for Apply: every asset for this
// machine's own platform, checksum- and signature-verified, written to a
// scratch location Apply can rename into place without touching the network
// again.
type Staged struct {
	Version  string            `json:"version"`
	Dir      string            `json:"dir"`
	Binaries map[string]string `json:"binaries"` // binary name -> staged file path
}

// Status is what a caller asking "are we up to date" reads.
type Status struct {
	Current     string     `json:"current"`
	Channel     Channel    `json:"channel"`
	LatestKnown string     `json:"latestKnown,omitempty"`
	CheckedAt   *time.Time `json:"checkedAt,omitempty"`
}
