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
