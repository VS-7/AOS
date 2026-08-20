package collection

import (
	"strings"
	"time"
	"unicode"
)

// HookAction is the closed set of normalisations a collection may declare.
//
// The original accepts source-code strings for onCreated/onUpdated/onDeleted
// and writes them into a generated schema.ts — an agent producing executable
// code in the workspace, with no sandbox and no review. These cover the cases
// actually observed there (timestamps and normalisation) without opening that
// door. Anything beyond them is a Routine with an activity trigger, which goes
// through the sandbox and is audited.
type HookAction string

const (
	ActionSetTimestamp HookAction = "setTimestamp"
	ActionSlugify      HookAction = "slugify"
	ActionDefaultTo    HookAction = "defaultTo"
	ActionComputeFrom  HookAction = "computeFrom"
)

// Hook is one declared normalisation.
type Hook struct {
	On     string     `json:"on" jsonschema:"When this hook runs: create or update."` // create | update
	Action HookAction `json:"action" jsonschema:"One of: setTimestamp, slugify, defaultTo, computeFrom."`
	Field  string     `json:"field" jsonschema:"The field this hook writes."`
	From   string     `json:"from,omitempty" jsonschema:"Source field, for slugify and computeFrom."` // slugify, computeFrom
	Value  any        `json:"value,omitempty" jsonschema:"The default value, for defaultTo."`         // defaultTo
}

// ApplyHooks runs the collection's declared hooks over a record. It returns a
// new map rather than mutating: the caller still holds what the agent sent, and
// an error halfway through must not leave a half-normalised record behind.
func ApplyHooks(c Collection, data map[string]any, now time.Time, op string) (map[string]any, error) {
	out := make(map[string]any, len(data)+len(c.Hooks))
	for k, v := range data {
		out[k] = v
	}
	for _, h := range c.Hooks {
		if h.On != "" && h.On != op {
			continue
		}
		switch h.Action {
		case ActionSetTimestamp:
			out[h.Field] = now.UTC().Format(time.RFC3339)
		case ActionSlugify:
			source, _ := out[h.From].(string)
			out[h.Field] = slugify(source)
		case ActionDefaultTo:
			if existing, ok := out[h.Field]; !ok || existing == nil || existing == "" {
				out[h.Field] = h.Value
			}
		case ActionComputeFrom:
			if v, ok := out[h.From]; ok {
				out[h.Field] = v
			}
		default:
			return nil, errHookUnknown(c.ID, string(h.Action))
		}
	}
	return out, nil
}

// slugify is deliberately small: lowercase, letters and digits kept, everything
// else collapsed to a single hyphen. A slug is a path segment, and a path
// segment with a surprise in it is a path traversal waiting to be found.
func slugify(s string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteRune('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}
