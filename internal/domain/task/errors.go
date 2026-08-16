package task

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errInvalidName(name string) error {
	return apperr.New("TASK_INVALID_NAME").
		Causer("task.Service.Create").
		Msgf("%q does not produce a usable task name", name).
		Issue("name", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "name the work with at least one letter or digit",
			Command: build.Name + ` tasks create "Fix the denial pattern"`,
			Tool:    "tasks_create",
		})
}

func errNotFound(id string) error {
	return apperr.New("TASK_NOT_FOUND").
		Causer("task.Service.Get").
		Msgf("no task named %q exists", id).
		Issue("task", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the tasks that exist",
			Command: build.Name + " tasks list",
			Tool:    "tasks_list",
		})
}

func errInvalidStatus(status string) error {
	return apperr.New("TASK_INVALID_STATUS").
		Causer("task.Service.SetStatus").
		Msgf("%q is not a task status", status).
		Issue("status", status).
		Issue("valid", []string{
			"suggestion", "backlog", "planning", "todo",
			"in_progress", "stopped", "in_review", "finished",
		}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use one of the eight listed in the issue",
		})
}

func errInvalidPriority(priority string) error {
	return apperr.New("TASK_INVALID_PRIORITY").
		Causer("task.Service.Create").
		Msgf("%q is not a priority", priority).
		Issue("priority", priority).
		Issue("valid", []string{"no_priority", "urgent", "high", "medium", "low"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use one of the five listed in the issue",
		})
}

// errNotAnEntryPoint keeps creation from routing around the lifecycle. A task
// created straight into in_progress would skip every guard on the way there.
func errNotAnEntryPoint(status Status) error {
	return apperr.New("TASK_NOT_AN_ENTRY_POINT").
		Causer("task.Service.Create").
		Msgf("a task cannot be created directly in %s", status).
		Issue("status", string(status)).
		Issue("valid", []string{"suggestion", "backlog", "planning", "todo"}).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "create it in one of the four entry states and move it from there, so the guards on the way run",
			Tool:  "tasks_set_status",
		})
}

func errInvalidTransition(id string, from, to Status) error {
	return apperr.New("TASK_INVALID_TRANSITION").
		Causer("task.Service.SetStatus").
		Msgf("a task that is %s cannot move to %s", from, to).
		Issue("task", id).
		Issue("from", string(from)).
		Issue("to", string(to)).
		Issue("allowed", from.NextStates()).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "move to one of the states in the issue; finished work is reopened by creating the task that says what was wrong with it",
		})
}

// errStatusIsNotAField is the original's prose rule made mechanical: "use
// set_status for lifecycle moves; never change status via update".
func errStatusIsNotAField(id string) error {
	return apperr.New("TASK_STATUS_NOT_WRITABLE").
		Causer("task.Service.Update").
		Msgf("a task's status is moved, not written").
		Issue("task", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "use set-status, which validates the move and runs the guards on it",
			Command: build.Name + " tasks set-status " + id + " in_review",
			Tool:    "tasks_set_status",
			Input:   map[string]any{"id": id, "status": "in_review"},
		})
}

// errReviewBlocked is the master prompt's hardest rule, enforced. The original
// states it as guidance and hopes.
func errReviewBlocked(id string, pending int, ids []string) error {
	return apperr.New("TASK_REVIEW_BLOCKED").
		Causer("task.Service.guardReview").
		Msgf("this task has %d step(s) still open, so it is not ready for review", pending).
		Issue("task", id).
		Issue("pending", pending).
		Issue("todos", ids).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label:   "finish each open step with the evidence for it, or skip the ones that stopped applying",
			Command: build.Name + " todos list " + id,
			Tool:    "todos_list",
			Input:   map[string]any{"task": id},
		})
}

func errDependenciesPending(id string, blocked []string) error {
	return apperr.New("TASK_DEPENDENCIES_PENDING").
		Causer("task.Service.guardDependencies").
		Msgf("this task waits on %d task(s) that are not finished", len(blocked)).
		Issue("task", id).
		Issue("blockedBy", blocked).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "finish what it depends on first, or drop the dependency if it no longer holds",
			Tool:  "tasks_update",
			Input: map[string]any{"id": id, "dependsOn": []string{}},
		})
}

func errSelfDependency(id string) error {
	return apperr.New("TASK_SELF_DEPENDENCY").
		Causer("task.Service.checkDependencies").
		Msgf("a task cannot depend on itself").
		Issue("task", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "remove the task's own identifier from its dependency list",
		})
}

func errDependencyCycle(id, through string) error {
	return apperr.New("TASK_DEPENDENCY_CYCLE").
		Causer("task.Service.checkDependencies").
		Msgf("this dependency closes a cycle back to %q, and neither task could ever start", id).
		Issue("task", id).
		Issue("through", through).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "break the cycle: one of the two tasks has to be able to start without the other",
		})
}

func errUnknownType(kind string, known []string) error {
	return apperr.New("TASK_UNKNOWN_TYPE").
		Causer("task.Service.Create").
		Msgf("%q is not a task type of this workspace", kind).
		Issue("type", kind).
		Issue("valid", known).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "use one of the workspace's types, or add this one to the taxonomy",
			Command: build.Name + " workspace get",
			Tool:    "workspace_get",
		})
}

func errInvalidTime(field, raw string, cause error) error {
	return apperr.New("TASK_INVALID_TIME").
		Causer("task.Service.Create").
		Msgf("%q is not an RFC3339 instant", raw).
		Issue("field", field).
		Issue("value", raw).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "write the instant as 2026-03-01T12:00:00Z",
		})
}

func errWorktreesUnavailable(id string) error {
	return apperr.New("TASK_WORKTREES_UNAVAILABLE").
		Causer("task.Service.Branch").
		Msgf("this installation cannot create isolated checkouts").
		Issue("task", id).
		Status(apperr.StatusNotImplemented).
		CTA(apperr.CallToAction{
			Label: "run the task in the working tree, or install git and restart the daemon",
		})
}

func errWorktreeFailed(id, branch string, cause error) error {
	return apperr.New("TASK_WORKTREE_FAILED").
		Causer("task.Service.Branch").
		Msgf("the isolated checkout could not be created").
		Issue("task", id).
		Issue("branch", branch).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWorktreeLimit(limit, existing int) error {
	return apperr.New("TASK_WORKTREE_LIMIT").
		Causer("task.Service.pruneToLimit").
		Msgf("there are already %d checkouts and the workspace allows %d, and none of the extras belongs to finished work", existing, limit).
		Issue("limit", limit).
		Issue("existing", existing).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "finish or stop a task that holds a checkout, or raise the workspace's worktree limit",
			Command: build.Name + " tasks list --status in_progress",
			Tool:    "tasks_list",
			Input:   map[string]any{"status": "in_progress"},
		})
}

// errSetupFailed reports a checkout that exists with a setup that did not run.
// It is not a failure of Branch: destroying the branch over a failed script
// would lose whatever the script did manage to do.
func errSetupFailed(id, path string, cause error) error {
	return apperr.New("TASK_SETUP_SCRIPT_FAILED").
		Causer("task.Service.Branch").
		Msgf("the checkout was created but the workspace setup script failed in it").
		Issue("task", id).
		Issue("path", path).
		Status(apperr.StatusUnprocessableEntity).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "the checkout is usable; install what the task needs from inside it, or fix the workspace onCreateScript",
		})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("TASK_READ_FAILED").
		Causer("task.Service."+op).
		Msgf("the task could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("TASK_WRITE_FAILED").
		Causer("task.Service."+op).
		Msgf("the task could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
