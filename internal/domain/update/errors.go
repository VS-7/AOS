package update

import "github.com/OWNER/aos/internal/core/apperr"

func errReleaseSourceUnavailable(cause error) error {
	return apperr.New("UPDATE_SOURCE_UNAVAILABLE").
		Causer("update.Service.Check").
		Msgf("could not reach the release channel: %v", cause).
		Status(apperr.StatusServiceUnavailable).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check network access, then retry"})
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

func errNothingStaged() error {
	return apperr.New("UPDATE_NOTHING_STAGED").
		Causer("update.Service.Apply").
		Msgf("no verified release is staged to apply").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "run update.check then update.download first"})
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
		CTA(apperr.CallToAction{Label: "the previous binaries are untouched; retry, or investigate before retrying"})
}

// errRolledBack fires when the daemon did not report healthy after
// restarting on the new binaries. The previous version is already back in
// place by the time this is returned — see service.go's Apply.
func errRolledBack(cause error) error {
	return apperr.New("UPDATE_ROLLED_BACK").
		Causer("update.Service.Apply").
		Msgf("the new version did not become healthy after restart; rolled back to the previous one: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the daemon is running the previous version again; this is a defect in the release, not in your machine"})
}

func errRollbackFailed(cause error) error {
	return apperr.New("UPDATE_ROLLBACK_FAILED").
		Causer("update.Service.Apply").
		Msgf("the new version failed health and rollback ALSO failed: %v", cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the daemon may be down. Restart it manually with the binaries in dist/, or from the previous release"})
}
