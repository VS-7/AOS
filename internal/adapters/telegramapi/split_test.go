package telegramapi

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// splitForTest is split at the real ceiling, named so the assertions below
// read as what they check rather than as a call with a constant in it.
func splitForTest(text string) []string { return split(text, maxChars) }

func TestSplitLeavesAShortMessageWhole(t *testing.T) {
	got := split("hello", maxChars)
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("got %v", got)
	}
}

func TestSplitBreaksOnBlockBoundary(t *testing.T) {
	// Two paragraphs, each safely under the limit alone, together over it.
	a := strings.Repeat("a", maxChars-100)
	b := strings.Repeat("b", 200)
	text := a + "\n\n" + b

	got := split(text, maxChars)
	if len(got) != 2 {
		t.Fatalf("got %d chunks, want 2: lens=%v", len(got), lens(got))
	}
	if got[0] != a || got[1] != b {
		t.Fatal("chunks did not split exactly on the blank-line boundary")
	}
}

func TestSplitNeverBreaksInsideAFence(t *testing.T) {
	fence := "```go\n" + strings.Repeat("x = 1\n", 6000) + "```"
	text := "before\n\n" + fence + "\n\nafter"

	got := split(text, maxChars)
	for _, c := range got {
		open := strings.Count(c, "```")
		if open%2 != 0 {
			t.Fatalf("a chunk has an unclosed fence: %d backtick-fences in %.50q...", open, c)
		}
	}
	joined := strings.Join(got, "\n\n")
	if !strings.Contains(joined, fence) {
		t.Fatal("the fenced block was not preserved intact across the split")
	}
}

func TestSplitOfA40000CharacterMessage(t *testing.T) {
	// The design doc's own test case.
	var b strings.Builder
	for b.Len() < 40000 {
		b.WriteString(strings.Repeat("word ", 20))
		b.WriteString("\n\n")
	}
	got := split(b.String(), maxChars)
	if len(got) < 2 {
		t.Fatalf("a 40,000-character message was not split: got %d chunk(s)", len(got))
	}
	for _, c := range got {
		if len(c) > maxChars {
			t.Fatalf("chunk of %d characters exceeds the %d limit", len(c), maxChars)
		}
	}
}

func lens(ss []string) []int {
	out := make([]int, len(ss))
	for i, s := range ss {
		out[i] = len(s)
	}
	return out
}

// The ceiling is the one sendMessage enforces, and it is measured the way
// Telegram measures it. A reply longer than that used to be shipped whole and
// refused by the API, so the person waiting on it got nothing at all.
func TestNoChunkExceedsWhatTheAPIAccepts(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("Uma frase razoavelmente longa em português, com acentuação.\n\n")
	}
	for _, chunk := range splitForTest(b.String()) {
		if n := utf8.RuneCountInString(chunk); n > 4096 {
			t.Fatalf("a chunk of %d characters would be refused by sendMessage", n)
		}
	}

	// Accents do not cost a chunk boundary they should not: the same text in
	// ASCII splits into the same number of pieces.
	accented := strings.Repeat("ção ção ção ção\n\n", 500)
	plain := strings.Repeat("cao cao cao cao\n\n", 500)
	if a, p := len(splitForTest(accented)), len(splitForTest(plain)); a != p {
		t.Errorf("accented text split into %d chunks and plain into %d; the limit is characters, not bytes", a, p)
	}
}
