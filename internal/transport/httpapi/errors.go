package httpapi

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errUnauthenticated() error {
	return apperr.New("HTTP_UNAUTHENTICATED").
		Causer("httpapi.authenticate").
		Msgf("this request carries no valid credential").
		Status(apperr.StatusUnauthorized).
		CTA(
			// Not `aos auth token issue`, which this named for as long as it
			// existed and which no build has ever had: there is no `auth`
			// group (defect #5). A credential comes from signing in, which
			// writes the one this machine's terminal reads, or from the API
			// token in the configuration.
			apperr.CallToAction{
				Label: "send the token in the Authorization header — a token in the query string is ignored",
			},
			apperr.CallToAction{
				Label: "on this machine, sign in through the application: it writes the credential " +
					build.Name + " reads. For a caller from elsewhere, present the configured API token",
				Command: build.Name + " config get",
				Tool:    "config_get",
			},
		)
}

func errPanic(requestID string) error {
	return apperr.New("HTTP_HANDLER_PANIC").
		Causer("httpapi.recoverer").
		Msgf("the request failed unexpectedly").
		Issue("requestId", requestID).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "report this request id — the stack trace is in the daemon log under it",
		})
}

func errBodyTooLarge(limit int) error {
	return apperr.New("HTTP_BODY_TOO_LARGE").
		Causer("httpapi.invoke").
		Msgf("the request body is larger than %d bytes", limit).
		Issue("limit", limit).
		Status(apperr.StatusPayloadTooLarge).
		CTA(apperr.CallToAction{
			Label: "a command payload is a small JSON object; send a reference rather than the contents",
		})
}

func errNoSuchRoute(method, path string) error {
	return apperr.New("HTTP_NO_SUCH_ROUTE").
		Causer("httpapi.notFound").
		Msgf("%s %s is not a route this daemon serves", method, path).
		Issue("path", path).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the published surface",
			Command: build.Name + " self tools",
		})
}

func errWrongMethod(method, path string) error {
	return apperr.New("HTTP_WRONG_METHOD").
		Causer("httpapi.methodNotAllowed").
		Msgf("%s is not accepted at %s", method, path).
		Issue("path", path).
		Status(apperr.StatusMethodNotAllowed).
		CTA(apperr.CallToAction{
			Label: "every command is a POST carrying its payload as a JSON body",
		})
}

// errNoAuthenticator is what a request gets when the installation asked for
// authentication and this build has nobody to perform it. It is a server
// fault, not the caller's, and it is answered rather than waved through.
func errNoAuthenticator() error {
	return apperr.New("HTTP_NO_AUTHENTICATOR").
		Causer("httpapi.authenticate").
		Msgf("authentication is required here but this daemon has no identity service wired").
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "this is a build or wiring fault; the daemon cannot verify anybody while security is enabled",
		})
}

func errWebhookNoToken() error {
	return apperr.New("HTTP_WEBHOOK_NO_TOKEN").
		Causer("httpapi.routineWebhook").
		Msgf("this request is not authorised to fire that routine").
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label: "send the routine's webhook token as \"Authorization: Bearer <token>\"; it is shown once, when the trigger is created",
			Tool:  "routines_rotate",
		})
}

func errWebhookUnreadable(err error) error {
	return apperr.New("HTTP_WEBHOOK_UNREADABLE").
		Causer("httpapi.routineWebhook").
		Msgf("the webhook's body could not be read").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "deliver it again"}).
		Wrap(err)
}
