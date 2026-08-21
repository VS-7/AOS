package marketplace

import (
	"context"

	"github.com/OWNER/aos/internal/domain/skill"
)

// Registry is the outbound port a configured marketplace registry
// implements. Two are planned: internal/adapters/marketplacegit (no central
// service required) and internal/adapters/marketplacehttp (a hosted index).
type Registry interface {
	Search(ctx context.Context, q SearchQuery) ([]Listing, error)

	// Fetch returns the package at source, at ref — the same shape
	// skill.Fetcher.Fetch returns, so whatever a registry fetches goes
	// straight into skill.Installer.InstallPackage unchanged.
	Fetch(ctx context.Context, source, ref string) (skill.Package, error)
}

// Installer is the slice of skill.Installer this package needs: the
// verify/consent/apply path every install obeys, driven here against a
// package this domain fetched over its own Registry rather than through
// skill.Installer's own single, differently-configured Fetcher.
type Installer interface {
	InstallPackage(ctx context.Context, source string, pkg skill.Package, acceptedAll func(skill.Permissions) bool) (*skill.Skill, error)
}
