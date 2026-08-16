package oklch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/oklch"
)

// TestTheAnchorsOfTheColourSpace. These are the values CSS Color 4 specifies,
// so a conversion that agrees with them agrees with the browser — which is the
// only agreement that matters here.
func TestTheAnchorsOfTheColourSpace(t *testing.T) {
	cases := []struct {
		hex  string
		l, c float64
	}{
		{"#ffffff", 1.0, 0.0},
		{"#000000", 0.0, 0.0},
		{"#808080", 0.5999, 0.0},
	}
	for _, tc := range cases {
		got, err := oklch.ParseHex(tc.hex)
		if err != nil {
			t.Fatalf("%s: %v", tc.hex, err)
		}
		if math.Abs(got.L-tc.l) > 0.01 {
			t.Errorf("%s: L = %.4f, want %.4f", tc.hex, got.L, tc.l)
		}
		if math.Abs(got.C-tc.c) > 0.01 {
			t.Errorf("%s: C = %.4f, want %.4f", tc.hex, got.C, tc.c)
		}
	}
}

// TestAColourSurvivesTheRoundTrip. The conversion back is what the contrast
// measurement runs on, so a lossy one would quietly mis-rank every theme.
func TestAColourSurvivesTheRoundTrip(t *testing.T) {
	for _, hex := range []string{"#2e3440", "#e5e9f0", "#88c0d0", "#bf616a", "#0d0b08", "#ffffff"} {
		original, err := oklch.ParseHex(hex)
		if err != nil {
			t.Fatal(err)
		}
		r, g, b := original.SRGB()
		back := oklch.FromSRGB(r, g, b)

		if math.Abs(back.L-original.L) > 0.002 {
			t.Errorf("%s: L drifted from %.4f to %.4f", hex, original.L, back.L)
		}
		if math.Abs(back.C-original.C) > 0.002 {
			t.Errorf("%s: C drifted from %.4f to %.4f", hex, original.C, back.C)
		}
	}
}

// TestTheShortFormsAndTheAlphaForm.
func TestTheShortFormsAndTheAlphaForm(t *testing.T) {
	short, err := oklch.ParseHex("#fff")
	if err != nil {
		t.Fatal(err)
	}
	long, err := oklch.ParseHex("#ffffff")
	if err != nil {
		t.Fatal(err)
	}
	if short != long {
		t.Fatalf("#fff = %+v, #ffffff = %+v", short, long)
	}

	// The alpha channel is read and discarded: a theme's transparency belongs
	// to the token that uses the colour, not to the colour.
	withAlpha, err := oklch.ParseHex("#ffffff80")
	if err != nil {
		t.Fatal(err)
	}
	if withAlpha != long {
		t.Fatalf("the alpha channel changed the colour: %+v", withAlpha)
	}

	// Whitespace and a missing hash are both what a hand-written theme file
	// looks like.
	if _, err := oklch.ParseHex("  2e3440 "); err != nil {
		t.Fatalf("a bare hex value was refused: %v", err)
	}
	for _, bad := range []string{"", "#", "#12", "#12345", "midnight", "#gggggg"} {
		if _, err := oklch.ParseHex(bad); err == nil {
			t.Errorf("%q parsed as a colour", bad)
		}
	}
}

// TestHueIsMixedTheShortWayRound. The midpoint of 350° and 10° is 0°, not 180°,
// and getting this wrong turns a subtle blend into a complementary colour.
func TestHueIsMixedTheShortWayRound(t *testing.T) {
	a := oklch.Color{L: 0.5, C: 0.1, H: 350}
	b := oklch.Color{L: 0.5, C: 0.1, H: 10}

	mixed := oklch.Mix(a, b, 1, 1)
	// The answer is 0°, which is also 360°.
	if !(mixed.H < 1 || mixed.H > 359) {
		t.Fatalf("hue = %.2f, want 0", mixed.H)
	}
	if math.Abs(mixed.L-0.5) > 1e-9 || math.Abs(mixed.C-0.1) > 1e-9 {
		t.Fatalf("mix = %+v", mixed)
	}
}

// TestWeightsDecideTheMix, and a mix of nothing is the midpoint rather than a
// division by zero.
func TestWeightsDecideTheMix(t *testing.T) {
	dark := oklch.Color{L: 0.1, C: 0, H: 0}
	light := oklch.Color{L: 0.9, C: 0, H: 0}

	if got := oklch.Mix(dark, light, 9, 1); math.Abs(got.L-0.18) > 1e-9 {
		t.Fatalf("L = %.4f, want 0.18", got.L)
	}
	if got := oklch.Mix(dark, light, 0, 0); math.Abs(got.L-0.5) > 1e-9 {
		t.Fatalf("a mix with no weight at all gave L = %.4f, want the midpoint", got.L)
	}
}

// TestContrastMatchesTheWCAGAnchors. Black on white is 21:1 by definition, and
// a colour against itself is 1:1.
func TestContrastMatchesTheWCAGAnchors(t *testing.T) {
	white, _ := oklch.ParseHex("#ffffff")
	black, _ := oklch.ParseHex("#000000")

	if got := oklch.Contrast(white, black); math.Abs(got-21) > 0.05 {
		t.Fatalf("black on white = %.2f:1, want 21", got)
	}
	if got := oklch.Contrast(black, white); math.Abs(got-21) > 0.05 {
		t.Fatalf("the ratio is not symmetric: %.2f", got)
	}
	if got := oklch.Contrast(white, white); math.Abs(got-1) > 1e-9 {
		t.Fatalf("a colour against itself = %.4f, want 1", got)
	}

	// A known failing pair, so the threshold means something.
	grey, _ := oklch.ParseHex("#777777")
	if got := oklch.Contrast(white, grey); got >= oklch.ContrastAA {
		t.Fatalf("mid grey on white is %.2f:1 and passed the AA floor", got)
	}
}

// TestTheCSSFormIsWhatABrowserParses.
func TestTheCSSFormIsWhatABrowserParses(t *testing.T) {
	c := oklch.Color{L: 0.5, C: 0.12345, H: 210.456}

	plain := c.CSS()
	if plain != "oklch(0.500 0.1235 210.46)" {
		t.Fatalf("CSS = %q", plain)
	}
	if !strings.HasPrefix(plain, "oklch(") || !strings.HasSuffix(plain, ")") {
		t.Fatalf("CSS = %q", plain)
	}

	alpha := c.CSSAlpha(0.75)
	if alpha != "oklch(0.500 0.1235 210.46 / 0.75)" {
		t.Fatalf("CSSAlpha = %q", alpha)
	}
}

// TestAColourOutsideTheGamutIsClampedRatherThanWrapped. Wrapping would turn an
// over-saturated accent into an unrelated colour, and this is only ever used to
// measure contrast against a display that clamps anyway.
func TestAColourOutsideTheGamutIsClampedRatherThanWrapped(t *testing.T) {
	wild := oklch.Color{L: 0.7, C: 0.9, H: 30}
	r, g, b := wild.SRGB()
	for name, v := range map[string]float64{"r": r, "g": g, "b": b} {
		if v < 0 || v > 1 {
			t.Fatalf("%s = %.4f, outside 0–1", name, v)
		}
	}
	if l := wild.Luminance(); l < 0 || l > 1 {
		t.Fatalf("luminance = %.4f", l)
	}
}
