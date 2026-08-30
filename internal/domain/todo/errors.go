package todo

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errTaskRequired(op string) error {
	return apperr.New("TODO_TASK_REQUIRED").
		Causer("todo.Service." + op).
		Msgf("a step identifier means nothing without the task it belongs to").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "pass the parent task",
			Command: build.Name + " todos list <task>",
			Tool:    "todos_list",
		})
}

func errParentMissing(taskID string) error {
	return apperr.New("TODO_TASK_NOT_FOUND").
		Causer("todo.Service.Create").
		Msgf("no task named %q exists, so a step cannot be added to it", taskID).
		Issue("task", taskID).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the tasks that exist",
			Command: build.Name + " tasks list",
			Tool:    "tasks_list",
		})
}

func errNotFound(taskID, id string) error {
	return apperr.New("TODO_NOT_FOUND").
		Causer("todo.Service.Get").
		Msgf("task %q has no step %q", taskID, id).
		Issue("task", taskID).
		Issue("todo", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "read the plan to see the steps it has",
			Command: build.Name + " todos list " + taskID,
			Tool:    "todos_list",
			Input:   map[string]any{"task": taskID},
		})
}

func errInvalidStatus(status string) error {
	return apperr.New("TODO_INVALID_STATUS").
		Causer("todo.Service.SetStatus").
		Msgf("%q is not a step status", status).
		Issue("status", status).
		Issue("valid", []string{"pending", "in_progress", "blocked", "finished", "skipped"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use one of the five listed in the issue",
		})
}

func errInvalidTransition(id string, from, to Status) error {
	return apperr.New("TODO_INVALID_TRANSITION").
		Causer("todo.Service.SetStatus").
		Msgf("a step that is %s cannot move to %s", from, to).
		Issue("todo", id).
		Issue("from", string(from)).
		Issue("to", string(to)).
		Issue("allowed", from.NextStates()).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "move to one of the states in the issue, or say why this step no longer applies and skip it",
		})
}

// errStatusIsNotAField exists because a field that silently did nothing would be
// worse than a refusal: a model would write it, read success, and believe the
// step had moved.
func errStatusIsNotAField(id string) error {
	return apperr.New("TODO_STATUS_NOT_WRITABLE").
		Causer("todo.Service.Update").
		Msgf("a step's status is moved, not written").
		Issue("todo", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "use set-status, which validates the move and records evidence",
			Command: build.Name + " todos set-status <task> " + id + " finished",
			Tool:    "todos_set-status",
		})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("TODO_READ_FAILED").
		Causer("todo.Service." + op).
		Msgf("the plan could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("TODO_WRITE_FAILED").
		Causer("todo.Service." + op).
		Msgf("the step could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
