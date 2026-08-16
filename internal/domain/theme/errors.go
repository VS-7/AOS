package theme

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errNotFound(id string) error {
	return apperr.New("THEME_NOT_FOUND").
		Causer("theme.Service.Get").
		Msgf("no theme named %q is installed", id).
		Issue("theme", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the themes this build ships with",
			Command: build.Name + " themes list",
			Tool:    "themes_list",
		})
}

func errInvalidID(id string) error {
	return apperr.New("THEME_INVALID_ID").
		Causer("theme.Validate").
		Msgf("%q does not identify a theme", id).
		Issue("id", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "give the theme a short identifier, as in \"my-theme\""})
}

func errNoVariants(id string) error {
	return apperr.New("THEME_NO_VARIANTS").
		Causer("theme.Validate").
		Msgf("a theme with no palette has nothing to render").
		Issue("theme", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "declare a light palette, a dark one, or both — a theme with both can follow the system",
		})
}

func errUnknownVariant(id, appearance string) error {
	return apperr.New("THEME_UNKNOWN_VARIANT").
		Causer("theme.Validate").
		Msgf("%q is not an appearance a palette can be for", appearance).
		Issue("theme", id).
		Issue("appearance", appearance).
		Issue("valid", []string{"light", "dark"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "a palette is light or dark; auto is what a theme carrying both becomes",
		})
}

func errInvalidAppearance(appearance string) error {
	return apperr.New("THEME_INVALID_APPEARANCE").
		Causer("theme.Service").
		Msgf("%q is not an appearance", appearance).
		Issue("appearance", appearance).
		Issue("valid", []string{"light", "dark", "auto"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use light, dark or auto"})
}

func errBadColour(id, field, value string, cause error) error {
	return apperr.New("THEME_INVALID_COLOUR").
		Causer("theme.Render").
		Msgf("%s of theme %q is %q, which is not a colour", field, id, value).
		Issue("theme", id).
		Issue("field", field).
		Issue("value", value).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "write the colour as a hex value, as in #2e3440"})
}

func errNoPalette(id string, want Appearance) error {
	return apperr.New("THEME_NO_PALETTE").
		Causer("theme.Render").
		Msgf("theme %q has no palette at all", id).
		Issue("theme", id).
		Issue("appearance", string(want)).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{Label: "install the theme again with a light or dark palette"})
}

// errMissingTokens is the token contract failing. A theme short of a property
// renders an invisible element somewhere, and the element it renders invisible
// is never the one you are looking at when you notice.
func errMissingTokens(id, appearance string, missing []string) error {
	return apperr.New("THEME_MISSING_TOKENS").
		Causer("theme.Validate").
		Msgf("the %s palette of %q produces %d fewer properties than the interface reads", appearance, id, len(missing)).
		Issue("theme", id).
		Issue("appearance", appearance).
		Issue("missing", missing).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "the missing properties are " + strings.Join(missing, ", ") +
				"; every one is derived, so a palette that produces none of them is missing a colour rather than a token",
		})
}

func errShadowsBuiltin(id string) error {
	return apperr.New("THEME_SHADOWS_BUILTIN").
		Causer("theme.Service.Install").
		Msgf("%q is the identifier of a theme this build ships with", id).
		Issue("theme", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "pick a different identifier; a preset that shadowed a built-in one would make the built-in unreachable",
		})
}

func errBuiltinIsPermanent(id string) error {
	return apperr.New("THEME_BUILTIN_IS_PERMANENT").
		Causer("theme.Service.Delete").
		Msgf("%q ships with the application and cannot be removed", id).
		Issue("theme", id).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "it is embedded in the binary, so deleting it would only mean pretending it is gone; choose another theme instead",
		})
}

func errStoreUnavailable() error {
	return apperr.New("THEME_STORE_UNAVAILABLE").
		Causer("theme.Service.Install").
		Msgf("this process has nowhere to keep a theme preset").
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{
			Label: "the built-in themes still work; a preset needs a state directory to live in",
		})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("THEME_READ_FAILED").
		Causer("theme.Service." + op).
		Msgf("the installed themes could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("THEME_WRITE_FAILED").
		Causer("theme.Service." + op).
		Msgf("the theme could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
