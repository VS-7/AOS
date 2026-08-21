package bot

import "github.com/OWNER/aos/internal/core/apperr"

// errEnvMissing fires when ${env.VAR}, in a channel's token, names a
// variable that is not set — the same idiom internal/domain/toolset's own
// errEnvMissing uses, duplicated for the reason EnvResolver's doc explains.
func errEnvMissing(name string) error {
	return apperr.New("BOT_ENV_MISSING").
		Causer("bot.interpolate").
		Msgf("the environment variable %q, referenced by ${env.%s}, is not set", name, name).
		Issue("variable", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "set the variable in the workspace environment before this channel can register"})
}

// errProviderNotAvailable fires when an agent's channel names a provider
// this build has no Provider for.
func errProviderNotAvailable(provider string) error {
	return apperr.New("BOT_PROVIDER_NOT_AVAILABLE").
		Causer("bot.Registry").
		Msgf("no provider registered for channel type %q", provider).
		Issue("provider", provider).
		Status(apperr.StatusNotImplemented).
		CTA(apperr.CallToAction{Label: "telegram is the only channel provider available in this build"})
}

// errRegistrationNotFound fires when an inbound webhook arrives for a
// provider/agent pair that was never registered — the webhook route was
// reached, but nothing here claims it.
func errRegistrationNotFound(provider, agentID string) error {
	return apperr.New("BOT_REGISTRATION_NOT_FOUND").
		Causer("bot.Registry").
		Msgf("no %s registration for agent %q", provider, agentID).
		Issue("provider", provider).
		Issue("agent", agentID).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "the agent's channels field may have changed since the webhook was registered"})
}

// errSendFailed wraps a Provider.Send failure.
func errSendFailed(provider, chatID string, cause error) error {
	return apperr.New("BOT_SEND_FAILED").
		Causer("bot.Registry.Send").
		Msgf("could not send to %s chat %q: %v", provider, chatID, cause).
		Issue("provider", provider).
		Issue("chatId", chatID).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the channel's token and that the provider is reachable"})
}

// errRateLimited fires when a chat's outbound rate limit is exhausted — an
// addition over the original, per the design doc's own decision, so one
// verbose agent cannot get the whole channel throttled by the provider.
func errRateLimited(chatID string) error {
	return apperr.New("BOT_RATE_LIMITED").
		Causer("bot.Registry.Send").
		Msgf("chat %q is sending too fast", chatID).
		Issue("chatId", chatID).
		Status(apperr.StatusTooManyRequests).
		CTA(apperr.CallToAction{Label: "wait before sending again"})
}
