package theme

import (
	"fmt"

	"github.com/OWNER/aos/internal/core/oklch"
)

// Render resolves a theme into the CSS custom properties the document root
// carries.
//
// This is a port of the original's theme provider, mix for mix. The reason it
// lives in Go rather than in the frontend is the token contract: the list of
// properties a theme must produce is checked against what the stylesheet
// consumes, and a check that runs in the same place the values are produced is
// one that cannot drift from them.
func Render(t Theme, want Appearance, native bool) (Rendered, error) {
	palette, resolved, ok := t.Palette(want)
	if !ok {
		return Rendered{}, errNoPalette(t.ID, want)
	}

	surface, err := oklch.ParseHex(palette.Surface)
	if err != nil {
		return Rendered{}, errBadColour(t.ID, "surface", palette.Surface, err)
	}
	ink, err := oklch.ParseHex(palette.Ink)
	if err != nil {
		return Rendered{}, errBadColour(t.ID, "ink", palette.Ink, err)
	}
	accent, err := oklch.ParseHex(palette.Accent)
	if err != nil {
		return Rendered{}, errBadColour(t.ID, "accent", palette.Accent, err)
	}

	// Contrast is inverted on purpose, as in the original: 0 gives a punchy
	// factor of 1.5, 50 is neutral at 1.0, 100 washes the layers out at 0.5.
	factor := 1.5 - float64(palette.Contrast)/100
	if factor <= 0 {
		// A contrast of 150 or more would divide by zero below. The dial is
		// documented as 0–100; anything past it is clamped rather than fatal.
		factor = 0.01
	}

	// Blur only means anything where the operating system draws the window.
	// In a browser there is nothing behind the page to blur, and a translucent
	// background there just reveals white.
	windows := palette.Windows
	if windows == "" {
		windows = Blur
	}
	if !native {
		windows = Solid
	}

	background := surface.CSS()
	sidebar := background
	if windows == Blur {
		background = surface.CSSAlpha(0.75)
		sidebar = "transparent"
	}

	// The weights are the original's, and the shape of them is the point: the
	// surface dominates every layer, and the contrast dial moves how much.
	muted := oklch.Mix(surface, ink, 96*factor, 4/factor)
	card := oklch.Mix(surface, ink, 90*factor, 6/factor)
	secondary := oklch.Mix(surface, accent, 86*factor, 8/factor)
	popover := oklch.Mix(surface, ink, 99*factor, 0)
	accentFill := oklch.Mix(surface, ink, 94*factor, 6/factor)

	// The foreground of the primary colour is black on a dark theme and white
	// on a light one. It is not derived from the accent: an accent light enough
	// to need black text and dark enough to need white is the same accent.
	primaryFg := "oklch(1.000 0.0000 0.00)"
	if resolved == Dark {
		primaryFg = "oklch(0.000 0.0000 0.00)"
	}

	tokens := map[string]string{
		// Base
		"background":         background,
		"foreground":         ink.CSS(),
		"primary":            accent.CSS(),
		"primary-foreground": primaryFg,
		"radius":             palette.Radius.CSS(),

		// Surfaces
		"muted":              muted.CSSAlpha(0.95),
		"muted-foreground":   ink.CSSAlpha(0.55),
		"accent":             accentFill.CSS(),
		"accent-foreground":  ink.CSS(),
		"popover":            popover.CSSAlpha(1),
		"popover-foreground": ink.CSS(),
		"card":               card.CSSAlpha(0.95),
		"card-foreground":    ink.CSS(),

		// Borders and inputs
		"border": ink.CSSAlpha(0.08),
		"input":  ink.CSSAlpha(0.12),
		"ring":   accent.CSS(),

		// Secondary
		"secondary":            secondary.CSSAlpha(0.95),
		"secondary-foreground": ink.CSS(),

		// Destructive is fixed rather than derived, as in the original: a theme
		// whose accent happens to be red must still have a distinguishable
		// danger colour.
		"destructive":            "oklch(0.600 0.2000 25.00)",
		"destructive-foreground": "oklch(0.980 0.0100 25.00)",

		// Sidebar
		"sidebar":                    sidebar,
		"sidebar-foreground":         ink.CSS(),
		"sidebar-primary":            accent.CSS(),
		"sidebar-primary-foreground": primaryFg,
		"sidebar-border":             ink.CSSAlpha(0.1),
		"sidebar-accent":             accentFill.CSS(),
		"sidebar-accent-foreground":  ink.CSS(),
		"sidebar-ring":               accent.CSS(),
	}

	// Charts walk the accent's hue around the wheel, so a five-series chart is
	// legible in every theme without anybody choosing five colours per theme.
	for i, offset := range []float64{0, 160, 30, 200, 90} {
		hue := accent.H + offset
		for hue >= 360 {
			hue -= 360
		}
		tokens[fmt.Sprintf("chart-%d", i+1)] = oklch.Color{L: accent.L, C: accent.C, H: hue}.CSS()
	}

	// The author's semantic colours, and the memory-graph categories derived
	// from the accent. A graph whose category colours were literals would be
	// unreadable in half the themes.
	for name, hex := range palette.Semantic {
		parsed, err := oklch.ParseHex(hex)
		if err != nil {
			return Rendered{}, errBadColour(t.ID, "semantic."+name, hex, err)
		}
		tokens["semantic-"+name] = parsed.CSS()
	}
	for name, colour := range categoryTokens(accent, ink) {
		tokens[name] = colour
	}

	return Rendered{
		Theme: t.ID, Appearance: resolved, Windows: windows, Tokens: tokens,
	}, nil
}

// MemoryCategories are the thirteen the memory graph colours by. They are
// listed here rather than imported so that the theme aggregate does not depend
// on the memory aggregate for a list of names — two features that must agree on
// a vocabulary and not on a type.
var MemoryCategories = []string{
	"decision", "intent", "commitment", "relationship", "event",
	"observation", "error", "learning", "fact", "reference",
	"instruction", "preference", "context",
}

// categoryTokens spreads the thirteen memory categories evenly around the
// accent's hue.
//
// Deriving them rather than fixing them is what keeps the graph readable: a
// fixed palette that looks right on Dracula is mud on Solarized Light. The
// chroma is lifted a little so the nodes separate from the surface, and the
// lightness comes from the ink so they stay legible against it.
func categoryTokens(accent, ink oklch.Color) map[string]string {
	out := make(map[string]string, len(MemoryCategories))
	step := 360.0 / float64(len(MemoryCategories))
	chroma := accent.C
	if chroma < 0.08 {
		// A monochrome accent would make every category the same grey.
		chroma = 0.08
	}
	for i, category := range MemoryCategories {
		hue := accent.H + step*float64(i)
		for hue >= 360 {
			hue -= 360
		}
		lightness := accent.L*0.6 + ink.L*0.4
		out["category-"+category] = oklch.Color{L: lightness, C: chroma, H: hue}.CSS()
	}
	return out
}
