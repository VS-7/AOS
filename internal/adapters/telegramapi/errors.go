package telegramapi

import "github.com/OWNER/aos/internal/core/apperr"

func errWebhookSecretMismatch() error {
	return apperr.New("TELEGRAMAPI_WEBHOOK_SECRET_MISMATCH").
		Causer("telegramapi.Provider.Parse").
		Msgf("webhook request carried the wrong secret token").
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{Label: "this request did not come from the registered webhook; if it should have, re-register the channel"})
}

func errUnsupportedUpdate() error {
	return apperr.New("TELEGRAMAPI_UPDATE_UNSUPPORTED").
		Causer("telegramapi.Provider.Parse").
		Msgf("update carried no message this build understands").
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{Label: "only plain text messages are handled today"})
}

func errDecodeFailed(cause error) error {
	return apperr.New("TELEGRAMAPI_DECODE_FAILED").
		Causer("telegramapi.Provider.Parse").
		Msgf("could not decode the update: %v", cause).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the request body was not the JSON Telegram documents for an Update"})
}

func errEncodeFailed(method string, cause error) error {
	return apperr.New("TELEGRAMAPI_ENCODE_FAILED").
		Causer("telegramapi.Provider.call").
		Msgf("could not encode the request for %s: %v", method, cause).
		Issue("method", method).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

func errRequestFailed(method string, cause error) error {
	return apperr.New("TELEGRAMAPI_REQUEST_FAILED").
		Causer("telegramapi.Provider.call").
		Msgf("could not reach Telegram for %s: %v", method, cause).
		Issue("method", method).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check network connectivity to api.telegram.org"})
}

func errAPIFailed(method string, status int, description string) error {
	return apperr.New("TELEGRAMAPI_API_FAILED").
		Causer("telegramapi.Provider.call").
		Msgf("telegram refused %s (%d): %s", method, status, description).
		Issue("method", method).
		Issue("description", description).
		Status(apperr.StatusBadGateway).
		CTA(apperr.CallToAction{Label: "check the bot token and that the request is well-formed"})
}

func errSendPartial(sent, total int, cause error) error {
	return apperr.New("TELEGRAMAPI_SEND_PARTIAL").
		Causer("telegramapi.Provider.Send").
		Msgf("sent %d of %d chunks before failing: %v", sent, total, cause).
		Issue("sent", sent).
		Issue("total", total).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the recipient saw a truncated message; resending will duplicate what already arrived"})
}
