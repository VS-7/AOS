package activity

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errIncomplete(namespace, event string) error {
	return apperr.New("ACTIVITY_INCOMPLETE").
		Causer("activity.Service.Publish").
		Msgf("an activity needs both a namespace and an event, and this one has %q and %q", namespace, event).
		Issue("namespace", namespace).
		Issue("event", event).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "name the kind of thing and what happened to it, as in namespace \"task\" and event \"status_changed\"",
		})
}

func errNotFound(id string) error {
	return apperr.New("ACTIVITY_NOT_FOUND").
		Causer("activity.Service.Get").
		Msgf("no activity is recorded under %q", id).
		Issue("activity", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list what the log holds",
			Command: build.Name + " activity list",
			Tool:    "activity_list",
		})
}

func errInvalidTime(field, raw string, cause error) error {
	return apperr.New("ACTIVITY_INVALID_TIME").
		Causer("activity.Service.List").
		Msgf("%q is not an RFC3339 instant", raw).
		Issue("field", field).
		Issue("value", raw).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "write the instant as 2026-03-01T12:00:00Z",
		})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("ACTIVITY_READ_FAILED").
		Causer("activity.Service."+op).
		Msgf("the activity log could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("ACTIVITY_WRITE_FAILED").
		Causer("activity.Service."+op).
		Msgf("the activity log could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
