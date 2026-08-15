package config

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errFieldForbidden(path string) error {
	return apperr.New("CONFIG_FIELD_FORBIDDEN").
		Causer("config.Service.Update").
		Msgf("an agent may not change %q", path).
		Issue("field", path).
		Issue("allowed", strings.Join(agentWritable, ", ")).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "ask the workspace owner to change this setting in the app, or pick an agent-writable field",
			Command: build.Name + " config get",
			Tool:    "config_get",
		})
}

func errUnknownField(path string) error {
	return apperr.New("CONFIG_FIELD_UNKNOWN").
		Causer("config.Service.Update").
		Msgf("no configuration field at %q", path).
		Issue("field", path).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "read the current configuration to find the right path",
			Command: build.Name + " config get",
			Tool:    "config_get",
		})
}

func errRevealDenied() error {
	return apperr.New("CONFIG_REVEAL_DENIED").
		Causer("config.Service.Get").
		Msgf("secrets are never revealed to an agent").
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "run this on an interactive terminal to reveal a secret",
			Command: build.Name + " config get --reveal",
		})
}

func errLoadFailed(err error) error {
	return apperr.New("CONFIG_LOAD_FAILED").
		Causer("config.Service.load").
		Msgf("cannot read the configuration file").
		Status(apperr.StatusInternalServerError).
		Wrap(err)
}

func errSaveFailed(err error) error {
	return apperr.New("CONFIG_SAVE_FAILED").
		Causer("config.Service.Update").
		Msgf("cannot write the configuration file").
		Status(apperr.StatusInternalServerError).
		Wrap(err)
}
