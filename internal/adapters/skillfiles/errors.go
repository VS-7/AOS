package skillfiles

import "github.com/OWNER/aos/internal/core/apperr"

func errResolveFailed(skillID string, cause error) error {
	return apperr.New("SKILLFILES_RESOLVE_FAILED").
		Causer("skillfiles.Files").
		Msgf("could not resolve the directory of skill %q: %v", skillID, cause).
		Issue("skill", skillID).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace root is readable"})
}

// errPathOutside fires when a file's own path — Write's file.Path or
// Remove's path — would resolve outside the skill's directory. It is the
// same boundary skillfetch.Local reads a package through, applied here on
// the way back out: a package that could write outside its own directory
// could overwrite anything else the workspace holds.
func errPathOutside(skillID, path string, cause error) error {
	return apperr.New("SKILLFILES_PATH_OUTSIDE").
		Causer("skillfiles.Files").
		Msgf("%q resolves outside skill %q's own directory", path, skillID).
		Issue("skill", skillID).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "every file a package writes must live under its own directory"})
}

func errWriteFailed(skillID, path string, cause error) error {
	return apperr.New("SKILLFILES_WRITE_FAILED").
		Causer("skillfiles.Files.Write").
		Msgf("writing %q for skill %q failed: %v", path, skillID, cause).
		Issue("skill", skillID).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace is writable and has space"})
}

func errRemoveFailed(skillID, path string, cause error) error {
	return apperr.New("SKILLFILES_REMOVE_FAILED").
		Causer("skillfiles.Files.Remove").
		Msgf("removing %q for skill %q failed: %v", path, skillID, cause).
		Issue("skill", skillID).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace is writable"})
}
