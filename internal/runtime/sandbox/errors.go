package sandbox

import (
	"path/filepath"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
)

func errNoRoot() error {
	return apperr.New("SANDBOX_NO_ROOT").
		Causer("sandbox.New").
		Msgf("a sandbox needs a workspace or a worktree to be confined to").
		Status(apperr.StatusInternalServerError)
}

func errRelativeRoot(root string) error {
	return apperr.New("SANDBOX_RELATIVE_ROOT").
		Causer("sandbox.New").
		Msgf("the sandbox root must be absolute, and %q is not", root).
		Issue("root", root).
		Status(apperr.StatusInternalServerError)
}

func errRootUnreadable(root string, cause error) error {
	return apperr.New("SANDBOX_ROOT_UNREADABLE").
		Causer("sandbox.New").
		Msgf("the sandbox root %q could not be resolved", root).
		Issue("root", root).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errPathUnreadable(path string, cause error) error {
	return apperr.New("SANDBOX_PATH_UNREADABLE").
		Causer("sandbox.checkFS").
		Msgf("%q could not be resolved", path).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the spelling of the path, then list the directory it should be in"})
}

// errPathOutside is what a traversal attempt looks like from the inside: not a
// blocked attack, just a path that is not in the workspace.
func errPathOutside(asked, resolved, root string) error {
	return apperr.New("SANDBOX_PATH_OUTSIDE").
		Causer("sandbox.checkFS").
		Msgf("%q resolves outside the workspace", asked).
		Issue("path", asked).
		Issue("resolved", resolved).
		Issue("root", root).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "work inside the workspace root named in the issue; a path that leaves it is not reachable from here",
		})
}

func errTmpReadOnly(resolved string, op Op) error {
	return apperr.New("SANDBOX_TMP_READ_ONLY").
		Causer("sandbox.checkFS").
		Msgf("the temporary output directory can be read and not written to").
		Issue("path", resolved).
		Issue("operation", string(op)).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "read the spilled output from here, and write anything you want to keep inside the workspace",
		})
}

func errGitReadOnly(resolved string, op Op) error {
	return apperr.New("SANDBOX_GIT_READ_ONLY").
		Causer("sandbox.checkFS").
		Msgf("the .git directory can be read and not written to").
		Issue("path", resolved).
		Issue("operation", string(op)).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "change the repository through the git command, which keeps it consistent",
			Command: "git status",
		})
}

func errPermissionDenied(op Op, resolved string, held Permissions) error {
	return apperr.New("SANDBOX_PERMISSION_DENIED").
		Causer("sandbox.checkFS").
		Msgf("this agent may not %s; it holds %s", op, strings.Join(held.List(), ", ")).
		Issue("operation", string(op)).
		Issue("path", resolved).
		Issue("held", held.List()).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "ask the workspace owner to widen the agent's permissions",
			Command: "add \"" + string(op) + "\" under sandbox.permissions in the agent's file",
		})
}

func errRootRemoval(root string) error {
	return apperr.New("SANDBOX_ROOT_REMOVAL").
		Causer("sandbox.Remove").
		Msgf("the workspace root itself cannot be removed").
		Issue("root", root).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "remove the specific paths you meant, not the root"})
}

func errIO(op, path string, cause error) error {
	return apperr.New("SANDBOX_IO_FAILED").
		Causer("sandbox."+op).
		Msgf("could not %s %q", op, path).
		Issue("operation", op).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the underlying reason is in the message; check that the path exists and is readable"})
}

func errExecPermissionRequired(name string) error {
	return apperr.New("SANDBOX_EXEC_PERMISSION_REQUIRED").
		Causer("sandbox.VerifyExec").
		Msgf("this agent may not run commands").
		Issue("command", name).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "ask the workspace owner to grant execution and list the binary",
			Command: "add \"execute\" under sandbox.permissions in the agent's file",
		})
}

func errExecNotFound(name string) error {
	return apperr.New("SANDBOX_EXEC_NOT_FOUND").
		Causer("sandbox.VerifyExec").
		Msgf("%q is not installed, or is not on the controlled search path", name).
		Issue("command", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "if the binary lives outside the standard directories, list it by absolute path in the agent's policy",
		})
}

// errExecNotAllowed carries the exact line to add. The friction of an allowlist
// is real, and an error that says only "denied" turns it into a wall; an error
// that says what to write turns it into a decision somebody makes once.
func errExecNotAllowed(name, resolved string) error {
	return apperr.New("SANDBOX_EXEC_NOT_ALLOWED").
		Causer("sandbox.VerifyExec").
		Msgf("%q is not in this agent's allowlist", name).
		Issue("command", name).
		Issue("resolved", resolved).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "add the binary to the agent's policy, or ask the workspace owner to",
			Command: "add \"" + filepath.Base(resolved) + "\" under sandbox.exec.allow in the agent's file",
		})
}

func errShellNotAllowed(name, resolved string) error {
	return apperr.New("SANDBOX_SHELL_NOT_ALLOWED").
		Causer("sandbox.VerifyExec").
		Msgf("%q is a shell, and this agent may not reach one", name).
		Issue("command", name).
		Issue("resolved", resolved).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "run the program directly instead of through a shell; a shell would make the allowlist a suggestion",
		})
}

func errDeniedArgs(pattern, line string) error {
	return apperr.New("SANDBOX_ARGS_DENIED").
		Causer("sandbox.VerifyExec").
		Msgf("this command line matches a denied pattern in the agent's policy").
		Issue("pattern", pattern).
		Issue("command", line).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "the pattern that matched is in the issue; a different approach is needed, not a different spelling",
		})
}

func errExecFailed(name string, cause error) error {
	return apperr.New("SANDBOX_EXEC_FAILED").
		Causer("sandbox.Run").
		Msgf("%q could not be started", name).
		Issue("command", name).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
