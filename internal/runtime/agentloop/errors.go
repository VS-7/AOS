package agentloop

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errNoProvider() error {
	return apperr.New("AGENT_NO_PROVIDER").
		Causer("agentloop.Loop.Run").
		Msgf("this turn has no model provider").
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label:   "configure a provider and a default model",
			Command: build.Name + " config update --set agents.models.default.provider=openai",
		})
}

// errProviderNotEnabled is the fifth level of the model cascade: nothing was
// configured anywhere.
func errProviderNotEnabled(model string) error {
	e := apperr.New("AGENT_PROVIDER_NOT_ENABLED").
		Causer("agentloop.Resolve").
		Msgf("no model provider is configured for this agent").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "set a default provider and model for the installation",
			Command: build.Name + " config update --set agents.models.default.provider=openai --set agents.models.default.model=gpt-5",
			Tool:    "config_update",
		})
	if model != "" {
		e = e.Issue("model", model).
			CTA(apperr.CallToAction{
				Label: "or name the provider in the agent's front matter, as \"" + model + " (openai)\"",
			})
	}
	return e
}

func errProviderFailed(provider string, cause error) error {
	return apperr.New("AGENT_PROVIDER_FAILED").
		Causer("agentloop.Loop.call").
		Msgf("the %s provider did not answer", provider).
		Issue("provider", provider).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "the provider's own message is in the cause; a credential or a rate limit is the usual reason",
		})
}

// errTurnCancelled is a turn that was interrupted: the person navigated away,
// the daemon is shutting down, or the total timeout elapsed. It carries what
// was spent before the interruption, because that is billed either way.
func errTurnCancelled(steps int, used Usage, cause error) error {
	return apperr.New("AGENT_TURN_CANCELLED").
		Causer("agentloop.Loop.Run").
		Msgf("the turn was interrupted after %d model calls", steps).
		Issue("steps", steps).
		Issue("tokens", used.Total).
		Status(apperr.StatusRequestTimeout).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "ask again if this was not deliberate; the work already done is in the conversation",
		})
}

// errStepsExhausted is the ceiling the original does not have. It carries the
// usage, because the first question anybody asks about a runaway turn is what
// it cost.
func errStepsExhausted(limit int, used Usage) error {
	return apperr.New("AGENT_STEPS_EXHAUSTED").
		Causer("agentloop.Loop.Run").
		Msgf("the turn reached its ceiling of %d model calls without finishing", limit).
		Issue("maxSteps", limit).
		Issue("tokens", used.Total).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "the agent was looping; narrow the request, or split it into tasks with checkpoints",
		})
}
