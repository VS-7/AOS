package template

import (
	"context"
	"strings"
	"time"
)

const (
	// renderTimeout bounds one Render call. The original applies no such
	// limit; a template with a pathological loop must not be able to hang
	// the daemon that renders it (docs/04 - Domínio/Template (Go).md).
	renderTimeout = 5 * time.Second

	// maxOutputBytes bounds a render's output. Checked after the engine
	// returns, the same way the design doc's own sketch does — this does not
	// bound memory used while a huge string is being built, only refuses to
	// hand one back.
	maxOutputBytes = 4 << 20 // 4 MB
)

// validateVariables checks in against the contract declared — every Required
// entry with no Default must have a value, and every entry that is missing
// but has a Default is filled from it. It never checks Type: Variable.Type is
// descriptive metadata for a caller inspecting the contract, not enforced
// here (see entity.go's own doc comment on Variable.Type).
func validateVariables(id string, declared []Variable, given map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(given)+len(declared))
	for k, v := range given {
		out[k] = v
	}
	for _, v := range declared {
		if _, ok := out[v.Name]; ok {
			continue
		}
		if v.Default != nil {
			out[v.Name] = v.Default
			continue
		}
		if v.Required {
			return nil, errVariableMissing(id, v.Name)
		}
	}
	return out, nil
}

// render runs t.Content through the engine, bounded by renderTimeout and
// maxOutputBytes.
//
// This is the one place in this system where user-authored Liquid actually
// executes (ADR-0014). vars is exactly what validateVariables produced —
// never config, never the process environment, never anything this function
// adds on its own — so a template body that reads {{ config.security.secret
// }} finds nothing, not a leaked value.
//
// osteele/liquid's Render takes no context, so a pathological loop cannot
// actually be interrupted mid-render — this runs it on a goroutine and stops
// waiting at the timeout instead. An abandoned render keeps running until it
// finishes or the process exits; it is not killed. That is an honest
// limitation of the engine, not a claim this function does not make.
func render(ctx context.Context, eng Engine, id, source string, vars map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()

	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := eng.Render(source, vars)
		done <- result{out: out, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", errRenderTimeout(id)
	case r := <-done:
		if r.err != nil {
			return "", errRenderFailed(id, r.err)
		}
		if len(r.out) > maxOutputBytes {
			return "", errOutputTooLarge(id, len(r.out))
		}
		return r.out, nil
	}
}

// hasDelimiters reports whether source contains a Liquid object or tag
// delimiter — the same "no placeholders, no parse" check ADR-0014 records
// from the original, used here only to decide whether Validate is worth
// running on an obviously-plain body, not to skip rendering (rendering is
// this domain's entire purpose, unlike the prompt path ADR-0014 also
// describes).
func hasDelimiters(source string) bool {
	return strings.Contains(source, "{{") || strings.Contains(source, "{%")
}
