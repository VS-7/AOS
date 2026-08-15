package prompt

import (
	"encoding/json"
	"strings"

	"github.com/osteele/liquid"

	"github.com/OWNER/aos/internal/core/apperr"
)

// Renderer is the template engine, behind a port.
//
// It is a port and not a direct dependency for one structural reason: ADR-0014
// says the prompt engine and the template domain must be separate packages, and
// a package that imports the engine directly is one refactor away from
// importing the domain that also uses it.
type Renderer interface {
	Render(template string, vars map[string]any) (string, error)
}

// Liquid is the engine the template domain also uses.
type Liquid struct{ engine *liquid.Engine }

// NewLiquid builds the engine.
func NewLiquid() *Liquid { return &Liquid{engine: liquid.NewEngine()} }

// Render evaluates a template against a variable map.
func (l *Liquid) Render(template string, vars map[string]any) (string, error) {
	out, err := l.engine.ParseAndRenderString(template, vars)
	if err != nil {
		return "", err
	}
	return out, nil
}

// renderIfNeeded is the single gate between persisted data and the template
// engine.
//
// Both conditions are required. The opt-in is the security contract: a memory,
// an agent's own instructions and a workspace record are written by somebody —
// sometimes by the agent itself — and a template can read variables. The
// delimiter check is the original's optimisation, kept because it is also a
// reduction of surface: a string that was never meant to be a template never
// meets the parser.
func renderIfNeeded(r Renderer, tpl string, vars map[string]any, allow bool) (string, error) {
	if !allow {
		return tpl, nil
	}
	if !strings.Contains(tpl, "{{") && !strings.Contains(tpl, "{%") {
		return tpl, nil
	}
	if r == nil {
		// No engine and a template that wants one. Returning the text as it is
		// would be the quiet answer; it is also the answer that ships a prompt
		// with `{{ }}` in it and lets nobody notice.
		return "", errNoRenderer()
	}
	return r.Render(tpl, vars)
}

func errNoRenderer() error {
	return apperr.New("PROMPT_NO_RENDERER").
		Causer("prompt.Builder.Build").
		Msgf("a section asked to be rendered and no template engine is installed").
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "install a renderer on the builder, or drop the renderTemplate flag on the section",
		})
}

func errTemplateFailed(section string, cause error) error {
	return apperr.New("PROMPT_TEMPLATE_FAILED").
		Causer("prompt.Builder.Build").
		Msgf("the %s section could not be rendered", section).
		Issue("section", section).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "the template is in the section named in the issue; the engine's message says which tag it choked on",
		})
}

func marshalCompact(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
