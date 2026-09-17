package workspace

import (
	"fmt"
	"strconv"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errInvalidName(name string) error {
	return apperr.New("WORKSPACE_INVALID_NAME").
		Causer("workspace.Service.Create").
		Msgf("%q does not produce a usable workspace identifier", name).
		Issue("name", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "use a name with at least one letter or digit",
			Command: build.Name + ` workspace create "Project Alpha"`,
			Tool:    "workspace_create",
		})
}

func errAlreadyExists(id string) error {
	return apperr.New("WORKSPACE_ALREADY_EXISTS").
		Causer("workspace.Service.Create").
		Msgf("a workspace named %q is already registered", id).
		Issue("workspace", id).
		Status(apperr.StatusConflict).
		CTA(
			apperr.CallToAction{
				Label:   "open the existing workspace instead",
				Command: build.Name + " workspace get " + id,
				Tool:    "workspace_get",
				Input:   map[string]any{"workspace": id},
			},
			apperr.CallToAction{
				Label:   "list what is already registered",
				Command: build.Name + " workspace list",
				Tool:    "workspace_list",
			},
		)
}

func errNotFound(id string) error {
	e := apperr.New("WORKSPACE_NOT_FOUND").
		Causer("workspace.Service.Get").
		Issue("workspace", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the registered workspaces",
			Command: build.Name + " workspace list",
			Tool:    "workspace_list",
		})
	if id == "" {
		return e.Msgf("no workspace was named and none is active").
			CTA(apperr.CallToAction{
				Label:   "register the repository you are in",
				Command: build.Name + " workspace introspect",
				Tool:    "workspace_introspect",
			})
	}
	return e.Msgf("no workspace named %q is registered", id)
}

// errRelativePath exists because a workspace path is resolved from three
// different places — a flag, an environment variable, the working directory —
// and a relative one would silently mean something different in each.
func errRelativePath(p string) error {
	return apperr.New("WORKSPACE_PATH_NOT_ABSOLUTE").
		Causer("workspace.Service.Create").
		Msgf("the workspace path must be absolute, and %q is not", p).
		Issue("path", p).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "pass an absolute path",
			Command: build.Name + " workspace create Alpha --path \"$PWD\"",
			Tool:    "workspace_create",
		})
}

func errScaffoldFailed(path string, cause error) error {
	return apperr.New("WORKSPACE_SCAFFOLD_FAILED").
		Causer("workspace.Service.scaffold").
		Msgf("could not lay out the workspace at %q", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errStoreFailed(op string, cause error) error {
	return apperr.New("WORKSPACE_STORE_FAILED").
		Causer("workspace.Service."+op).
		Msgf("the workspace registry could not be %s", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errSeedFailed(id string, cause error) error {
	return apperr.New("WORKSPACE_ORCHESTRATOR_SEED_FAILED").
		Causer("workspace.Service.Create").
		Msgf("the workspace was laid out but its orchestrator could not be created").
		Issue("workspace", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnknownField(path string) error {
	return apperr.New("WORKSPACE_UNKNOWN_FIELD").
		Causer("workspace.Service.Update").
		Msgf("%q is not a field of a workspace", path).
		Issue("field", path).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "read the workspace to see the fields it has",
			Command: build.Name + " workspace get",
			Tool:    "workspace_get",
		})
}

func errInvalidValue(field string, value any, want string, example map[string]any) error {
	shown := fmt.Sprint(value)
	if text, ok := value.(string); ok {
		// Quoted, so a blank name reads as "" rather than as nothing at all.
		shown = strconv.Quote(text)
	}
	return apperr.New("WORKSPACE_INVALID_VALUE").
		Causer("workspace.Service.Update").
		Msgf("%s cannot be %s: it takes %s", field, shown, want).
		Issue("field", field).
		Issue("value", value).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "set " + field + " to " + want,
			Tool:  "workspace_update",
			Input: map[string]any{"set": example},
		})
}

// errInvalidTaskType names the entry of the task-type list that was refused,
// and which of its fields, so a form can point at the row rather than at the
// whole list. index is -1 when the list itself is the problem.
func errInvalidTaskType(index int, field, value, want string) error {
	path := "tasks"
	if index >= 0 {
		path = fmt.Sprintf("tasks[%d].%s", index, field)
	}
	msg := fmt.Sprintf("the task types need %s", want)
	if index >= 0 {
		msg = fmt.Sprintf("task type %d (%s) needs %s", index+1, strconv.Quote(value), want)
	}
	return apperr.New("WORKSPACE_INVALID_TASK_TYPE").
		Causer("workspace.Service.Update").
		Msgf("%s", msg).
		Issue("field", path).
		Issue("value", value).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "read the task types, fix that entry and send the whole list again",
			Command: build.Name + " workspace get",
			Tool:    "workspace_get",
		})
}

func errAccessDenied(workspaceID, userID string) error {
	return apperr.New("WORKSPACE_ACCESS_DENIED").
		Causer("workspace.Service.AuthorizeWorkspace").
		Msgf("this account is not a member of %q", workspaceID).
		Issue("workspace", workspaceID).
		Issue("user", userID).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "list the workspaces this account can reach",
			Command: build.Name + " workspace list",
			Tool:    "workspace_list",
		})
}
