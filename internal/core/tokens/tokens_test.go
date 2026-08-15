package tokens_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/tokens"
)

// TestEstimateIsRoughlyFourCharactersPerToken pins the heuristic and, with it,
// the margin. The number is used to slice output and to let an agent budget its
// context; it is documented as approximate and must never be used for billing.
func TestEstimateIsRoughlyFourCharactersPerToken(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
		{strings.Repeat("a", 400), 100},
	}
	for _, c := range cases {
		if got := tokens.Estimate(c.text); got != c.want {
			t.Errorf("Estimate(%d chars) = %d, want %d", len(c.text), got, c.want)
		}
	}
}

// TestWideScriptsCostMore: one CJK character is closer to one token than to a
// quarter of one, and a prompt full of them would otherwise be badly underrated.
func TestWideScriptsCostMore(t *testing.T) {
	latin := tokens.Estimate("aaaaaaaa")
	cjk := tokens.Estimate("日本語のテキスト")
	if cjk <= latin {
		t.Fatalf("cjk = %d, latin = %d — wide scripts must not be underrated", cjk, latin)
	}
}

// TestSliceCutsOnALineBoundary: half a JSON line is not parseable, and an agent
// that receives one wastes a turn discovering that.
func TestSliceCutsOnALineBoundary(t *testing.T) {
	text := "first line here\nsecond line here\nthird line here\n"

	out, more := tokens.Slice(text, 5, 0)
	if !more {
		t.Error("the rest was not announced")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("the cut landed inside a line: %q", out)
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.Contains(text, line) {
			t.Errorf("a line was mangled: %q", line)
		}
	}
}

func TestSliceWithoutABudgetReturnsEverything(t *testing.T) {
	text := "a\nb\nc\n"
	out, more := tokens.Slice(text, 0, 0)
	if out != text || more {
		t.Fatalf("out = %q, more = %v", out, more)
	}
}

func TestOffsetSkipsThePreviousPage(t *testing.T) {
	text := "first line here\nsecond line here\nthird line here\n"
	first, _ := tokens.Slice(text, 5, 0)
	second, _ := tokens.Slice(text, 5, tokens.Estimate(first))
	if second == "" {
		t.Fatal("the second page is empty")
	}
	if strings.Contains(second, "first line") {
		t.Errorf("the offset did not skip the first page: %q", second)
	}
}
