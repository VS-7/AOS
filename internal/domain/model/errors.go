package model

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errNotConnected(provider string, connected []string) error {
	return apperr.New("MODEL_PROVIDER_NOT_CONNECTED").
		Causer("model.Service.List").
		Msgf("%q has no credential on this installation, so there is nothing to ask it with", provider).
		Issue("provider", provider).
		Issue("connected", connected).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "connect the provider first; the ones already connected are in the issue",
			Command: build.Name + " config get",
		})
}

func errNoCatalog() error {
	return apperr.New("MODEL_CATALOG_UNAVAILABLE").
		Causer("model.Service.List").
		Msgf("this process cannot reach model providers").
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "this is a wiring defect, not a configuration one",
		})
}
