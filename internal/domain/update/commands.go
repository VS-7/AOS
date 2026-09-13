package update

import (
	"time"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a human reads on the CLI. No MCP/agent surface — the
// same boundary internal/domain/tunnel and internal/domain/gateway
// document: an agent does not get to decide when its own daemon restarts
// out from under it, and docs/08 - Entrega/Auto-Update.md is explicit that
// installing requires confirmation, not silent automation.
var GroupDoc = command.GroupDoc{
	Name:    "update",
	Tool:    "Update",
	Summary: "Check for, download and apply a signed update to aos, aosd and aos-desktop together.",
	Doc: `Keep the three binaries on one version, verified before anything is
installed.

Check reads the release channel; it never downloads. It answers with a
state: up-to-date, available, not-configured (this build has no release
feed) or developer-build (a binary with no release version to compare). Only
a strictly newer release is offered.

Download fetches this platform's assets for a release Check found, verifies
the checksums file's signature against the embedded key and every asset's
own checksum, and refuses to stage anything on the first failure. Apply
takes the staged version, proves the staged files against the signature
again, swaps them in, waits for in-flight agent turns to finish first,
restarts the daemon, and rolls back automatically if the new version does
not report healthy. Download and Apply are for administrators (super
accounts) only, and never for an agent or an MCP client
(UPDATE_NOT_FOR_AGENTS).

The daemon cannot restart itself, so Apply refuses to run inside it
(UPDATE_RESTART_UNAVAILABLE) before touching anything; Check's install
field names the command that installs from a terminal instead, and says
when the window has to be reopened afterwards. That command restarts only
the daemon its supervisor started — the desktop application or
` + "`aos gateway`" + `. Beside a daemon started by hand or by a service manager it
refuses (UPDATE_DAEMON_NOT_SUPERVISED), and install says so
(unsupervised): that daemon is stopped where it was started, and the
command then starts the new version itself. An install is done only when
the restarted daemon answers as the staged version. Three kinds of installation
are reinstalled whole rather than updated one binary at a time
(UPDATE_REINSTALL_REQUIRED, with the reason): a signed macOS application
bundle; a directory this account cannot write, such as an AppImage or
Program Files; and the server flavour of the daemon, which carries the web
interface that the daemon a release publishes does not.

## When to use
- A person checking for updates, or scripting an update in CI/an installer

## When NOT to use
- Not from an agent. MCP clients see this group like every other, and Check
  and Status answer them, but Download and Apply refuse a call made through
  MCP or by an agent: an update decision is a human's, and Apply restarts the
  very process serving the agent's own turn.`,
	Hint: `Apply without a prior Download of that same version fails with
UPDATE_NOTHING_STAGED.`,
}

// CheckInput selects a channel; empty means stable.
type CheckInput struct {
	Channel Channel `json:"channel,omitempty" jsonschema:"stable or beta. Defaults to stable."`

	command.Reasoning
}

// CheckOutput reports what the channel has, measured against this
// installation.
type CheckOutput struct {
	State    CheckState `json:"state" jsonschema:"up-to-date, available, not-configured (no release feed in this build) or developer-build (no release version to compare)."`
	UpToDate bool       `json:"upToDate" jsonschema:"True only when State is up-to-date: the channel has nothing newer than Current."`
	Current  string     `json:"current" jsonschema:"This installation's own version."`
	Channel  Channel    `json:"channel"`
	Release  *Release   `json:"release,omitempty" jsonschema:"The newest release on Channel, when State is available or developer-build."`
	// Install is set when State is available.
	Install   *Install  `json:"install,omitempty" jsonschema:"How the offered release can be installed here: here, terminal (run Command) or reinstall."`
	CheckedAt time.Time `json:"checkedAt"`
}

// DownloadInput carries the release Check found.
type DownloadInput struct {
	Release *Release `json:"release" jsonschema:"The release to download, as CheckOutput.release returned it." validate:"required"`

	command.Reasoning
}

// DownloadOutput is what got staged.
type DownloadOutput struct {
	Staged Staged `json:"staged"`
}

// ApplyInput names the release Download staged. Only the version: what
// was staged, and where, is the daemon's own record, never the caller's.
type ApplyInput struct {
	Version string `json:"version" jsonschema:"The version to install, as DownloadOutput.staged.version returned it." validate:"required"`

	command.Reasoning
}

// ApplyOutput reports what happened: the new version running, or a
// rollback to the previous one (with the error explaining why, via the
// command's own failure path — RolledBack true only appears on an error
// return, kept here for a caller inspecting a caught error's context).
type ApplyOutput struct {
	Version    string `json:"version,omitempty"`
	RolledBack bool   `json:"rolledBack"`
}

// StatusInput takes nothing.
type StatusInput struct {
	command.Reasoning
}

// Register declares the group on the registry. Local because this operates
// the machine's own installed binaries, the same reasoning
// internal/domain/gateway and internal/domain/tunnel already document.
// Registry stays false: never published to an agent.
func Register(reg *command.Registry, svc Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[CheckInput, CheckOutput]{
		Group:   "update",
		Name:    "check",
		Summary: "Query the release channel. Never downloads anything.",
		Doc:     "Read the newest release on Channel and compare it against this installation's own version.",
		Examples: []command.Example{
			{Description: "check the stable channel", Input: CheckInput{}},
			{Description: "check beta", Input: CheckInput{Channel: ChannelBeta}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Check for an update", ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		Handler:     svc.Check,
	})

	command.MustRegister(reg, command.Command[DownloadInput, DownloadOutput]{
		Group:   "update",
		Name:    "download",
		Summary: "Fetch and verify this platform's assets for a release. Nothing is installed yet.",
		Doc: `Downloads the checksums file and its signature first, and refuses the
whole release (UPDATE_SIGNATURE_INVALID) if the signature does not verify
against the embedded public key before a single asset is fetched — or
UPDATE_SIGNATURE_MISSING when the release was published without one. Each
asset's binary, version and platform must match its file name in the signed
checksums (UPDATE_ASSET_REJECTED), and its own SHA-256 must match that file;
a mismatch (UPDATE_CHECKSUM_MISMATCH) leaves nothing staged. Only binaries
already installed on this machine are downloaded, and only for a release
newer than this one (UPDATE_NOT_NEWER). An installation that is reinstalled
whole downloads nothing (UPDATE_REINSTALL_REQUIRED).`,
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Download and verify an update", OpenWorldHint: true},
		Handler:     svc.Download,
	})

	command.MustRegister(reg, command.Command[ApplyInput, ApplyOutput]{
		Group:   "update",
		Name:    "apply",
		Summary: "Install a staged, verified release and restart the daemon.",
		Doc: `Refuses before touching anything when another download or install is
running on this installation, in this daemon or in a terminal
(UPDATE_IN_PROGRESS), when the release is not staged under that version
(UPDATE_NOTHING_STAGED), when this process cannot restart the
daemon (UPDATE_RESTART_UNAVAILABLE — the daemon does not restart itself),
when the daemon answering was not started by this installation's supervisor
(UPDATE_DAEMON_NOT_SUPERVISED — stop it where it was started, and the
install starts the new version itself) or the supervisor's daemon is not
answering (UPDATE_DAEMON_NOT_ANSWERING), or when the installation is
reinstalled whole — a bundle, a directory this account cannot write, a
server daemon (UPDATE_REINSTALL_REQUIRED).
Then proves every staged file against the signed checksums again
(UPDATE_STAGED_TAMPERED discards what no longer matches), waits for
in-flight agent turns to finish (bounded — UPDATE_ACTIVE_WORK_TIMEOUT if
they never do), looks at the daemon again, swaps the staged binaries in,
restarts the daemon, and waits for the process the supervisor started to
answer as the staged version. On any failure after the swap — including a
daemon that came back up as another version — every binary is rolled back
and the daemon is restarted again on the previous version; the previous
binaries are removed only once the new version has answered.
UPDATE_ROLLED_BACK reports that this happened, not that the whole operation
silently failed.`,
		Examples: []command.Example{
			{Description: "install the release a download staged", Input: ApplyInput{Version: "v0.16.0"}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Apply a staged update", OpenWorldHint: true},
		Handler:     svc.Apply,
	})

	command.MustRegister(reg, command.Command[StatusInput, Status]{
		Group:   "update",
		Name:    "status",
		Summary: "Report the current version, the last check and what is staged, without checking the network.",
		Doc:     "Read this installation's own version, whether it has a release feed, what the last check found, what is staged, how a release would be installed here, and whether a download or install is running right now (busy) — which is how a caller follows a download whose answer it never received.",
		Examples: []command.Example{
			{Description: "what am I running", Input: StatusInput{}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Update status", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Status,
	})
}
