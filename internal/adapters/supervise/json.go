package supervise

import "encoding/json"

// The record is JSON. These two wrappers exist so the file above reads without
// an encoding/json import competing for attention with the process handling.

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }

func jsonMarshalIndent(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
