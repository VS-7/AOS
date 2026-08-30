package command

import "github.com/OWNER/aos/internal/core/apperr"

// IssueCommand is the key under which an error names the command that failed.
//
// It is underscored because `issue` also carries one entry per field violation,
// keyed by the field's own JSON name — and one command's payload has a field
// genuinely called "tool" (toolsets_call). While the metadata used that key,
// that one command out of ~140 reported "the tool field is required" where
// every other reported which command failed, so a log or client parsing
// `issue.tool` read the wrong thing for exactly it (defect #6). The prefix is
// the convention the rest of the wire already uses for what belongs to the
// envelope rather than to the domain: `_reasoning`, `_cta`, `_tokens`.
const IssueCommand = "_command"

func errInvalidInput(key string, err error) *apperr.Error {
	return apperr.New("COMMAND_INVALID_INPUT").
		Causer(key).
		Msgf("the payload of %s could not be decoded: %v", key, err).
		Issue(IssueCommand, key).
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
		Issue(IssueCommand, key).
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
		Issue(IssueCommand, key).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "restart the daemon; if it persists, this is a defect in how the workspace registries are built",
		})
}
