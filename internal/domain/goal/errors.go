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

// errPriorityInvalid mirrors errStatusInvalid: a closed set, and a refusal
// that names the accepted values rather than saying only "invalid".
func errPriorityInvalid(raw string) error {
	return apperr.New("GOAL_PRIORITY_INVALID").
		Causer("goal.Service").
		Msgf("%q is not a goal priority", raw).
		Issue("priority", raw).
		Issue("allowed", "no_priority, urgent, high, medium, low").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of no_priority, urgent, high, medium, low"})
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

// errAlreadyExists fires when the id Create derives from a title is one
// another goal already has — two titles can differ only in case or
// punctuation and still slug alike. It used to surface as GOAL_WRITE_FAILED,
// a 500 telling the person to retry.
func errAlreadyExists(id string) error {
	return apperr.New("GOAL_ALREADY_EXISTS").
		Causer("goal.Service.Create").
		Msgf("a goal %q already exists", id).
		Issue("id", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "choose a different title, or update the existing goal instead",
			Tool:  "goals_update",
		})
}

// errDueAtInvalid fires when an update's due date is neither empty nor an
// RFC3339 instant.
func errDueAtInvalid(raw string) error {
	return apperr.New("GOAL_DUE_AT_INVALID").
		Causer("goal.Service.Update").
		Msgf("%q is not an RFC3339 instant", raw).
		Issue("dueAt", raw).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "send an RFC3339 instant such as 2026-09-20T00:00:00Z, or an empty string to clear the deadline"})
}
