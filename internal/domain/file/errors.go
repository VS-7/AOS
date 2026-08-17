package file

import "github.com/OWNER/aos/internal/core/apperr"

func errWorkspaceUnavailable(cause error) error {
	return apperr.New("FILE_WORKSPACE_UNAVAILABLE").
		Causer("file.Service").
		Msgf("no workspace root is available to resolve files against").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errOutsideWorkspace(path string) error {
	return apperr.New("FILE_OUTSIDE_WORKSPACE").
		Causer("file.Service.resolve").
		Msgf("%q resolves outside the workspace", path).
		Issue("path", path).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "work inside the workspace root; a path that leaves it is not reachable from here",
		})
}

func errUnreadable(path string, cause error) error {
	return apperr.New("FILE_PATH_UNREADABLE").
		Causer("file.Service.resolve").
		Msgf("%q could not be resolved", path).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the spelling of the path, then list the directory it should be in"})
}

func errFSFailed(op, path string, cause error) error {
	return apperr.New("FILE_IO_FAILED").
		Causer("file.Service."+op).
		Msgf("could not %s %q", op, path).
		Issue("operation", op).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the path exists and that the workspace root is still on disk"})
}

func errNotDirectory(path string) error {
	return apperr.New("FILE_NOT_A_DIRECTORY").
		Causer("file.Service.Tree").
		Msgf("%q is not a directory", path).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "list its parent directory instead, or read it directly"})
}

func errIsDirectory(op, path string) error {
	return apperr.New("FILE_IS_A_DIRECTORY").
		Causer("file.Service."+op).
		Msgf("%q is a directory", path).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "list it instead of reading or writing it directly"})
}

func errAlreadyExists(path string) error {
	return apperr.New("FILE_ALREADY_EXISTS").
		Causer("file.Service.Move").
		Msgf("%q already exists", path).
		Issue("path", path).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "move to a different destination, or remove the existing path first"})
}

func errRootRemoval() error {
	return apperr.New("FILE_ROOT_REMOVAL").
		Causer("file.Service.Delete").
		Msgf("the workspace root itself cannot be removed").
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "remove the specific paths you meant, not the root"})
}

func errGitFailed(op, path string, cause error) error {
	return apperr.New("FILE_GIT_FAILED").
		Causer("file.Service."+op).
		Msgf("git could not answer for %q", path).
		Issue("operation", op).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
