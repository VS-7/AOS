package marketplace

import "github.com/OWNER/aos/internal/core/apperr"

// errSourceRequired fires when a caller names no package to fetch.
func errSourceRequired() error {
	return apperr.New("MARKETPLACE_SOURCE_REQUIRED").
		Causer("marketplace.Service").
		Msgf("a source is required").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "name the package, \"<owner>/<repo>\""})
}

// errNoRegistriesConfigured fires when Deps.Registries is empty — a
// marketplace with nothing configured to search or install from.
func errNoRegistriesConfigured() error {
	return apperr.New("MARKETPLACE_NO_REGISTRIES_CONFIGURED").
		Causer("marketplace.Service").
		Msgf("no marketplace registries are configured").
		Status(apperr.StatusPreconditionFailed).
		CTA(apperr.CallToAction{Label: "add a registry under marketplace.registries in the workspace config"})
}

// errRegistryUnknown fires when a caller names a registry id that is not
// configured.
func errRegistryUnknown(id string) error {
	return apperr.New("MARKETPLACE_REGISTRY_UNKNOWN").
		Causer("marketplace.Service").
		Msgf("no registry %q is configured", id).
		Issue("registry", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "list the configured registries in the workspace config"})
}

// errRegistryUnreachable wraps a Search or Fetch failure against one
// registry — recorded as this Registry's own reason, not escalated as the
// whole call's failure while other configured registries might still
// answer, per the design doc's "Registry indisponível degrada com erro
// claro e CTA, sem travar o CLI".
func errRegistryUnreachable(id string, cause error) error {
	return apperr.New("MARKETPLACE_REGISTRY_UNREACHABLE").
		Causer("marketplace.Service").
		Msgf("registry %q did not answer: %v", id, cause).
		Issue("registry", id).
		Status(apperr.StatusBadGateway).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check the registry's configuration and that its source is reachable"})
}

// errFetchFailed fires when Install exhausted every registry it tried
// without one successfully fetching source.
func errFetchFailed(source string, cause error) error {
	b := apperr.New("MARKETPLACE_FETCH_FAILED").
		Causer("marketplace.Service.Install").
		Msgf("could not fetch %q from any configured registry", source).
		Issue("source", source).
		Status(apperr.StatusBadGateway).
		CTA(apperr.CallToAction{Label: "check the source and that at least one configured registry can reach it"})
	if cause != nil {
		b = b.Wrap(cause)
	}
	return b
}

// errListingNotFound fires when Get finds no listing at source in any
// registry it searched.
func errListingNotFound(source string) error {
	return apperr.New("MARKETPLACE_LISTING_NOT_FOUND").
		Causer("marketplace.Service.Get").
		Msgf("no listing %q in any configured registry", source).
		Issue("source", source).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "search the marketplace before fetching a specific listing", Tool: "marketplace_discovery"})
}
