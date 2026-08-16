// Package theme is the appearance of the application: the built-in presets,
// the ones a user installs, and the tokens every component reads.
//
// A theme here is four colours and a dial, not a table of a hundred values.
// The surfaces, borders and states are derived from surface, ink, accent and
// contrast — which is how the original does it, and why thirty-eight themes are
// thirty-eight short files rather than thirty-eight spreadsheets.
package theme

import (
	"sort"
	"strings"
)

// Appearance is what a palette is for. It drives the native window material,
// not only the CSS.
type Appearance string

const (
	Light Appearance = "light"
	Dark  Appearance = "dark"

	// Auto follows the operating system. A theme is auto when it carries both
	// palettes, which every built-in one does.
	Auto Appearance = "auto"
)

// Valid reports whether a is one of the three.
func (a Appearance) Valid() bool { return a == Light || a == Dark || a == Auto }

// Radius is the corner rounding a theme asks for.
type Radius string

// The four rounding steps, with the values the original maps them to.
var radiusValues = map[Radius]string{
	"none": "0rem",
	"sm":   "0.375rem",
	"md":   "0.75rem",
	"lg":   "1rem",
}

// CSS returns the length this rounding renders as.
func (r Radius) CSS() string {
	if v, ok := radiusValues[r]; ok {
		return v
	}
	return radiusValues["lg"]
}

// Windows is how the window behind the interface is drawn: a solid colour, or
// the operating system's blur behind a translucent background.
type Windows string

const (
	Solid Windows = "solid"
	Blur  Windows = "blur"
)

// Palette is one appearance of a theme, as its author wrote it.
type Palette struct {
	Surface  string `yaml:"surface" json:"surface" jsonschema:"Background colour of the application, as a hex value."`
	Ink      string `yaml:"ink" json:"ink" jsonschema:"Colour of text on the surface, as a hex value."`
	Accent   string `yaml:"accent" json:"accent" jsonschema:"Primary accent, as a hex value."`
	Contrast int    `yaml:"contrast" json:"contrast" jsonschema:"How strongly derived layers separate from the surface, 0 to 100. 50 is neutral."`

	Radius  Radius  `yaml:"radius,omitempty" json:"radius,omitempty" jsonschema:"Corner rounding: none, sm, md or lg."`
	Windows Windows `yaml:"windows,omitempty" json:"windows,omitempty" jsonschema:"solid or blur. Blur asks the operating system for a translucent window."`

	// Semantic are the colours the author chose for meaning rather than for
	// hierarchy: success, error, and the two diff colours.
	Semantic map[string]string `yaml:"semantic,omitempty" json:"semantic,omitempty" jsonschema:"Colours by meaning: accent, success, warning, error, info, diffAdded, diffRemoved."`
}

// Theme is one appearance preset.
type Theme struct {
	ID   string `yaml:"id" json:"id" jsonschema:"Identifier of the theme."`
	Name string `yaml:"name" json:"name" jsonschema:"Human-readable name."`

	Author  string `yaml:"author,omitempty" json:"author,omitempty" jsonschema:"Who made it."`
	Builtin bool   `yaml:"-" json:"builtin" jsonschema:"Whether it ships with the application."`

	// Variants holds the palettes, keyed by appearance. Every built-in theme
	// carries both, which is what makes following the system possible: a theme
	// with one palette cannot honour a preference it has no colours for.
	Variants map[Appearance]Palette `yaml:"variants" json:"variants" jsonschema:"Palettes by appearance: light and dark."`
}

// Appearance reports what this theme can be.
func (t Theme) Appearance() Appearance {
	_, hasLight := t.Variants[Light]
	_, hasDark := t.Variants[Dark]
	switch {
	case hasLight && hasDark:
		return Auto
	case hasLight:
		return Light
	default:
		return Dark
	}
}

// Palette returns the palette for an appearance, resolving auto and falling
// back to whichever the theme has.
func (t Theme) Palette(a Appearance) (Palette, Appearance, bool) {
	if a == Auto || a == "" {
		a = Dark
	}
	if p, ok := t.Variants[a]; ok {
		return p, a, true
	}
	other := Dark
	if a == Dark {
		other = Light
	}
	if p, ok := t.Variants[other]; ok {
		return p, other, true
	}
	return Palette{}, "", false
}

// Appearances lists what this theme offers, in a stable order.
func (t Theme) Appearances() []Appearance {
	var out []Appearance
	for _, a := range []Appearance{Dark, Light} {
		if _, ok := t.Variants[a]; ok {
			out = append(out, a)
		}
	}
	return out
}

// Rendered is a theme resolved to the CSS custom properties the frontend sets
// on the document root.
type Rendered struct {
	Theme      string            `json:"theme" jsonschema:"Identifier of the theme this came from."`
	Appearance Appearance        `json:"appearance" jsonschema:"Which palette was rendered."`
	Windows    Windows           `json:"windows" jsonschema:"solid or blur, for the native window material."`
	Tokens     map[string]string `json:"tokens" jsonschema:"CSS custom property name to value, without the leading dashes."`
}

// TokenNames lists the properties this rendering defines, sorted, which is what
// the token-contract check compares against.
func (r Rendered) TokenNames() []string {
	out := make([]string, 0, len(r.Tokens))
	for name := range r.Tokens {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// normalizeID is how a theme identifier is written on disk and addressed.
func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
