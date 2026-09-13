package update

import (
	"errors"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// errSourceUnreachable is the one failure that is about the network: the
// channel did not answer. causer is the operation that asked, because Check
// and Download both reach the channel and a Download failure attributed to
// Check sends somebody looking in the wrong place.
func errSourceUnreachable(causer string, cause error) error {
	return apperr.New("UPDATE_SOURCE_UNREACHABLE").
		Causer(causer).
		Msgf("could not reach the release channel: %v", cause).
		Status(apperr.StatusServiceUnavailable).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check network access, then retry"})
}

// errSourceFailed: the channel answered, with something other than what was
// asked for — a server error, a body that is not a manifest.
func errSourceFailed(causer string, cause error) error {
	return apperr.New("UPDATE_SOURCE_FAILED").
		Causer(causer).
		Msgf("the release channel answered with an error: %v", cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry later; if it persists, the release channel is misconfigured"})
}

// errChannelEmpty fires when a feed is configured and has no manifest for the
// channel. It used to be read as "no release", which the window showed as
// "You are on the newest release." — the one answer that is certainly wrong
// for a feed pointed at the wrong address.
//
// What to do about it depends on whose feed it is. One set on this machine
// can be pointed somewhere else. The feed a build carries is the latest
// release's own, and when it is empty that release was published without
// one: telling the person to check an environment variable they never set
// sends them nowhere.
func errChannelEmpty(channel Channel, customFeed bool, cause error) error {
	action := "the latest release was published without an update feed; install it with the installer, or from the release page"
	if customFeed {
		action = "check that " + envUpdateBaseURL + " points at a feed that publishes " + string(channel) + ".json"
	}
	return apperr.New("UPDATE_CHANNEL_EMPTY").
		Causer("update.Service.Check").
		Msgf("the release feed publishes nothing on the %s channel: %v", channel, cause).
		Issue("channel", string(channel)).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: action})
}

// envUpdateBaseURL names the setting in the words a person configures it
// with. Spelled from build.EnvPrefix rather than imported from core/env: the
// domain reads no environment, it only names it.
const envUpdateBaseURL = build.EnvPrefix + "_UPDATE_BASE_URL"

func errReleaseVersionInvalid(version string) error {
	return apperr.New("UPDATE_RELEASE_VERSION_INVALID").
		Causer("update.Service.Check").
		Msgf("the release channel published %q, which is not a version", version).
		Issue("version", version).
		Status(apperr.StatusBadGateway).
		CTA(apperr.CallToAction{Label: "the release manifest is malformed; nothing can be installed from it until it is republished"})
}

func errChecksumsMissing(cause error) error {
	return apperr.New("UPDATE_CHECKSUMS_MISSING").
		Causer("update.Service.Download").
		Msgf("the release has no checksums file: %v", cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "this release cannot be verified, so it cannot be installed automatically; install it with the installer"})
}

// errSignatureMissing is not a network problem, and it is not transient: a
// release published without a signature stays that way.
func errSignatureMissing(cause error) error {
	return apperr.New("UPDATE_SIGNATURE_MISSING").
		Causer("update.Service.Download").
		Msgf("the release was published without a signature: %v", cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "this release cannot be verified, so it cannot be installed automatically; install it with the installer"})
}

func errAssetUnavailable(binary string, cause error) error {
	return apperr.New("UPDATE_ASSET_UNAVAILABLE").
		Causer("update.Service.Download").
		Msgf("the release lists %s, but it is not there: %v", binary, cause).
		Issue("binary", binary).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry later — a release can still be uploading its assets"})
}

// errAssetRejected fires when the unsigned manifest describes an asset
// differently from the signed checksums: a binary name this package does not
// manage, a platform or version its signed file name does not carry. The
// manifest is the part of a release anybody between here and the feed could
// rewrite, so a disagreement is refused whole rather than resolved.
func errAssetRejected(filename, reason string) error {
	return apperr.New("UPDATE_ASSET_REJECTED").
		Causer("update.Service.Download").
		Msgf("the release manifest's entry for %q does not match its signed checksums: %s", filename, reason).
		Issue("filename", filename).
		Status(apperr.StatusBadGateway).
		CTA(apperr.CallToAction{Label: "do not retry blindly — the release manifest disagrees with what was signed"})
}

func errNoAssetForPlatform(binary, platform string) error {
	return apperr.New("UPDATE_NO_ASSET_FOR_PLATFORM").
		Causer("update.Service.Download").
		Msgf("this release has no %s asset for %s", binary, platform).
		Issue("binary", binary).
		Issue("platform", platform).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "this platform is not yet published for this release; wait for the next one"})
}

// errChecksumMismatch fires when a downloaded asset's own SHA-256 does not
// match the (already signature-verified) checksums file. Nothing is staged
// when this fires — see Download's own doc comment.
func errChecksumMismatch(binary string) error {
	return apperr.New("UPDATE_CHECKSUM_MISMATCH").
		Causer("update.Service.Download").
		Msgf("the downloaded %s does not match its published checksum — refusing to install it", binary).
		Issue("binary", binary).
		Status(apperr.StatusBadGateway).
		CTA(apperr.CallToAction{Label: "retry the download; if this persists, the release channel itself may be compromised"})
}

// errSignatureInvalid fires when the checksums file's own signature does
// not verify against the embedded public key. This is the one refusal in
// this package with no retry advice: a signature that does not verify is
// not a transient failure.
func errSignatureInvalid(cause error) error {
	return apperr.New("UPDATE_SIGNATURE_INVALID").
		Causer("update.Service.Download").
		Msgf("the release checksums file's signature does not verify: %v", cause).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "do not retry blindly — this means the release channel served something it did not sign"})
}

func errStageFailed(cause error) error {
	return apperr.New("UPDATE_STAGE_FAILED").
		Causer("update.Service.Download").
		Msgf("could not keep the verified release for installing: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "nothing was installed; check the free space and permissions of the state directory, then retry"})
}

// errNotNewer refuses to download or install a release that is not newer
// than this installation. The signed file names carry the version, so this
// is what stops an old, genuinely signed release from being replayed as an
// update.
func errNotNewer(causer, version, current string) error {
	return apperr.New("UPDATE_NOT_NEWER").
		Causer(causer).
		Msgf("%s is not newer than this installation (%s)", version, current).
		Issue("version", version).
		Issue("current", current).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "check for updates again"})
}

func errDeveloperBuild(causer, current string) error {
	return apperr.New("UPDATE_DEVELOPER_BUILD").
		Causer(causer).
		Msgf("this is a development build (%s), which has no release version to update from", current).
		Issue("current", current).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "rebuild it from source, or install a release with the installer"})
}

// errReinstallRequired refuses to download or install into an installation
// that takes a release whole, and says why, because the remedy differs: the
// installer or the release page for a bundle or a directory this account
// cannot change, and the server installer for a daemon that carries the web
// interface.
func errReinstallRequired(causer string, why ReinstallReason) error {
	message := "this installation is a signed application bundle, which cannot be updated one binary at a time"
	action := "install the new version whole, with the installer or from the release page"
	switch why {
	case ReinstallReadOnly:
		message = "the directory this installation's binaries live in cannot be changed by this account, so they cannot be replaced one at a time"
	case ReinstallServer:
		message = "this daemon carries the web interface, and a release publishes only the daemon without it — installing that would leave this server answering the API alone"
		action = "run install.sh again with AOS_SERVER=1, which installs the server build of the new release"
	}
	return apperr.New("UPDATE_REINSTALL_REQUIRED").
		Causer(causer).
		Msgf("%s", message).
		Issue("reason", string(why)).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: action})
}

func errForbidden(causer string) error {
	return apperr.New("UPDATE_FORBIDDEN").
		Causer(causer).
		Msgf("only an administrator of this installation may download or install updates").
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "ask a super account on this installation to install the update"})
}

func errAuthorizationFailed(causer string, cause error) error {
	return apperr.New("UPDATE_AUTHORIZATION_FAILED").
		Causer(causer).
		Msgf("could not tell whether this account may install updates: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; nothing was downloaded or installed"})
}

func errNothingStaged(version string) error {
	return apperr.New("UPDATE_NOTHING_STAGED").
		Causer("update.Service.Apply").
		Msgf("no verified release %s is staged to apply", version).
		Issue("version", version).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "run update.check then update.download first"})
}

func errStateUnreadable(causer string, cause error) error {
	return apperr.New("UPDATE_STATE_UNREADABLE").
		Causer(causer).
		Msgf("could not read what is staged: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "download the release again"})
}

// errStagedTampered fires when a staged file, or the record of what was
// staged, no longer proves itself against the signature. The staged release
// is discarded before this is returned.
func errStagedTampered(binary string, cause error) error {
	return apperr.New("UPDATE_STAGED_TAMPERED").
		Causer("update.Service.Apply").
		Msgf("the staged %s no longer matches its signed checksum, so it was discarded: %v", binary, cause).
		Issue("binary", binary).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "download the release again; if this repeats, something is changing files in the state directory"})
}

// errStagedIncomplete fires when the staged release no longer covers every
// binary installed here — one was installed after the download. Installing
// the rest would leave that one on the previous version. The staged release
// is discarded before this is returned.
func errStagedIncomplete(binary string) error {
	return apperr.New("UPDATE_STAGED_INCOMPLETE").
		Causer("update.Service.Apply").
		Msgf("%s is installed here, and the staged release does not include it, so nothing was installed", binary).
		Issue("binary", binary).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "download the release again, so every installed binary is updated together"})
}

// errInstallationUnreadable: which binaries are installed here could not be
// told, and an update that cannot tell cannot keep them on one version.
func errInstallationUnreadable(causer string, cause error) error {
	return apperr.New("UPDATE_INSTALLATION_UNREADABLE").
		Causer(causer).
		Msgf("could not tell which binaries are installed here: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "nothing was downloaded or installed; check the permissions of the installation directory, then retry"})
}

// errRestartUnavailable is Apply refusing up front, with nothing touched,
// because the process it runs in cannot bring the new version up. It used to
// swap the binaries first and find out after: the daemon refused to restart
// itself, the rollback's own restart was refused the same way, and the
// answer was "rollback ALSO failed — the daemon may be down" from a daemon
// that was answering the call, with the staged file consumed.
//
// Where the installation carries the window, the window already open stays
// on the previous release after the terminal install, so that is said too.
func errRestartUnavailable(install Install) error {
	actions := []apperr.CallToAction{{
		Label:   "install it from a terminal, where a separate process can restart the daemon and roll back if the new version does not come up",
		Command: install.Command,
	}}
	if install.Reopen {
		actions = append(actions, apperr.CallToAction{Label: "when it finishes, quit and reopen AOS: the open window keeps running the previous release until then"})
	}
	return apperr.New("UPDATE_RESTART_UNAVAILABLE").
		Causer("update.Service.Apply").
		Msgf("the daemon cannot restart itself onto a new version, so it did not replace anything").
		Status(apperr.StatusConflict).
		CTA(actions...)
}

func errActiveWorkTimeout(grace string) error {
	return apperr.New("UPDATE_ACTIVE_WORK_TIMEOUT").
		Causer("update.Service.Apply").
		Msgf("agent turns were still in flight after the %s grace period — refusing to restart under them", grace).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "retry once the in-flight turns finish, or stop them first"})
}

func errApplyFailed(cause error) error {
	return apperr.New("UPDATE_APPLY_FAILED").
		Causer("update.Service.Apply").
		Msgf("could not put the new binaries in place: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the previous binaries are back in place; retry, or investigate before retrying"})
}

// errRolledBack fires when the daemon did not report healthy after
// restarting on the new binaries. The previous version is already back in
// place and answering by the time this is returned — see service.go's Apply.
func errRolledBack(cause error) error {
	return apperr.New("UPDATE_ROLLED_BACK").
		Causer("update.Service.Apply").
		Msgf("the new version did not become healthy after restart; rolled back to the previous one: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the daemon is running the previous version again; this is a defect in the release, not in your machine"})
}

// errRestartAfterRollback: the previous binaries are back on disk, and the
// daemon did not come back up on them. The files are fine; the process is
// the problem, and starting it is the whole remedy.
func errRestartAfterRollback(cause, restart error) error {
	return apperr.New("UPDATE_RESTART_AFTER_ROLLBACK_FAILED").
		Causer("update.Service.Apply").
		Msgf("the new version failed (%v); the previous binaries are back in place, but the daemon did not come back up on them: %v", cause, restart).
		Status(apperr.StatusInternalServerError).
		Wrap(errors.Join(cause, restart)).
		CTA(apperr.CallToAction{Label: "start the daemon again", Command: build.Name + " gateway start"})
}

// errRollbackFailed is the one outcome where the binaries on disk may be a
// mix of two versions: restoring at least one of them failed.
func errRollbackFailed(cause, rollback error) error {
	return apperr.New("UPDATE_ROLLBACK_FAILED").
		Causer("update.Service.Apply").
		Msgf("the new version failed (%v), and restoring the previous binaries failed too: %v", cause, rollback).
		Status(apperr.StatusInternalServerError).
		Wrap(errors.Join(cause, rollback)).
		CTA(apperr.CallToAction{Label: "reinstall with the installer — the binaries on disk may be from two different versions"})
}
