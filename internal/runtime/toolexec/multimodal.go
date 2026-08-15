package toolexec

import (
	"strings"
)

// FilePart is content that must reach the model untouched: an image the
// provider will render, audio it will transcribe, a document it will read.
//
// It is a named type rather than a heuristic over maps because the difference
// between "a picture the agent produced on purpose" and "a base64 blob that
// leaked into a tool result" cannot be recovered from the bytes, and guessing
// wrong in one direction breaks images while guessing wrong in the other burns
// the context window.
type FilePart struct {
	MediaType string `json:"mediaType"`
	Data      string `json:"data,omitempty"` // base64, or a data: URL
	URI       string `json:"uri,omitempty"`
	Name      string `json:"name,omitempty"`
}

// passthrough reports whether a value must reach the model as it is.
func passthrough(v any) (any, bool) {
	switch t := v.(type) {
	case FilePart:
		return t, true
	case *FilePart:
		if t == nil {
			return nil, false
		}
		return *t, true
	case []FilePart:
		return t, true
	}
	return nil, false
}

// dataURLPrefix is what an inline image looks like at the start.
const dataURLPrefix = "data:"

// TrimInlineBlob shortens something that looks like an accidental blob.
//
// It is applied to the text form before truncation, so a tool that returns a
// JSON document with a base64 field does not spend the whole visible budget on
// it. Content declared as a FilePart never reaches here.
func TrimInlineBlob(s string) string {
	if len(s) <= MaxBase64Len {
		return s
	}
	if !strings.HasPrefix(s, dataURLPrefix) && !looksBase64(s) {
		return s
	}
	return s[:MaxBase64Len] + "…[" + itoa(len(s)-MaxBase64Len) + " more characters of encoded data omitted]"
}

// looksBase64 is deliberately conservative: a long run of nothing but the
// base64 alphabet. Prose does not look like this, and a false positive would
// truncate a legitimate answer.
func looksBase64(s string) bool {
	if len(s) < MaxBase64Len {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '+', c == '/', c == '=', c == '\n', c == '\r':
		default:
			return false
		}
	}
	return true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}
