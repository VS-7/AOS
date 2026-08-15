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
		CTA(configureModelCTA())
}

// configureModelCTA is the one place that says how to point the installation at
// a model. The slot is a map, so it is set whole rather than by dotted path —
// which is worth spelling out, because the obvious guess does not work.
func configureModelCTA() apperr.CallToAction {
	return apperr.CallToAction{
		Label: "point the default slot at a provider and a model in " + build.StateDir + "/config.json",
		Tool:  "config_update",
		Input: map[string]any{"set": map[string]any{
			"agents.models": map[string]any{
				"default": map[string]any{"provider": "openai", "model": "gpt-5"},
			},
		}},
	}
}

// errProviderNotEnabled is the fifth level of the model cascade: nothing was
// configured anywhere.
func errProviderNotEnabled(model string) error {
	e := apperr.New("AGENT_PROVIDER_NOT_ENABLED").
		Causer("agentloop.Resolve").
		Msgf("no model provider is configured for this agent").
		Status(apperr.StatusBadRequest).
		CTA(configureModelCTA())
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
