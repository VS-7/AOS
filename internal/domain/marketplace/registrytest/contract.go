// Package registrytest is the port contract every marketplace.Registry
// implementation obeys, run against each real adapter — internal/adapters/
// marketplacegit and internal/adapters/marketplacehttp — rather than only
// against a fake, per docs/07 "Testes de Contrato de Port".
//
// Fixture is a Build func's contract with this suite: the registry it
// returns must already contain exactly one listing at Source "acme/crm",
// owner "acme", tagged "crm", with Manifest.Name "crm" once fetched.
package registrytest

import (
	"context"
	"testing"

	"github.com/OWNER/aos/internal/domain/marketplace"
)

// Contract runs the fixture-independent assertions every Registry must
// satisfy against reg, which the caller has already seeded with the one
// listing this package's doc comment describes.
func Contract(t *testing.T, reg marketplace.Registry) {
	t.Helper()

	t.Run("SearchWithNoFilterFindsTheFixtureListing", func(t *testing.T) {
		got, err := reg.Search(context.Background(), marketplace.SearchQuery{})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if !containsSource(got, "acme/crm") {
			t.Fatalf("Search = %+v, want it to include the fixture listing acme/crm", got)
		}
	})

	t.Run("SearchByOwnerFindsTheFixtureListing", func(t *testing.T) {
		got, err := reg.Search(context.Background(), marketplace.SearchQuery{Owner: "acme"})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if !containsSource(got, "acme/crm") {
			t.Fatalf("Search by owner=acme = %+v, want it to include acme/crm", got)
		}
	})

	t.Run("SearchByTagFindsTheFixtureListing", func(t *testing.T) {
		got, err := reg.Search(context.Background(), marketplace.SearchQuery{Tag: "crm"})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if !containsSource(got, "acme/crm") {
			t.Fatalf("Search by tag=crm = %+v, want it to include acme/crm", got)
		}
	})

	t.Run("FetchReturnsTheFixturePackage", func(t *testing.T) {
		pkg, err := reg.Fetch(context.Background(), "acme/crm", "")
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if pkg.Manifest.Name != "crm" {
			t.Fatalf("Fetch = %+v, want Manifest.Name crm", pkg.Manifest)
		}
	})

	t.Run("FetchOfAnUnlistedSourceIsRefused", func(t *testing.T) {
		if _, err := reg.Fetch(context.Background(), "nobody/nothing", ""); err == nil {
			t.Fatal("Fetch of an unlisted source succeeded, want an error")
		}
	})
}

func containsSource(listings []marketplace.Listing, source string) bool {
	for _, l := range listings {
		if l.Source == source {
			return true
		}
	}
	return false
}
