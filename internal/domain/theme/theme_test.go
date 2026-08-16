package theme

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/oklch"
)

// memStore is the Store port in memory. The real one writes YAML and has its
// own suite; what is tested here is the aggregate.
type memStore struct {
	mu     sync.Mutex
	themes map[string]Theme
	failOn string
}

func newStore() *memStore { return &memStore{themes: map[string]Theme{}} }

func (s *memStore) List(context.Context) ([]Theme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn == "list" {
		return nil, errors.New("the theme directory is unreadable")
	}
	out := make([]Theme, 0, len(s.themes))
	for _, t := range s.themes {
		out = append(out, t)
	}
	return out, nil
}

func (s *memStore) Get(_ context.Context, id string) (*Theme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.themes[id]
	if !ok {
		return nil, errors.New("no such theme")
	}
	return &t, nil
}

func (s *memStore) Save(_ context.Context, t Theme) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn == "save" {
		return errors.New("no space left")
	}
	s.themes[t.ID] = t
	return nil
}

func (s *memStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.themes, id)
	return nil
}

func newService(t *testing.T, store Store) *Service {
	t.Helper()
	svc, err := NewService(Deps{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func ctx() context.Context { return context.Background() }

// TestTheThirtyEightBuiltinThemesLoadAndValidate. They are embedded, so a
// broken one was broken when the binary was made — which is why NewService
// refuses to start rather than skipping it.
func TestTheThirtyEightBuiltinThemesLoadAndValidate(t *testing.T) {
	svc := newService(t, nil)

	out, err := svc.List(ctx(), ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 38 {
		t.Fatalf("%d built-in themes, want 38", out.Total)
	}
	for _, theme := range out.Themes {
		if !theme.Builtin {
			t.Fatalf("%q is not marked built-in", theme.ID)
		}
		if theme.Appearance() != Auto {
			t.Fatalf("%q carries only %s, so it cannot follow the system", theme.ID, theme.Appearance())
		}
	}
}

// TestEveryBuiltinThemeReachesWCAGAAOnItsOwnText. A theme that is beautiful and
// unreadable is an accessibility defect, and the original has no such check.
func TestEveryBuiltinThemeReachesWCAGAAOnItsOwnText(t *testing.T) {
	svc := newService(t, nil)
	out, err := svc.List(ctx(), ListInput{})
	if err != nil {
		t.Fatal(err)
	}

	for _, theme := range out.Themes {
		for _, appearance := range theme.Appearances() {
			palette, _, _ := theme.Palette(appearance)
			surface, err := oklch.ParseHex(palette.Surface)
			if err != nil {
				t.Fatalf("%s/%s: %v", theme.ID, appearance, err)
			}
			ink, err := oklch.ParseHex(palette.Ink)
			if err != nil {
				t.Fatalf("%s/%s: %v", theme.ID, appearance, err)
			}
			if ratio := oklch.Contrast(surface, ink); ratio < oklch.ContrastAA {
				t.Errorf("%s/%s: body text is %.2f:1 against the surface, below the %.1f:1 AA floor",
					theme.ID, appearance, ratio, oklch.ContrastAA)
			}
		}
	}
}

// TestTheAccentIsLegibleAsLargeText. The accent carries headings, links and the
// primary button, all of which are large or bold — so the bar is the large-text
// one, not the body-text one.
func TestTheAccentIsLegibleAsLargeText(t *testing.T) {
	svc := newService(t, nil)
	out, _ := svc.List(ctx(), ListInput{})

	for _, theme := range out.Themes {
		for _, appearance := range theme.Appearances() {
			palette, _, _ := theme.Palette(appearance)
			surface, _ := oklch.ParseHex(palette.Surface)
			accent, _ := oklch.ParseHex(palette.Accent)
			if ratio := oklch.Contrast(surface, accent); ratio < oklch.ContrastAALarge {
				t.Errorf("%s/%s: the accent is %.2f:1 against the surface, below the %.1f:1 floor for large text",
					theme.ID, appearance, ratio, oklch.ContrastAALarge)
			}
		}
	}
}

// TestARenderedThemeDefinesEveryTokenTheInterfaceReads. This is the contract in
// tokens.txt, and it is why a theme is validated by rendering it rather than by
// inspecting its fields: the tokens are derived, so the only honest check is to
// derive them.
func TestARenderedThemeDefinesEveryTokenTheInterfaceReads(t *testing.T) {
	svc := newService(t, nil)
	required := RequiredTokens()
	if len(required) < 40 {
		t.Fatalf("the token contract has only %d entries; tokens.txt did not load", len(required))
	}

	out, _ := svc.List(ctx(), ListInput{})
	for _, theme := range out.Themes {
		for _, appearance := range theme.Appearances() {
			rendered, err := Render(theme, appearance, true)
			if err != nil {
				t.Fatalf("%s/%s: %v", theme.ID, appearance, err)
			}
			for _, name := range required {
				if _, ok := rendered.Tokens[name]; !ok {
					t.Fatalf("%s/%s does not define --%s", theme.ID, appearance, name)
				}
			}
		}
	}
}

// TestTheThirteenMemoryCategoriesEachGetTheirOwnColour, derived from the accent
// so the graph stays readable in every theme.
func TestTheThirteenMemoryCategoriesEachGetTheirOwnColour(t *testing.T) {
	svc := newService(t, nil)
	out, _ := svc.List(ctx(), ListInput{})

	for _, theme := range out.Themes {
		rendered, err := Render(theme, Dark, true)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]string{}
		for _, category := range MemoryCategories {
			colour, ok := rendered.Tokens["category-"+category]
			if !ok {
				t.Fatalf("%s has no colour for the %s category", theme.ID, category)
			}
			if other, clash := seen[colour]; clash {
				t.Fatalf("%s gives %s and %s the same colour %s", theme.ID, category, other, colour)
			}
			seen[colour] = category
		}
	}
}

// TestBlurOnlyHappensWhereThereIsSomethingBehindTheWindow. In a browser a
// translucent background reveals white, not the desktop.
func TestBlurOnlyHappensWhereThereIsSomethingBehindTheWindow(t *testing.T) {
	svc := newService(t, nil)
	nord, err := svc.find(ctx(), "nord")
	if err != nil {
		t.Fatal(err)
	}

	native, err := Render(*nord, Dark, true)
	if err != nil {
		t.Fatal(err)
	}
	if native.Windows != Blur {
		t.Fatalf("the desktop rendering is %s", native.Windows)
	}
	if !strings.Contains(native.Tokens["background"], "/") {
		t.Fatalf("a blur window has an opaque background: %q", native.Tokens["background"])
	}
	if native.Tokens["sidebar"] != "transparent" {
		t.Fatalf("a blur window has an opaque sidebar: %q", native.Tokens["sidebar"])
	}

	browser, err := Render(*nord, Dark, false)
	if err != nil {
		t.Fatal(err)
	}
	if browser.Windows != Solid {
		t.Fatalf("the browser rendering is %s", browser.Windows)
	}
	if strings.Contains(browser.Tokens["background"], "/") {
		t.Fatalf("the browser got a translucent background: %q", browser.Tokens["background"])
	}
}

// TestContrastMovesTheDerivedLayers. It is the dial the whole model turns on:
// the same three colours at contrast 0 and contrast 100 are two different
// interfaces.
func TestContrastMovesTheDerivedLayers(t *testing.T) {
	base := Theme{ID: "probe", Variants: map[Appearance]Palette{
		Dark: {Surface: "#101010", Ink: "#f0f0f0", Accent: "#7aa2f7", Contrast: 0},
	}}
	washed := Theme{ID: "probe", Variants: map[Appearance]Palette{
		Dark: {Surface: "#101010", Ink: "#f0f0f0", Accent: "#7aa2f7", Contrast: 100},
	}}

	punchy, err := Render(base, Dark, false)
	if err != nil {
		t.Fatal(err)
	}
	subtle, err := Render(washed, Dark, false)
	if err != nil {
		t.Fatal(err)
	}
	if punchy.Tokens["card"] == subtle.Tokens["card"] {
		t.Fatal("the contrast dial changed nothing")
	}
}

// TestAThemeThatWouldRenderNothingIsRefused.
func TestAThemeThatWouldRenderNothingIsRefused(t *testing.T) {
	cases := []struct {
		name  string
		theme Theme
		code  string
	}{
		{"no identifier", Theme{Variants: map[Appearance]Palette{
			Dark: {Surface: "#000", Ink: "#fff", Accent: "#f00"},
		}}, "THEME_INVALID_ID"},
		{"no palette", Theme{ID: "empty"}, "THEME_NO_VARIANTS"},
		{"a palette for an appearance that is not one", Theme{
			ID: "odd", Variants: map[Appearance]Palette{"sepia": {Surface: "#000", Ink: "#fff", Accent: "#f00"}},
		}, "THEME_UNKNOWN_VARIANT"},
		{"a colour that is not one", Theme{
			ID: "broken", Variants: map[Appearance]Palette{
				Dark: {Surface: "midnight", Ink: "#fff", Accent: "#f00"},
			},
		}, "THEME_INVALID_COLOUR"},
	}
	for _, tc := range cases {
		err := Validate(tc.theme)
		if err == nil {
			t.Fatalf("%s was accepted", tc.name)
		}
		got, ok := apperr.As(err)
		if !ok || !strings.HasSuffix(got.Code, tc.code) {
			t.Fatalf("%s: error = %v, want %s", tc.name, err, tc.code)
		}
	}
}

// TestAPresetIsValidatedBeforeItIsStored, so an unreadable interface is caught
// on install rather than by somebody squinting at it.
func TestAPresetIsValidatedBeforeItIsStored(t *testing.T) {
	store := newStore()
	svc := newService(t, store)

	if _, err := svc.Install(ctx(), InstallInput{
		ID: "broken", Variants: map[Appearance]Palette{
			Dark: {Surface: "not a colour", Ink: "#fff", Accent: "#f00"},
		},
	}); err == nil {
		t.Fatal("a preset with a colour that is not one was stored")
	}
	if len(store.themes) != 0 {
		t.Fatal("the refused preset was stored anyway")
	}

	installed, err := svc.Install(ctx(), InstallInput{
		ID: "midnight", Variants: map[Appearance]Palette{
			Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7", Contrast: 70},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Name != "midnight" {
		t.Fatalf("the name did not fall back to the identifier: %q", installed.Name)
	}
	if installed.Appearance() != Dark {
		t.Fatalf("a single-palette theme reports %s", installed.Appearance())
	}
}

// TestAPresetCannotShadowABuiltinOne, because the built-in would become
// unreachable and nobody would be able to say why.
func TestAPresetCannotShadowABuiltinOne(t *testing.T) {
	svc := newService(t, newStore())
	_, err := svc.Install(ctx(), InstallInput{
		ID: "nord", Variants: map[Appearance]Palette{
			Dark: {Surface: "#000000", Ink: "#ffffff", Accent: "#7aa2f7"},
		},
	})
	if err == nil {
		t.Fatal("a preset shadowed a built-in theme")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "THEME_SHADOWS_BUILTIN") {
		t.Fatalf("error = %v", err)
	}
}

// TestABuiltinThemeCannotBeDeleted. It is in the binary; deleting it would only
// mean pretending it is gone.
func TestABuiltinThemeCannotBeDeleted(t *testing.T) {
	svc := newService(t, newStore())
	_, err := svc.Delete(ctx(), DeleteInput{ID: "nord"})
	if err == nil {
		t.Fatal("a built-in theme was deleted")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "THEME_BUILTIN_IS_PERMANENT") {
		t.Fatalf("error = %v", err)
	}
}

// TestAPresetIsDeletedAndABuiltinSurvivesIt.
func TestAPresetIsDeletedAndABuiltinSurvivesIt(t *testing.T) {
	store := newStore()
	svc := newService(t, store)

	if _, err := svc.Install(ctx(), InstallInput{
		ID: "midnight", Variants: map[Appearance]Palette{
			Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	before, _ := svc.List(ctx(), ListInput{})
	if before.Total != 39 {
		t.Fatalf("%d themes after installing one", before.Total)
	}

	if _, err := svc.Delete(ctx(), DeleteInput{ID: "midnight"}); err != nil {
		t.Fatal(err)
	}
	after, _ := svc.List(ctx(), ListInput{})
	if after.Total != 38 {
		t.Fatalf("%d themes after deleting it", after.Total)
	}
	if _, err := svc.Delete(ctx(), DeleteInput{ID: "midnight"}); err == nil {
		t.Fatal("deleting it twice reported success")
	}
}

// TestWithoutAStoreTheBuiltinThemesStillWork, which is what a run with no state
// directory gets.
func TestWithoutAStoreTheBuiltinThemesStillWork(t *testing.T) {
	svc := newService(t, nil)

	out, err := svc.List(ctx(), ListInput{})
	if err != nil || out.Total != 38 {
		t.Fatalf("%d themes, %v", out.Total, err)
	}
	if _, err := svc.Install(ctx(), InstallInput{ID: "x"}); err == nil {
		t.Fatal("a preset was installed with nowhere to keep it")
	}
}

// TestGetRendersEveryAppearanceTheThemeOffers.
func TestGetRendersEveryAppearanceTheThemeOffers(t *testing.T) {
	svc := newService(t, nil)

	out, err := svc.Get(ctx(), GetInput{ID: "NORD", Native: true})
	if err != nil {
		t.Fatalf("the identifier is not matched case-insensitively: %v", err)
	}
	if len(out.Rendered) != 2 {
		t.Fatalf("rendered %d appearances", len(out.Rendered))
	}
	seen := map[Appearance]bool{}
	for _, r := range out.Rendered {
		seen[r.Appearance] = true
		if len(r.TokenNames()) != len(r.Tokens) {
			t.Fatal("TokenNames lost an entry")
		}
	}
	if !seen[Dark] || !seen[Light] {
		t.Fatalf("appearances = %v", seen)
	}

	if _, err := svc.Get(ctx(), GetInput{ID: "no-such-theme"}); err == nil {
		t.Fatal("a theme that does not exist was read")
	}
	if _, err := svc.Get(ctx(), GetInput{ID: "nord", Appearance: "sepia"}); err == nil {
		t.Fatal("an appearance that is not one was accepted")
	}
}

// TestListFiltersByWhatAThemeCanBe.
func TestListFiltersByWhatAThemeCanBe(t *testing.T) {
	store := newStore()
	svc := newService(t, store)
	if _, err := svc.Install(ctx(), InstallInput{
		ID: "dark-only", Variants: map[Appearance]Palette{
			Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	auto, err := svc.List(ctx(), ListInput{Appearance: Auto})
	if err != nil {
		t.Fatal(err)
	}
	if auto.Total != 38 {
		t.Fatalf("%d themes can follow the system, want the 38 built-in ones", auto.Total)
	}

	dark, err := svc.List(ctx(), ListInput{Appearance: Dark})
	if err != nil {
		t.Fatal(err)
	}
	if dark.Total != 39 {
		t.Fatalf("%d themes offer dark", dark.Total)
	}
	if _, err := svc.List(ctx(), ListInput{Appearance: "sepia"}); err == nil {
		t.Fatal("an appearance that is not one was accepted")
	}
}

// TestAStoreThatCannotBeReadIsReportedRatherThanShowingOnlyTheBuiltins.
func TestAStoreThatCannotBeReadIsReportedRatherThanShowingOnlyTheBuiltins(t *testing.T) {
	store := newStore()
	store.failOn = "list"
	svc := newService(t, store)

	if _, err := svc.List(ctx(), ListInput{}); err == nil {
		t.Fatal("an unreadable store listed as empty")
	}

	store.failOn = "save"
	if _, err := svc.Install(ctx(), InstallInput{
		ID: "midnight", Variants: map[Appearance]Palette{
			Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7"},
		},
	}); err == nil {
		t.Fatal("a store that refuses every write reported success")
	}
}

// TestRegisterPublishesTheWholeGroup.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	reg := command.NewRegistry()
	Register(reg, newService(t, newStore()))

	want := []string{"themes_delete", "themes_get", "themes_install", "themes_list"}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for _, d := range reg.Sorted() {
		if d.Key() == "themes_delete" && !d.Annotations().DestructiveHint {
			t.Error("deleting a theme must be announced destructive")
		}
	}
}

// TestARadiusFallsBackRatherThanRenderingNothing.
func TestARadiusFallsBackRatherThanRenderingNothing(t *testing.T) {
	cases := map[Radius]string{
		"none": "0rem", "sm": "0.375rem", "md": "0.75rem", "lg": "1rem",
		"": "1rem", "enormous": "1rem",
	}
	for radius, want := range cases {
		if got := radius.CSS(); got != want {
			t.Errorf("radius %q = %q, want %q", radius, got, want)
		}
	}
}

// TestAPaletteIsFoundEvenWhenTheAppearanceAskedForIsMissing, because a theme
// with one palette should still render rather than refusing.
func TestAPaletteIsFoundEvenWhenTheAppearanceAskedForIsMissing(t *testing.T) {
	only := Theme{ID: "light-only", Variants: map[Appearance]Palette{
		Light: {Surface: "#ffffff", Ink: "#111111", Accent: "#0055aa"},
	}}

	palette, resolved, ok := only.Palette(Dark)
	if !ok || resolved != Light || palette.Surface != "#ffffff" {
		t.Fatalf("palette = %+v, %s, %v", palette, resolved, ok)
	}
	if _, _, ok := (Theme{ID: "empty"}).Palette(Dark); ok {
		t.Fatal("a theme with no palette produced one")
	}
}
