# Update

Check for, download and apply a signed update to aos, aosd and aos-desktop together.

Keep the three binaries on one version, verified before anything is
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
  turn.

## Commands

### `update_apply`

Install a staged, verified release and restart the daemon.

Waits for in-flight agent turns to finish (bounded — UPDATE_ACTIVE_WORK_TIMEOUT
if they never do), swaps the staged binaries in, restarts the daemon, and
verifies it becomes healthy. On any failure after the swap, every binary is
rolled back and the daemon is restarted again on the previous version —
UPDATE_ROLLED_BACK reports that this happened, not that the whole operation
silently failed.

### `update_check`

Query the release channel. Never downloads anything.

Read the newest release on Channel and compare it against this installation's own version.

- check the stable channel
- check beta

### `update_download`

Fetch and verify this platform's assets for a release. Nothing is installed yet.

Downloads the checksums file and its signature first, and refuses the
whole release (UPDATE_SIGNATURE_INVALID) if the signature does not verify
against the embedded public key before a single asset is fetched. Each
asset's own SHA-256 is then checked against the (now-trusted) checksums
file; a mismatch (UPDATE_CHECKSUM_MISMATCH) leaves nothing staged.

### `update_status`

Report the current version and channel, without checking the network.

Read this installation's own version and configured channel.

- what am I running

