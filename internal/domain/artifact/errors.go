package artifact

import "github.com/OWNER/aos/internal/core/apperr"

// errNotFound fires when no artifact has this ID.
func errNotFound(id string) error {
	return apperr.New("ARTIFACT_NOT_FOUND").
		Causer("artifact.Service.Get").
		Msgf("no artifact %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "list configured artifacts",
			Tool:  "artifacts_list",
		})
}

// errInvalidVisibility fires when a caller names a visibility outside the
// three declared members.
func errInvalidVisibility(raw string) error {
	return apperr.New("ARTIFACT_INVALID_VISIBILITY").
		Causer("artifact.Service").
		Msgf("%q is not an artifact visibility", raw).
		Issue("visibility", raw).
		Issue("allowed", "private, workspace, by_password").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of private, workspace, by_password"})
}

// errPasswordRequired fires when a visibility of by_password is set on an
// artifact that has never had SetPassword called on it — a link with no
// password to check against would either deny everyone or no one.
func errPasswordRequired(id string) error {
	return apperr.New("ARTIFACT_PASSWORD_REQUIRED").
		Causer("artifact.Service.Authorize").
		Msgf("artifact %q is by_password but has no password set", id).
		Issue("id", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "set a password before sharing a by_password link",
			Tool:  "artifacts_set_password",
		})
}

// errUnauthorized fires when Authorize refuses a request against an
// artifact's visibility.
func errUnauthorized(id string) error {
	return apperr.New("ARTIFACT_UNAUTHORIZED").
		Causer("artifact.Service.Authorize").
		Msgf("not authorized to read artifact %q", id).
		Issue("id", id).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "authenticate, join the workspace, or supply the correct password"})
}

// errScaffoldFailed wraps a failure writing the entrypoint file Create
// scaffolds when none is given.
func errScaffoldFailed(id string, cause error) error {
	return apperr.New("ARTIFACT_SCAFFOLD_FAILED").
		Causer("artifact.Service.Create").
		Msgf("could not scaffold artifact %q: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errHashFailed wraps a failure of the password KDF itself, distinct from a
// wrong password, which is not an error at all — see Authorize.
func errHashFailed(id string, cause error) error {
	return apperr.New("ARTIFACT_HASH_FAILED").
		Causer("artifact.Service.SetPassword").
		Msgf("could not hash the password for artifact %q: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errReadFailed wraps a repository failure that is not "not found" — the
// operation name is dynamic because it fires from several call sites.
func errReadFailed(op string, cause error) error {
	return apperr.New("ARTIFACT_READ_FAILED").
		Causer("artifact.Service."+op).
		Msgf("could not read artifacts: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errWriteFailed wraps a repository failure while writing.
func errWriteFailed(op string, cause error) error {
	return apperr.New("ARTIFACT_WRITE_FAILED").
		Causer("artifact.Service."+op).
		Msgf("could not save the artifact: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
