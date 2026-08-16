// Package oklch converts colours to OKLCH and mixes them there.
//
// It exists because the original derives every surface of a theme from three
// colours and a contrast dial, and it does that mixing in OKLCH rather than in
// sRGB. The difference is not academic: mixing two colours in sRGB darkens and
// desaturates them in a way that makes a "slightly raised surface" read as a
// different, muddier colour. OKLCH is perceptually uniform, so a mix at 90/10
// looks like a mix at 90/10.
//
// The conversion is the standard sRGB → linear → CIE XYZ (D65) → Oklab → OKLCH
// chain from Björn Ottosson's definition, which is what CSS Color 4 specifies
// and therefore what the browser will agree with.
package oklch

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Color is a colour in OKLCH: lightness 0–1, chroma, hue in degrees.
type Color struct {
	L float64
	C float64
	H float64
}

// ParseHex reads "#rgb", "#rrggbb" or "#rrggbbaa" and converts to OKLCH.
//
// The alpha channel is read and discarded: a theme's alpha is applied by the
// token that uses the colour, not carried by the colour itself.
func ParseHex(hex string) (Color, error) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(hex), "#"))
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	case 8:
		s = s[:6]
	default:
		return Color{}, fmt.Errorf("oklch: %q is not a hex colour", hex)
	}

	value, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("oklch: %q is not a hex colour", hex)
	}
	r := float64((value>>16)&0xff) / 255
	g := float64((value>>8)&0xff) / 255
	b := float64(value&0xff) / 255
	return FromSRGB(r, g, b), nil
}

// FromSRGB converts gamma-encoded sRGB components in 0–1 to OKLCH.
func FromSRGB(r, g, b float64) Color {
	lr, lg, lb := linearize(r), linearize(g), linearize(b)

	l := math.Cbrt(0.4122214708*lr + 0.5363325363*lg + 0.0514459929*lb)
	m := math.Cbrt(0.2119034982*lr + 0.6806995451*lg + 0.1073969566*lb)
	s := math.Cbrt(0.0883024619*lr + 0.2817188376*lg + 0.6299787005*lb)

	labL := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	labA := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	labB := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s

	chroma := math.Hypot(labA, labB)
	hue := math.Atan2(labB, labA) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return Color{L: labL, C: chroma, H: hue}
}

// linearize undoes the sRGB transfer function.
func linearize(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// SRGB converts back to gamma-encoded sRGB in 0–1, clamped to the gamut.
//
// Clamping rather than gamut-mapping is deliberate: this is used only to
// measure contrast, and a colour outside sRGB is one the display will clamp
// anyway. A theme whose contrast depends on out-of-gamut precision is a theme
// nobody can actually read.
func (c Color) SRGB() (r, g, b float64) {
	l := c.L + 0.3963377774*math.Cos(c.H*math.Pi/180)*c.C + 0.2158037573*math.Sin(c.H*math.Pi/180)*c.C
	m := c.L - 0.1055613458*math.Cos(c.H*math.Pi/180)*c.C - 0.0638541728*math.Sin(c.H*math.Pi/180)*c.C
	s := c.L - 0.0894841775*math.Cos(c.H*math.Pi/180)*c.C - 1.2914855480*math.Sin(c.H*math.Pi/180)*c.C

	l, m, s = l*l*l, m*m*m, s*s*s

	lr := +4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	lg := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	lb := -0.0041960863*l - 0.7034186147*m + 1.7076147010*s

	return clamp(delinearize(lr)), clamp(delinearize(lg)), clamp(delinearize(lb))
}

func delinearize(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func clamp(v float64) float64 {
	return math.Min(1, math.Max(0, v))
}

// Mix blends two colours by weight, interpolating hue the short way round.
//
// Hue is an angle, so averaging the numbers is wrong: the midpoint of 350° and
// 10° is 0°, not 180°. Interpolating the unit vectors and taking the angle back
// is what gives the answer a person expects.
func Mix(a, b Color, weightA, weightB float64) Color {
	total := weightA + weightB
	pa, pb := 0.5, 0.5
	if total != 0 {
		pa, pb = weightA/total, weightB/total
	}

	ar, br := a.H*math.Pi/180, b.H*math.Pi/180
	x := math.Cos(ar)*pa + math.Cos(br)*pb
	y := math.Sin(ar)*pa + math.Sin(br)*pb
	hue := math.Atan2(y, x) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return Color{L: a.L*pa + b.L*pb, C: a.C*pa + b.C*pb, H: hue}
}

// CSS renders the colour as a CSS Color 4 oklch() function.
//
// The precision matches the original's: three decimals of lightness, four of
// chroma, two of hue. It is enough to be indistinguishable and short enough
// that a stylesheet stays readable.
func (c Color) CSS() string {
	return fmt.Sprintf("oklch(%.3f %.4f %.2f)", c.L, c.C, c.H)
}

// CSSAlpha renders the colour with an alpha channel.
func (c Color) CSSAlpha(alpha float64) string {
	return fmt.Sprintf("oklch(%.3f %.4f %.2f / %.2f)", c.L, c.C, c.H, alpha)
}

// Luminance is the WCAG relative luminance, for contrast.
func (c Color) Luminance() float64 {
	r, g, b := c.SRGB()
	return 0.2126*linearize(r) + 0.7152*linearize(g) + 0.0722*linearize(b)
}

// Contrast is the WCAG 2 contrast ratio between two colours, from 1 to 21.
func Contrast(a, b Color) float64 {
	la, lb := a.Luminance(), b.Luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// The two WCAG 2 thresholds for text.
const (
	// ContrastAA is the ratio normal-size text must reach.
	ContrastAA = 4.5

	// ContrastAALarge is the ratio large text must reach.
	ContrastAALarge = 3.0
)
