package authapi

import "github.com/OWNER/aos/internal/core/apperr"

func errUnauthenticated() error {
	return apperr.New("AUTH_HTTP_UNAUTHENTICATED").
		Causer("authapi").
		Msgf("this request carries no valid credential").
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{Label: "log in first"})
}

func errBodyTooLarge(limit int) error {
	return apperr.New("AUTH_HTTP_BODY_TOO_LARGE").
		Causer("authapi.decode").
		Msgf("the request body is larger than %d bytes", limit).
		Issue("limit", limit).
		Status(apperr.StatusPayloadTooLarge).
		CTA(apperr.CallToAction{Label: "this endpoint carries a handful of short fields, not a bulk upload"})
}

func errBadRequestBody(cause error) error {
	return apperr.New("AUTH_HTTP_BAD_BODY").
		Causer("authapi.decode").
		Msgf("the request body is not valid JSON").
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "send a JSON object matching this endpoint's fields"})
}

func errNotJSON() error {
	return apperr.New("AUTH_HTTP_NOT_JSON").
		Causer("authapi.regenerateAPIToken").
		Msgf("this request must be sent with Content-Type: application/json").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "send the request from the application, or with a JSON body and that header"})
}

func errAPITokenReplacingItself() error {
	return apperr.New("AUTH_HTTP_API_TOKEN_CANNOT_REPLACE_ITSELF").
		Causer("authapi.regenerateAPIToken").
		Msgf("the API token cannot be used to replace the API token").
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "sign in to the application and regenerate it from Settings > Developers"})
}
