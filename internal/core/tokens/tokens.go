// Package tokens estimates how much context a piece of text will consume.
//
// It lives in core rather than in the CLI formatter because two surfaces need
// it: the CLI pages its output by token budget, and the composite MCP tool
// reports a per-action estimate so the agent can budget its own context before
// calling.
package tokens

import (
	"strings"
	"unicode"
)

// Estimate approximates the token count of a string.
//
// The heuristic is four characters per token for Latin text, with a correction
// for CJK, where a character is closer to one token. It is documented as
// approximate: the error margin is around 10%, which is fine for slicing output
// and must never be used for billing. The original uses a tokeniser without
// declaring a margin at all.
func Estimate(s string) int {
	if s == "" {
		return 0
	}
	var latin, wide int
	for _, r := range s {
		if isWide(r) {
			wide++
			continue
		}
		latin++
	}
	est := wide + (latin+3)/4
	if est == 0 {
		return 1
	}
	return est
}

// isWide reports whether a rune is from a script where one character is roughly
// one token: CJK ideographs, kana and Hangul.
func isWide(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// Slice cuts rendered output at a token budget, on a line boundary, and reports
// whether anything was left out.
//
// Cutting on a line boundary matters: half a JSON line is not parseable, and an
// agent that receives one wastes a turn discovering that.
func Slice(s string, limit, offset int) (out string, more bool) {
	if limit <= 0 && offset <= 0 {
		return s, false
	}
	lines := strings.SplitAfter(s, "\n")

	spent, skipped := 0, 0
	var b strings.Builder
	for _, line := range lines {
		cost := Estimate(line)
		if skipped < offset {
			skipped += cost
			continue
		}
		if limit > 0 && spent+cost > limit && b.Len() > 0 {
			return b.String(), true
		}
		b.WriteString(line)
		spent += cost
		if limit > 0 && spent >= limit {
			// Whether anything remains is decided by what is left after this
			// line, not by the budget alone.
			return b.String(), b.Len() < len(s)-offsetBytes(lines, offset)
		}
	}
	return b.String(), false
}

func offsetBytes(lines []string, offset int) int {
	if offset <= 0 {
		return 0
	}
	spent, n := 0, 0
	for _, line := range lines {
		if spent >= offset {
			break
		}
		spent += Estimate(line)
		n += len(line)
	}
	return n
}
