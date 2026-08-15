package apperr

import (
	"errors"
	"net/http"
)

// The behavioural sentinels. Codes are strings for the consumer; internally,
// callers branch on behaviour with errors.Is instead of parsing codes.
//
// The specification lists four (NotFound, Conflict, Forbidden, Unavailable).
// Three more exist here because the invariant "every *Error unwraps to exactly
// one sentinel" cannot hold otherwise: a 400, a 401 and a 500 have no sentinel
// among the four. See the decision recorded in "Estratégia de Erros".
var (
	ErrInvalid      = errors.New("invalid")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnavailable  = errors.New("unavailable")
	ErrInternal     = errors.New("internal")
)

// Sentinels lists every sentinel, in the order a report should present them.
// The catalog test uses it to assert that each error resolves to exactly one.
var Sentinels = []error{
	ErrInvalid, ErrUnauthorized, ErrForbidden,
	ErrNotFound, ErrConflict, ErrUnavailable, ErrInternal,
}

func sentinelForStatus(status int) error {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrInvalid
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict, http.StatusPreconditionFailed:
		return ErrConflict
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusTooManyRequests:
		return ErrUnavailable
	default:
		return ErrInternal
	}
}
