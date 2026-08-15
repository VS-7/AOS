package mcpserver

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
)

func errInvalidComposite(tool string, err error) error {
	return apperr.New("COMMAND_INVALID_INPUT").
		Causer("mcpserver.composite").
		Msgf("the payload of %s could not be decoded: %v", tool, err).
		Issue("tool", tool).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "call the tool with schema:true to see the shape it expects",
			Tool:  tool,
			Input: map[string]any{"schema": true},
		}).
		Wrap(err)
}

func errReasoningRequired(tool string) error {
	return apperr.New("COMMAND_REASONING_REQUIRED").
		Causer("mcpserver.composite").
		Msgf("%s", command.ReasoningRejection).
		Issue("tool", tool).
		Issue("field", command.ReasoningField).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "resend the call with _reasoning explaining why this tool is being called now",
			Tool:  tool,
		})
}

func errUnknownAction(tool, action string, known []string) error {
	return apperr.New("COMMAND_ACTION_UNKNOWN").
		Causer("mcpserver.composite").
		Msgf("%q has no action %q", tool, action).
		Issue("tool", tool).
		Issue("action", action).
		Issue("actions", strings.Join(known, ", ")).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "call the tool with schema:true to see every action and its input",
			Tool:  tool,
			Input: map[string]any{"schema": true},
		})
}
