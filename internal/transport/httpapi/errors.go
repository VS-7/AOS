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
		CTA(apperr.CallToAction{
			Label:   "send the token in the Authorization header — a token in the query string is ignored",
			Command: build.Name + " auth token issue",
		})
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
