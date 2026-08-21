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

// errNameRequired fires when Create is asked to name nothing.
func errNameRequired() error {
	return apperr.New("PROJECT_NAME_REQUIRED").
		Causer("project.Service.Create").
		Msgf("a project needs a name").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "name the project before creating it"})
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
