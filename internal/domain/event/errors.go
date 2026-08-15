package event

import (
	"github.com/OWNER/aos/internal/core/apperr"
)

// errHookFailed is only reachable in strict mode, where a workspace has
// declared that a hook it cannot run is a reason to stop rather than a warning.
func errHookFailed(hookID string, t Type, cause error) error {
	return apperr.New("HOOK_FAILED").
		Causer("event.Service.Emit").
		Msgf("hook %q failed on %s and this workspace runs hooks in strict mode", hookID, t).
		Issue("hook", hookID).
		Issue("event", string(t)).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "fix the hook, or turn strict mode off to let a failing hook be a warning",
		})
}

// errBlocked is what a blocking UserPromptSubmit hook produces. It is a 403
// rather than a 500 because nothing malfunctioned: policy said no.
func errBlocked(t Type, hookID, reason string) error {
	if reason == "" {
		reason = "no reason given"
	}
	return apperr.New("HOOK_BLOCKED").
		Causer("event.Service.Emit").
		Msgf("%s was blocked by a hook: %s", t, reason).
		Issue("event", string(t)).
		Issue("hook", hookID).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "the hook that blocked this is named in the issue; its reason is the message",
		})
}

// Blocked builds the error a blocking hook produces, so the runtime does not
// have to know how to phrase it.
func Blocked(t Type, o Outcome) error { return errBlocked(t, o.HookID, o.Reason) }
