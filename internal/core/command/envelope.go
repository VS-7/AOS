package command

import (
	"encoding/json"

	"github.com/OWNER/aos/internal/core/apperr"
)

// Envelope wraps every command result.
//
// A call to action on success is guidance, not error handling: after creating a
// task, suggest starting it. The original carries the same idea as
// ResponseWithCTA<T>, and it is one of the reasons its agent chains operations
// without being told how.
type Envelope struct {
	Data       any                   `json:"data"`
	CTA        []apperr.CallToAction `json:"_cta,omitempty"`
	Deprecated *DeprecationNotice    `json:"_deprecated,omitempty"`
}

// WithCTA is implemented by a result that suggests what to do next.
type WithCTA interface {
	CallsToAction() []apperr.CallToAction
}

// Wrap builds the envelope of a successful call.
func Wrap(out any, notice *DeprecationNotice) Envelope {
	e := Envelope{Data: out, Deprecated: notice}
	if c, ok := out.(WithCTA); ok {
		e.CTA = c.CallsToAction()
	}
	return e
}

// MarshalEnvelope renders a result the way every surface reports it.
func MarshalEnvelope(out any, notice *DeprecationNotice) ([]byte, error) {
	return json.Marshal(Wrap(out, notice))
}
