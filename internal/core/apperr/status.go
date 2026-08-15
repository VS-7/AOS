package apperr

import "net/http"

// The HTTP statuses an error may carry, re-exported here so that domain code
// never imports net/http.
//
// "Estratégia de Erros" writes Status(http.StatusNotFound) inside a domain
// errors.go, but "Hexagonal e Regra de Dependência" forbids net/http anywhere
// under internal/domain, and that rule is the one declared inviolable. Domain
// code therefore writes Status(apperr.StatusNotFound); the value is identical.
const (
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusForbidden           = http.StatusForbidden
	StatusNotFound            = http.StatusNotFound
	StatusMethodNotAllowed    = http.StatusMethodNotAllowed
	StatusPayloadTooLarge     = http.StatusRequestEntityTooLarge
	StatusConflict            = http.StatusConflict
	StatusGone                = http.StatusGone
	StatusPreconditionFailed  = http.StatusPreconditionFailed
	StatusUnprocessableEntity = http.StatusUnprocessableEntity
	StatusTooManyRequests     = http.StatusTooManyRequests
	StatusInternalServerError = http.StatusInternalServerError
	StatusNotImplemented      = http.StatusNotImplemented
	StatusBadGateway          = http.StatusBadGateway
	StatusServiceUnavailable  = http.StatusServiceUnavailable
	StatusGatewayTimeout      = http.StatusGatewayTimeout
)
