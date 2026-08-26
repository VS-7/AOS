package command

import "github.com/OWNER/aos/internal/core/apperr"

func errInvalidInput(key string, err error) *apperr.Error {
	return apperr.New("COMMAND_INVALID_INPUT").
		Causer(key).
		Msgf("the payload of %s could not be decoded: %v", key, err).
		Issue("tool", key).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "inspect the input schema for this action instead of retrying blindly",
			Tool:  key,
			Input: map[string]any{"schema": true},
		}).
		Wrap(err)
}

func errValidation(key string) *apperr.Error {
	return apperr.New("COMMAND_VALIDATION_FAILED").
		Causer(key).
		Msgf("the payload of %s is not valid", key).
		Issue("tool", key).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "read the issues, inspect the contract with schema:true, then fix the payload",
			Tool:  key,
			Input: map[string]any{"schema": true},
		})
}

// errUnroutable is raised when a routed registry cannot find the command it
// publishes in the registry a call resolved to.
//
// It should be unreachable: every backing registry is built by the same
// wiring, so a key in the published surface is a key in all of them. Reaching
// it means two registries were assembled by different code paths, and running
// the wrong workspace's handler instead would be a silent write to somebody
// else's directory — which is the failure routing exists to prevent.
func errUnroutable(key string) error {
	return apperr.New("COMMAND_UNROUTABLE").
		Causer("command.Route").
		Msgf("%s could not be routed to a workspace", key).
		Issue("command", key).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "restart the daemon; if it persists, this is a defect in how the workspace registries are built",
		})
}
