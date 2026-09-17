package task

import (
	"strconv"

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
			Tool:  "tasks_set-status",
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
			Tool:    "tasks_set-status",
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

// errWorktreeFailed carries what git said in the message itself. The cause is
// not rendered on any surface, and "could not be created" with the reason only
// in there left a person and an agent alike with nothing to act on.
func errWorktreeFailed(id, branch string, cause error) error {
	return apperr.New("TASK_WORKTREE_FAILED").
		Causer("task.Service.Branch").
		Msgf("the isolated checkout could not be created: %s", reasonOf(cause)).
		Issue("task", id).
		Issue("branch", branch).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "fix what git reported and branch again, or execute the task in the workspace without an isolated checkout",
			Tool:  "tasks_branch",
			Input: map[string]any{"id": id},
		})
}

// errWorktreeNoRepository is a workspace checkouts cannot be cut from, because
// it is in no Git repository at all.
func errWorktreeNoRepository(id string, source WorktreeSource) error {
	return apperr.New("TASK_WORKTREE_NO_REPOSITORY").
		Causer("task.Service.Branch").
		Msgf("the workspace at %s is not in a Git repository, so no isolated checkout can be cut from it", source.Dir).
		Issue("task", id).
		Issue("workspace", source.Dir).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "make the workspace a Git repository and commit once, then branch again",
			Command: "git -C " + strconv.Quote(source.Dir) + " init && git -C " + strconv.Quote(source.Dir) + " commit --allow-empty -m \"Start the workspace\"",
		}, apperr.CallToAction{
			Label: "or execute the task in the workspace itself, without an isolated checkout",
		})
}

// errWorktreeBaseMissing is a repository with nothing to check out: the base
// names no commit. A repository nobody has committed to yet is the usual case —
// and when it is a repository the workspace only sits inside, such as a home
// directory under version control, it is named, because the fix may be to
// give the workspace a repository of its own instead.
//
// Each case is one whole chain from apperr.New to its CTA: the error catalog
// is read from the source, and a CTA attached to a builder held in a variable
// is one it cannot see, so the refusal was catalogued as a 409 with nothing to
// do about it.
func errWorktreeBaseMissing(id, base string, source WorktreeSource) error {
	named := base
	if named == "" {
		named = "HEAD"
	}
	if source.Own || source.Toplevel == "" {
		repo := source.Toplevel
		if repo == "" {
			repo = source.Dir
		}
		return apperr.New("TASK_WORKTREE_BASE_MISSING").
			Causer("task.Service.Branch").
			Msgf("there is no commit on %q to cut the task's branch from", named).
			Issue("task", id).
			Issue("base", named).
			Issue("workspace", source.Dir).
			Status(apperr.StatusConflict).
			CTA(apperr.CallToAction{
				Label:   "commit once on " + named + " in " + repo + ", then branch again",
				Command: "git -C " + strconv.Quote(repo) + " commit --allow-empty -m \"Start the workspace\"",
			}, apperr.CallToAction{
				Label: "or branch again naming a base that exists",
				Tool:  "tasks_branch",
				Input: map[string]any{"id": id},
			})
	}
	// The enclosing repository is offered only as something to decide on, never
	// as a command: it is as often a home directory under version control as a
	// monorepo, and a commit there set an agent on a chain that ended with the
	// workspace — secrets included — staged into it (see errWorktreeNotCommitted).
	return apperr.New("TASK_WORKTREE_BASE_MISSING").
		Causer("task.Service.Branch").
		Msgf("the workspace at %s is a folder of the repository at %s, and there is no commit on %q there to cut the task's branch from", source.Dir, source.Toplevel, named).
		Issue("task", id).
		Issue("base", named).
		Issue("workspace", source.Dir).
		Issue("enclosingRepository", source.Toplevel).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "give the workspace a repository of its own and commit once, then branch again",
			Command: "git -C " + strconv.Quote(source.Dir) + " init && git -C " + strconv.Quote(source.Dir) + " commit --allow-empty -m \"Start the workspace\"",
		}, apperr.CallToAction{
			Label: "or, only if the workspace is part of the project at " + source.Toplevel + ", have its owner make the first commit there, then branch again",
		})
}

// errWorktreeNotCommitted is a workspace that is a folder of a project the
// project never committed: a checkout of the project would not contain it.
//
// The enclosing repository is frequently not a project at all but a home
// directory under version control, which is usually pushed somewhere. The
// refusal used to hand over `git add <folder> && git commit` there as the thing
// to run, and that staged the workspace's .env into it. A repository of the
// workspace's own is what is offered to run; committing the folder into the
// enclosing one is left as a decision, with what to look out for.
func errWorktreeNotCommitted(id string, source WorktreeSource) error {
	return apperr.New("TASK_WORKTREE_NOT_COMMITTED").
		Causer("task.Service.Branch").
		Msgf("the workspace at %s is the folder %s of the repository at %s, which has not committed it, so a checkout of that repository would not contain the workspace", source.Dir, source.Subdir, source.Toplevel).
		Issue("task", id).
		Issue("workspace", source.Dir).
		Issue("enclosingRepository", source.Toplevel).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "give the workspace a repository of its own and commit once, then branch again",
			Command: "git -C " + strconv.Quote(source.Dir) + " init && git -C " + strconv.Quote(source.Dir) + " commit --allow-empty -m \"Start the workspace\"",
		}, apperr.CallToAction{
			Label: "or, only if the workspace is part of the project at " + source.Toplevel + ", commit its folder there yourself — review what git stages first so secrets such as .env stay out — then branch again",
		}, apperr.CallToAction{
			Label: "or execute the task in the workspace itself, without an isolated checkout",
		})
}

// reasonOf is the sentence closest to what went wrong: an application error's
// own message rather than its code, or the error as it came.
func reasonOf(err error) string {
	if app, ok := apperr.As(err); ok && app.Message != "" {
		return app.Message
	}
	return err.Error()
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
		Causer("task.Service." + op).
		Msgf("the task could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("TASK_WRITE_FAILED").
		Causer("task.Service." + op).
		Msgf("the task could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
