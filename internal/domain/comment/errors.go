package comment

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errTaskRequired(op string) error {
	return apperr.New("COMMENT_TASK_REQUIRED").
		Causer("comment.Service."+op).
		Msgf("a comment identifier means nothing without the task it belongs to").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "pass the parent task",
			Command: build.Name + " comments list <task>",
			Tool:    "comments_list",
		})
}

func errParentMissing(taskID string) error {
	return apperr.New("COMMENT_TASK_NOT_FOUND").
		Causer("comment.Service.Create").
		Msgf("no task named %q exists, so there is nothing to comment on", taskID).
		Issue("task", taskID).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the tasks that exist",
			Command: build.Name + " tasks list",
			Tool:    "tasks_list",
		})
}

func errParentCommentMissing(taskID, parentID string) error {
	return apperr.New("COMMENT_PARENT_NOT_FOUND").
		Causer("comment.Service.Create").
		Msgf("task %q has no comment %q to reply to", taskID, parentID).
		Issue("task", taskID).
		Issue("parent", parentID).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "read the discussion to find the comment you meant",
			Command: build.Name + " comments list " + taskID,
			Tool:    "comments_list",
			Input:   map[string]any{"task": taskID},
		})
}

func errNotFound(taskID, id string) error {
	return apperr.New("COMMENT_NOT_FOUND").
		Causer("comment.Service.Get").
		Msgf("task %q has no comment %q", taskID, id).
		Issue("task", taskID).
		Issue("comment", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "read the discussion to see what is there",
			Command: build.Name + " comments list " + taskID,
			Tool:    "comments_list",
			Input:   map[string]any{"task": taskID},
		})
}

// errNoActor is what an unattributable write looks like. A comment with no
// author would be a hole in the one audit trail this aggregate exists to keep.
func errNoActor() error {
	return apperr.New("COMMENT_NO_ACTOR").
		Causer("comment.Service.Create").
		Msgf("this request carries no identity, and a comment nobody wrote is not a comment").
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label: "make the call as an agent or a signed-in user; authorship is taken from the request, never from the payload",
		})
}

func errForbidden(id, author, actor string) error {
	return apperr.New("COMMENT_FORBIDDEN").
		Causer("comment.Service.guardOwnership").
		Msgf("this comment was written by %q, and you are %q", author, actor).
		Issue("comment", id).
		Issue("author", author).
		Issue("actor", actor).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "reply to it instead of editing it; a discussion where anyone can rewrite anyone is not a record of anything",
			Tool:  "comments_create",
		})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("COMMENT_READ_FAILED").
		Causer("comment.Service."+op).
		Msgf("the discussion could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("COMMENT_WRITE_FAILED").
		Causer("comment.Service."+op).
		Msgf("the comment could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
