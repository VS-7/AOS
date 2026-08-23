package update

import (
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

Check reads the release channel; it never downloads. Download fetches this
platform's assets for a release Check found, verifies the checksums file's
signature against the embedded key and every asset's own checksum, and
refuses to stage anything on the first failure. Apply swaps the staged
binaries in, waits for in-flight agent turns to finish first, restarts the
daemon, and rolls back automatically if the new version does not report
healthy.

## When to use
- A person checking for updates, or scripting an update in CI/an installer

## When NOT to use
- Not from an agent: there is no MCP tool for this group. An update decision
  is a human's, and Apply restarts the very process serving the agent's own
  turn.`,
	Hint: `Download without a prior Check that found a new release fails with
UPDATE_NOTHING_STAGED. Apply without a prior Download fails the same way.`,
}

// CheckInput selects a channel; empty means stable.
type CheckInput struct {
	Channel Channel `json:"channel,omitempty" jsonschema:"stable or beta. Defaults to stable."`

	command.Reasoning
}

// CheckOutput reports whether a newer release exists.
type CheckOutput struct {
	UpToDate bool     `json:"upToDate" jsonschema:"True when Current is already the newest release on Channel."`
	Current  string   `json:"current" jsonschema:"This installation's own version."`
	Channel  Channel  `json:"channel"`
	Release  *Release `json:"release,omitempty" jsonschema:"The newer release found, when UpToDate is false."`
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

// ApplyInput carries the release Download staged.
type ApplyInput struct {
	Staged Staged `json:"staged" jsonschema:"The staged release, as DownloadOutput.staged returned it." validate:"required"`

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
against the embedded public key before a single asset is fetched. Each
asset's own SHA-256 is then checked against the (now-trusted) checksums
file; a mismatch (UPDATE_CHECKSUM_MISMATCH) leaves nothing staged.`,
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Download and verify an update", OpenWorldHint: true},
		Handler:     svc.Download,
	})

	command.MustRegister(reg, command.Command[ApplyInput, ApplyOutput]{
		Group:   "update",
		Name:    "apply",
		Summary: "Install a staged, verified release and restart the daemon.",
		Doc: `Waits for in-flight agent turns to finish (bounded — UPDATE_ACTIVE_WORK_TIMEOUT
if they never do), swaps the staged binaries in, restarts the daemon, and
verifies it becomes healthy. On any failure after the swap, every binary is
rolled back and the daemon is restarted again on the previous version —
UPDATE_ROLLED_BACK reports that this happened, not that the whole operation
silently failed.`,
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Apply a staged update", OpenWorldHint: true},
		Handler:     svc.Apply,
	})

	command.MustRegister(reg, command.Command[StatusInput, Status]{
		Group:   "update",
		Name:    "status",
		Summary: "Report the current version and channel, without checking the network.",
		Doc:     "Read this installation's own version and configured channel.",
		Examples: []command.Example{
			{Description: "what am I running", Input: StatusInput{}},
		},
		Local:       true,
		Registry:    false,
		Annotations: command.Annotations{Title: "Update status", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Status,
	})
}
