package skillhooks

import "github.com/OWNER/aos/internal/core/apperr"

// errResolveFailed fires when this adapter cannot resolve where a skill's
// own directory lives, before it can confine a hook's Command against it.
func errResolveFailed(skillID string, cause error) error {
	return apperr.New("SKILLHOOKS_RESOLVE_FAILED").
		Causer("skillhooks.Hooks.Register").
		Msgf("could not resolve the directory of skill %q: %v", skillID, cause).
		Issue("skill", skillID).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace root is readable"})
}

// errScriptNotExecutable fires when a hook's own script cannot be made
// executable — most likely because Register ran before Apply's own file
// write reached disk, or the write landed with permissions this process
// cannot change.
func errScriptNotExecutable(skillID, hookID, command string, cause error) error {
	return apperr.New("SKILLHOOKS_SCRIPT_NOT_EXECUTABLE").
		Causer("skillhooks.Hooks.Register").
		Msgf("hook %q of skill %q: could not make %q executable: %v", hookID, skillID, command, cause).
		Issue("skill", skillID).
		Issue("hook", hookID).
		Issue("command", command).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errCommandOutside fires when a hook's Command names a path that resolves
// outside the declaring skill's own installed directory — the same
// containment failure artifactfiles.errPathOutside reports for an
// artifact's entrypoint.
func errCommandOutside(skillID, hookID, command string, cause error) error {
	return apperr.New("SKILLHOOKS_COMMAND_OUTSIDE").
		Causer("skillhooks.Hooks.Register").
		Msgf("hook %q of skill %q: command %q resolves outside the skill's own directory: %v", hookID, skillID, command, cause).
		Issue("skill", skillID).
		Issue("hook", hookID).
		Issue("command", command).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "a hook's command must live inside the skill's own package"})
}
