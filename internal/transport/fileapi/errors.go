package fileapi

import "github.com/OWNER/aos/internal/core/apperr"

func errBodyTooLarge(limit int) error {
	return apperr.New("FILE_HTTP_BODY_TOO_LARGE").
		Causer("fileapi.decode").
		Msgf("the request body is larger than %d bytes", limit).
		Issue("limit", limit).
		Status(apperr.StatusPayloadTooLarge).
		CTA(apperr.CallToAction{Label: "send a smaller body — this endpoint carries file content, not bulk uploads"})
}

func errBadRequestBody(cause error) error {
	return apperr.New("FILE_HTTP_BAD_BODY").
		Causer("fileapi.decode").
		Msgf("the request body is not valid JSON").
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "send a JSON object matching this endpoint's fields"})
}
