package testx

import (
	"bytes"
	"encoding/json"
)

// MarshalStable serialises with sorted map keys and fixed indentation, so a
// golden never fails because Go randomised map iteration order.
//
// encoding/json already sorts map keys; the guarantee is restated here because
// every golden depends on it and because the helper also fixes indentation and
// disables HTML escaping, which would otherwise turn a "<" in a prompt into
// "<" and make the file unreadable in review.
func MarshalStable(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
