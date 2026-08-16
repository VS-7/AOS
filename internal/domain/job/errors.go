package job

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errNoQueue(op string) error {
	return apperr.New("JOB_QUEUE_UNAVAILABLE").
		Causer("job.Service."+op).
		Msgf("this process has no queue: deferred work runs in the daemon").
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{
			Label:   "start the daemon and ask it instead",
			Command: build.Name + " gateway start",
			Tool:    "gateway_start",
		})
}

func errNotFound(id string) error {
	return apperr.New("JOB_NOT_FOUND").
		Causer("job.Service.Get").
		Msgf("no job is recorded under %q", id).
		Issue("job", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list what the queue holds",
			Command: build.Name + " jobs list",
			Tool:    "jobs_list",
		})
}

func errInvalidStatus(status string) error {
	return apperr.New("JOB_INVALID_STATUS").
		Causer("job.Service.List").
		Msgf("%q is not a job status", status).
		Issue("status", status).
		Issue("valid", []string{"pending", "claimed", "succeeded", "failed", "dead"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of the five listed in the issue"})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("JOB_READ_FAILED").
		Causer("job.Service."+op).
		Msgf("the queue could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("JOB_WRITE_FAILED").
		Causer("job.Service."+op).
		Msgf("the queue could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
