package project

import "github.com/OWNER/aos/internal/core/apperr"

// errNotFound fires when no project has this id.
func errNotFound(id string) error {
	return apperr.New("PROJECT_NOT_FOUND").
		Causer("project.Service.Get").
		Msgf("no project %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "list projects",
			Tool:  "projects_list",
		})
}

// errNameRequired fires when Create's name has no letter or digit to slug.
func errNameRequired() error {
	return apperr.New("PROJECT_NAME_REQUIRED").
		Causer("project.Service.Create").
		Msgf("a project needs a name with at least one letter or digit").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "name the project with at least one letter or digit"})
}

// errSourceInvalid fires when Source fails one of the three checks the
// original enforces: absolute, exists, is a directory.
func errSourceInvalid(source, reason string) error {
	return apperr.New("PROJECT_SOURCE_INVALID").
		Causer("project.Service.validateSource").
		Msgf("project source %q is invalid: %s", source, reason).
		Issue("source", source).
		Issue("reason", reason).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "provide an absolute path to an existing directory on the host machine"})
}

// errReadFailed wraps a repository failure that is not "not found".
func errReadFailed(op string, cause error) error {
	return apperr.New("PROJECT_READ_FAILED").
		Causer("project.Service."+op).
		Msgf("could not read projects: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errWriteFailed wraps a repository failure while writing.
func errWriteFailed(op string, cause error) error {
	return apperr.New("PROJECT_WRITE_FAILED").
		Causer("project.Service."+op).
		Msgf("could not save the project: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errAlreadyExists fires when the id Create derives is one another project
// already has. It used to surface as PROJECT_WRITE_FAILED, a 500 whose call
// to action called it a bug, when the fix is the person's: another name.
func errAlreadyExists(id string) error {
	return apperr.New("PROJECT_ALREADY_EXISTS").
		Causer("project.Service.Create").
		Msgf("a project %q already exists", id).
		Issue("id", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "choose a different name, or update the existing project instead",
			Tool:  "projects_update",
		})
}
