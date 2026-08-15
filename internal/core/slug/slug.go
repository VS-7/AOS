// Package slug turns a human name into the identifier that names a file.
//
// It exists as its own package because three unrelated aggregates derive their
// identity the same way — a workspace from its name, an agent from its name, a
// routine from its title — and two implementations of "the same" slug rule
// would eventually disagree about a single accented character, which is enough
// to make one record unreachable from the other's key.
package slug

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Generate produces the URL- and filename-safe form of text.
//
// The rule is the original's, step for step: decompose to NFD so that an accent
// becomes a separate combining mark, drop the marks, lowercase, collapse
// whitespace into a hyphen, drop everything that is not a word character or a
// hyphen, then collapse and trim the hyphens.
//
//	Generate("Hello World!")  == "hello-world"
//	Generate("Café au Lait")  == "cafe-au-lait"
//	Generate("  --  ")        == ""
func Generate(text string) string {
	if text == "" {
		return ""
	}
	folded, _, err := transform.String(deaccent, text)
	if err != nil {
		// The transformer cannot fail on valid UTF-8; on invalid input, work
		// with what came in rather than losing the identifier entirely.
		folded = text
	}

	var b strings.Builder
	b.Grow(len(folded))
	pendingHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(folded)) {
		switch {
		case unicode.IsSpace(r) || r == '-':
			// Whitespace becomes a hyphen, and runs collapse into one. The
			// hyphen is only written once a keepable rune follows it, which
			// trims the leading and trailing ones without a second pass.
			pendingHyphen = b.Len() > 0
		case isWord(r):
			if pendingHyphen {
				b.WriteByte('-')
				pendingHyphen = false
			}
			b.WriteRune(r)
		default:
			// Dropped: punctuation and symbols. Note that it does not reset
			// pendingHyphen, so "a - b" and "a-b" agree.
		}
	}
	return b.String()
}

// IsValid reports whether s is already in slug form. It is what a validator
// asks before writing, so that a caller who passes an id rather than a name
// gets a rejection instead of a silently rewritten identity.
func IsValid(s string) bool { return s != "" && Generate(s) == s }

// isWord matches the original's \w: letters, digits and the underscore. The
// hyphen is handled separately because it is a separator, not a word character.
func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// deaccent decomposes, drops combining marks, and recomposes.
var deaccent = transform.Chain(
	norm.NFD,
	runes.Remove(runes.In(unicode.Mn)),
	norm.NFC,
)
