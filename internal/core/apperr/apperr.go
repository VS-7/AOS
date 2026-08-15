// Package apperr defines the single error type of the system.
//
// An error here is not only a diagnostic: it is an instruction for the next
// step of an agent. An error that merely fails wastes an LLM turn; an error
// that carries a call to action saves several. That is why every field of
// Error exists, and why 4xx errors are required to carry at least one CTA.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/OWNER/aos/internal/core/build"
)

// CallToAction is the actionable next step attached to an error or to a
// successful result. Command targets a human on a terminal; Tool and Input
// target an agent, which can execute the suggestion without reformulating it.
type CallToAction struct {
	Label   string `json:"label"`
	Command string `json:"command,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Input   any    `json:"input,omitempty"`
}

// Error is the structured error of the system. It is structured rather than a
// string because the same value is rendered differently on every surface: a
// coloured table on a TTY, JSON for an agent, an HTTP status plus body, a toast
// on the desktop.
//
// The exported field names differ from the JSON keys because the fluent
// builders below own those names (Go forbids a field and a method sharing one).
// The wire format is the contract; the field names are not.
type Error struct {
	Code       string         `json:"code"`            // AOS_MEMORY_NOT_FOUND
	CauserName string         `json:"causer"`          // memory.Service.Get
	Message    string         `json:"message"`         //
	Issues     map[string]any `json:"issue,omitempty"` // structured data about the problem
	Actions    []CallToAction `json:"cta,omitempty"`
	HTTPStatus int            `json:"-"` // HTTP status
	Cause      error          `json:"-"` // wrapped cause

	kind error // sentinel; derived from HTTPStatus unless set explicitly
}

// New starts a new error. The code is given without the product prefix, which
// New applies from build.ErrorPrefix — an already prefixed code is accepted and
// left alone so a code parsed back from the wire round-trips.
func New(code string) *Error {
	return &Error{Code: qualify(code), HTTPStatus: http.StatusInternalServerError}
}

func qualify(code string) string {
	prefix := build.ErrorPrefix + "_"
	if strings.HasPrefix(code, prefix) {
		return code
	}
	return prefix + code
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// Unwrap exposes both the wrapped cause and the behavioural sentinel, so that
// errors.Is(err, apperr.ErrNotFound) works regardless of what the error wraps.
func (e *Error) Unwrap() []error {
	out := make([]error, 0, 2)
	if e.Cause != nil {
		out = append(out, e.Cause)
	}
	if k := e.Sentinel(); k != nil {
		out = append(out, k)
	}
	return out
}

// Sentinel returns the behavioural sentinel of this error: the one set
// explicitly, or the one implied by the HTTP status.
func (e *Error) Sentinel() error {
	if e.kind != nil {
		return e.kind
	}
	return sentinelForStatus(e.HTTPStatus)
}

// Causer records the domain operation that produced the error. It stays manual
// on purpose: the value is semantic ("which domain operation failed"), not
// positional — runtime.Caller would report the wrapper, not the intent.
func (e *Error) Causer(v string) *Error { e.CauserName = v; return e }

// Msgf sets the human-readable message.
func (e *Error) Msgf(format string, args ...any) *Error {
	e.Message = fmt.Sprintf(format, args...)
	return e
}

// Issue adds one structured key/value describing the problem. Nothing tagged
// secret:"true" may ever reach here — see ADR-0010 and TestNoSecretsInIssue.
func (e *Error) Issue(k string, v any) *Error {
	if e.Issues == nil {
		e.Issues = map[string]any{}
	}
	e.Issues[k] = v
	return e
}

// Status sets the HTTP status and, with it, the default sentinel.
func (e *Error) Status(code int) *Error { e.HTTPStatus = code; return e }

// CTA appends one or more calls to action.
func (e *Error) CTA(c ...CallToAction) *Error { e.Actions = append(e.Actions, c...); return e }

// Wrap records the underlying cause.
func (e *Error) Wrap(err error) *Error { e.Cause = err; return e }

// Kind overrides the sentinel that the HTTP status would imply.
func (e *Error) Kind(sentinel error) *Error { e.kind = sentinel; return e }

// As extracts an *Error from any error chain.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// StatusOf returns the HTTP status an error should be reported with. An error
// that is not an *Error is an internal failure by definition: it escaped the
// domain without being classified.
func StatusOf(err error) int {
	if e, ok := As(err); ok && e.HTTPStatus != 0 {
		return e.HTTPStatus
	}
	return http.StatusInternalServerError
}
