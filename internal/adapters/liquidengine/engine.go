// Package liquidengine adapts github.com/osteele/liquid to
// internal/domain/template.Engine — the "Renderer of two functions" ADR-0014
// asks for, kept a thin wrapper so the third-party dependency stays
// substitutable behind that one small port.
package liquidengine

import "github.com/osteele/liquid"

// Engine renders Liquid source through a single shared
// github.com/osteele/liquid.Engine — safe for concurrent use, the same way
// the library's own zero-config Engine is documented to be, since it holds no
// per-render state between calls.
type Engine struct {
	eng *liquid.Engine
}

// New builds a Liquid engine with the library's default configuration —
// exactly what the original's LiquidJS syntax needs, no template store, no
// custom filters or tags registered.
func New() *Engine {
	return &Engine{eng: liquid.NewEngine()}
}

// Validate parses source without rendering it, so a caller can refuse a
// syntactically broken template before it is ever persisted.
func (e *Engine) Validate(source string) error {
	_, err := e.eng.ParseString(source)
	if err != nil {
		return err
	}
	return nil
}

// Render parses and renders source against vars. It applies no timeout or
// output cap of its own — internal/domain/template.service.Render is what
// bounds a call to this method, so the same Engine can be reused by a caller
// that wants no such bound.
func (e *Engine) Render(source string, vars map[string]any) (string, error) {
	out, err := e.eng.ParseAndRenderString(source, liquid.Bindings(vars))
	if err != nil {
		return "", err
	}
	return out, nil
}
