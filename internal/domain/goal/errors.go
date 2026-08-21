package goal

import "github.com/OWNER/aos/internal/core/apperr"

// errNotFound fires when no goal has this ID.
func errNotFound(id string) error {
	return apperr.New("GOAL_NOT_FOUND").
		Causer("goal.Service.Get").
		Msgf("no goal %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "list goals",
			Tool:  "goals_list",
		})
}

// errTitleRequired fires when Create is given a blank title, which has
// nothing to derive an id from.
func errTitleRequired() error {
	return apperr.New("GOAL_TITLE_REQUIRED").
		Causer("goal.Service.Create").
		Msgf("a goal needs a title to derive its id from").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "give the goal a non-blank title"})
}

// errStatusInvalid fires when a status is set to something outside the
// four-member union.
func errStatusInvalid(raw string) error {
	return apperr.New("GOAL_STATUS_INVALID").
		Causer("goal.Service").
		Msgf("%q is not a goal status", raw).
		Issue("status", raw).
		Issue("allowed", "active, achieved, abandoned, paused").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of active, achieved, abandoned, paused"})
}

// errReadFailed wraps a repository failure that is not "not found".
func errReadFailed(op string, cause error) error {
	return apperr.New("GOAL_READ_FAILED").
		Causer("goal.Service."+op).
		Msgf("could not read goals: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errWriteFailed wraps a repository failure while writing.
func errWriteFailed(op string, cause error) error {
	return apperr.New("GOAL_WRITE_FAILED").
		Causer("goal.Service."+op).
		Msgf("could not save the goal: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
