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

import (
	"strings"
	"time"
)

// Channel selects which release stream Check reads from.
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
)

// binaries is every executable an update may replace, and the only names
// Download stages or Apply swaps. A name is a file name on disk, so a list is
// the whole defence: a release feed, a request body or a record on disk that
// names "../aosd" names nothing this package will touch.
var binaries = []string{"aos", "aos-desktop", "aosd"}

// IsBinary reports whether name is one of the executables an update manages.
// Exported for the adapters, which refuse any other name themselves rather
// than trusting that the domain already did.
func IsBinary(name string) bool {
	for _, b := range binaries {
		if b == name {
			return true
		}
	}
	return false
}

// Binaries lists the executables an update manages.
func Binaries() []string { return append([]string(nil), binaries...) }

// Release is one published version, as the channel manifest describes it —
// see port.go's ReleaseSource for the manifest shape this is read from.
//
// Only ChecksumsURL's file is signed, so nothing here is trusted on its own
// word: an asset's binary, platform and version are read back out of its
// file name in the signed checksums before anything is staged under them.
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

	// PageURL is where a person reads about this release and downloads it by
	// hand — what an installation that cannot update in place is sent to.
	PageURL string `json:"pageUrl,omitempty"`
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
	// line in ChecksumsURL's file is keyed by, and, being signed, what the
	// binary, version and platform above are checked against.
	Filename string `json:"filename"`
}

// assetName is what a release asset's file name says about it:
// "<binary>_<version>_<goos>_<goarch>[.exe]", the name release.yml gives every
// raw binary. None of the four parts can contain an underscore — binary names
// are the list above, versions are git tags, and Go's GOOS/GOARCH have none.
type assetName struct {
	binary, version, platform string
}

func parseAssetName(filename string) (assetName, bool) {
	parts := strings.Split(strings.TrimSuffix(filename, ".exe"), "_")
	if len(parts) != 4 {
		return assetName{}, false
	}
	for _, p := range parts {
		if p == "" {
			return assetName{}, false
		}
	}
	return assetName{binary: parts[0], version: parts[1], platform: parts[2] + "/" + parts[3]}, true
}

// CheckState is what a check found, as one word a caller can branch on.
//
// UpToDate alone could not carry it. "No feed is configured" and "a
// development build cannot be compared with a release" both came out as
// UpToDate:true, and the window answered them with a green "You are on the
// newest release." while a newer release was published.
type CheckState string

const (
	// StateUpToDate: the channel's newest release is not newer than this one.
	StateUpToDate CheckState = "up-to-date"
	// StateAvailable: Release is newer than this installation.
	StateAvailable CheckState = "available"
	// StateNotConfigured: this build has no release feed to ask.
	StateNotConfigured CheckState = "not-configured"
	// StateDeveloperBuild: this binary carries no release version ("dev", a
	// bare commit), so no release is newer or older than it. Release is
	// still attached, for information.
	StateDeveloperBuild CheckState = "developer-build"
)

// InstallMethod is how a newer release can reach this installation.
type InstallMethod string

const (
	// InstallHere: Download and Apply work from the process answering.
	InstallHere InstallMethod = "here"
	// InstallFromTerminal: the binaries can be replaced in place, but the
	// process answering is the daemon itself, which cannot restart itself
	// and bring the new version up. A separate process can: Command.
	InstallFromTerminal InstallMethod = "terminal"
	// InstallReinstall: the binaries cannot be replaced one at a time — see
	// ReinstallReason for why; the new version is installed whole, with the
	// installer or from the release page.
	InstallReinstall InstallMethod = "reinstall"
)

// ReinstallReason is why an installation is reinstalled whole rather than
// updated one binary at a time. Each one is told a different remedy.
type ReinstallReason string

const (
	// ReinstallBundle: a signed macOS application bundle, whose signature
	// seals every file in it.
	ReinstallBundle ReinstallReason = "bundle"
	// ReinstallReadOnly: the directory the binaries live in cannot be
	// changed by the account running this — an AppImage's read-only mount,
	// an install for every account under Program Files or /usr.
	ReinstallReadOnly ReinstallReason = "read-only"
	// ReinstallServer: this daemon is the server flavour, with the web
	// interface compiled in (build.FlavourServer), and a release publishes
	// only the daemon without it.
	ReinstallServer ReinstallReason = "server"
)

// Install says how to install a release on this machine.
type Install struct {
	Method InstallMethod `json:"method"`
	// Reason is why, when Method is InstallReinstall.
	Reason ReinstallReason `json:"reason,omitempty"`
	// Command is the exact command that installs the staged or offered
	// release, when Method is InstallFromTerminal.
	Command string `json:"command,omitempty"`
	// Reopen is true when the window is one of the binaries an install
	// replaces here. The window already open goes on running the previous
	// release until it is quit and opened again.
	Reopen bool `json:"reopen,omitempty"`
	// Unsupervised is true when the daemon answering was not started by this
	// installation's supervisor — the desktop application or `aos gateway` —
	// but by hand (`aosd serve`) or by a service manager. Command cannot
	// restart a daemon it did not start: that one is stopped where it was
	// started first, and Command then starts the new version itself.
	Unsupervised bool `json:"unsupervised,omitempty"`
	// Unidentified is true when a daemon answers, the supervisor's record
	// names a live process, and the two cannot be matched — see
	// Daemon.unidentified. Restarting through the supervisor is what helps,
	// and it is the opposite of the advice Unsupervised carries.
	Unidentified bool `json:"unidentified,omitempty"`
}

// Daemon is which daemon answers for this installation, as the process asking
// sees it — DaemonSupervisor.Observe's answer.
//
// It exists because "something answers on the port" was the whole question,
// and it is the wrong one twice over. An install that restarted nothing it
// had started found the previous daemon still answering and reported the new
// version running; and a restart onto the binaries it had just replaced was
// taken for proof whichever binary the daemon had actually been started from.
type Daemon struct {
	// Self is true when the process asking is the daemon itself.
	Self bool
	// Address is where this installation's daemon answers, as host:port.
	Address string
	// RecordedPID is the process the supervisor's record names, while that
	// process is alive; 0 when there is no record or its process has gone.
	RecordedPID int
	// Answering is true when a daemon answered its health check at Address.
	Answering bool
	// PID and Version are what the daemon answering says about itself. PID
	// is 0 when it does not say, as a release from before it did.
	PID     int
	Version string
}

// supervised: the daemon answering is the very process the supervisor
// started, so a restart stops it and brings the binaries in place up instead.
func (d Daemon) supervised() bool {
	return d.Answering && d.RecordedPID > 0 && d.PID == d.RecordedPID
}

// unidentified: a daemon answers, the supervisor's record names a process
// that is alive, and the two cannot be matched.
//
// It is neither supervised nor started by hand, and calling it either is
// wrong in a way somebody then acts on. There are two ways to get here and
// both are the supervisor's own daemon: a release older than the health
// answer's pid, which says nothing about which process it is (install.sh
// replaced the binaries without restarting anything); and a daemon started
// through a wrapper script that does not exec, so the record holds the
// wrapper's pid and the daemon answers with its own.
//
// Neither is mended by stopping a terminal nobody is running — restarting
// through the supervisor is what tells them apart or brings a release that
// names its process up.
func (d Daemon) unidentified() bool {
	return d.Answering && d.RecordedPID > 0 && d.PID != d.RecordedPID
}

// unsupervised: a daemon answers and this installation's supervisor has no
// record of a live process at all, so that daemon was started some other way
// — `aosd serve` in a terminal, a service manager — and a restart from here
// cannot stop it.
func (d Daemon) unsupervised() bool { return d.Answering && d.RecordedPID == 0 }

// idle: no daemon runs at all — nothing answers, and the supervisor's record
// names no live process — so an install starts the new version rather than
// restarting one.
func (d Daemon) idle() bool { return !d.Answering && d.RecordedPID == 0 }

// restartable reports whether an install from the process asking can bring
// the new version up, and bring the previous one back if it does not.
func (d Daemon) restartable() bool { return !d.Self && (d.supervised() || d.idle()) }

// Staged is what Download left ready for Apply, as a caller sees it.
type Staged struct {
	Version  string            `json:"version"`
	Dir      string            `json:"dir"`
	Binaries map[string]string `json:"binaries"` // binary name -> staged file path
}

// Status is what a caller asking "are we up to date" reads.
type Status struct {
	Current string  `json:"current"`
	Channel Channel `json:"channel"`
	// Configured is false when this build has no release feed at all.
	Configured bool `json:"configured"`
	// Install is how a newer release would reach this installation.
	Install     Install    `json:"install"`
	LatestKnown string     `json:"latestKnown,omitempty"`
	CheckedAt   *time.Time `json:"checkedAt,omitempty"`
	// LastState is what the last check found.
	LastState CheckState `json:"lastState,omitempty"`
	// Staged is a verified release waiting for Apply, when there is one.
	Staged *Staged `json:"staged,omitempty"`
	// Busy is true while a Download or an Apply is running on this
	// installation — in this daemon or in a terminal. A caller that lost the
	// answer to its own download (a bridge that gave up waiting, a retry
	// told UPDATE_IN_PROGRESS) follows it here until it ends.
	Busy bool `json:"busy,omitempty"`
}

// Record is what the service keeps between calls, through Store.
type Record struct {
	LastCheck *LastCheck     `json:"lastCheck,omitempty"`
	Staged    *StagedRelease `json:"staged,omitempty"`
	// Installing is the version an Apply is swapping in, from before its
	// first swap until it ends. Still set afterwards, it is an Apply that
	// was killed partway, or one whose rollback failed: the backups it kept
	// are then the previous binaries, and nothing tidies them away.
	Installing string `json:"installing,omitempty"`
}

// LastCheck is the outcome of the most recent successful Check.
type LastCheck struct {
	At      time.Time  `json:"at"`
	Channel Channel    `json:"channel"`
	State   CheckState `json:"state"`
	Latest  string     `json:"latest,omitempty"`
}

// StagedRelease is Download's evidence, kept on this side of the API.
//
// Apply used to take the staged file paths from its request body — whatever a
// caller sent — and rename them over the live binaries with no signature in
// sight; "../" in a binary name put a file outside the install directory.
// Apply now takes a version, finds this record, and proves the staged files
// against the signed checksums again before it swaps anything.
type StagedRelease struct {
	Version string `json:"version"`
	// Checksums and Signature are the release's signed checksums file and
	// its signature, verbatim.
	Checksums string `json:"checksums"`
	Signature string `json:"signature"`
	// Files maps each staged binary to its file name in Checksums.
	Files map[string]string `json:"files"`
	// Paths maps each staged binary to where Stager put it — for display;
	// Apply never reads a path from here.
	Paths map[string]string `json:"paths"`
	Dir   string            `json:"dir"`
}

func (r *StagedRelease) view() *Staged {
	if r == nil {
		return nil
	}
	return &Staged{Version: r.Version, Dir: r.Dir, Binaries: r.Paths}
}
