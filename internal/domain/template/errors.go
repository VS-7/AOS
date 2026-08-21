package template

import "github.com/OWNER/aos/internal/core/apperr"

// errNotFound fires when no template has this ID.
func errNotFound(id string) error {
	return apperr.New("TEMPLATE_NOT_FOUND").
		Causer("template.Service.Get").
		Msgf("no template %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "list configured templates", Tool: "templates_list"})
}

// errIDRequired fires when Create is asked to write a template with no ID.
func errIDRequired() error {
	return apperr.New("TEMPLATE_ID_REQUIRED").
		Causer("template.Service.Create").
		Msgf("a template needs an id").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "name the template before creating it"})
}

// errContentInvalid fires when Create or Update is given Liquid content that
// does not parse. Refusing here, before anything is written, is the same
// "refuse at write time" rule view.Validate already applies to its own tree —
// a broken template is never the thing an agent discovers by rendering it.
func errContentInvalid(id string, cause error) error {
	return apperr.New("TEMPLATE_CONTENT_INVALID").
		Causer("template.Service.validateContent").
		Msgf("template %q does not parse as Liquid: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "fix the Liquid syntax before saving"})
}

// errVariableMissing fires when Render is called without a value for a
// Variable the template declares Required, and that variable has no Default.
func errVariableMissing(id, name string) error {
	return apperr.New("TEMPLATE_VARIABLE_MISSING").
		Causer("template.Service.Render").
		Msgf("template %q requires the variable %q, which was not given", id, name).
		Issue("id", id).
		Issue("variable", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "inspect the required variables first", Tool: "templates_get"})
}

// errRenderFailed wraps a failure of the Liquid engine itself — a runtime
// error the parser did not catch, such as a filter given the wrong argument
// type.
func errRenderFailed(id string, cause error) error {
	return apperr.New("TEMPLATE_RENDER_FAILED").
		Causer("template.Service.Render").
		Msgf("template %q failed to render: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusUnprocessableEntity).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the variables given against the template's own body"})
}

// errRenderTimeout fires when a render did not finish within renderTimeout —
// the guard against a pathological loop in user-authored Liquid that the
// original does not apply.
func errRenderTimeout(id string) error {
	return apperr.New("TEMPLATE_RENDER_TIMEOUT").
		Causer("template.Service.Render").
		Msgf("template %q did not finish rendering within %s", id, renderTimeout).
		Issue("id", id).
		Issue("timeout", renderTimeout.String()).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{Label: "the template likely has an unbounded loop — check its own body"})
}

// errOutputTooLarge fires when a render's output exceeds maxOutputBytes.
func errOutputTooLarge(id string, n int) error {
	return apperr.New("TEMPLATE_OUTPUT_TOO_LARGE").
		Causer("template.Service.Render").
		Msgf("template %q rendered %d bytes, over the %d byte limit", id, n, maxOutputBytes).
		Issue("id", id).
		Issue("bytes", n).
		Issue("limit", maxOutputBytes).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{Label: "the template's own loops or included data produced too much output — bound them"})
}

// errReadFailed wraps a repository failure that is not "not found" — the
// operation name is dynamic because it fires from several call sites.
func errReadFailed(op string, cause error) error {
	return apperr.New("TEMPLATE_READ_FAILED").
		Causer("template.Service." + op).
		Msgf("could not read templates: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errWriteFailed wraps a repository failure while writing.
func errWriteFailed(op string, cause error) error {
	return apperr.New("TEMPLATE_WRITE_FAILED").
		Causer("template.Service." + op).
		Msgf("could not save the template: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
