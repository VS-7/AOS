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
