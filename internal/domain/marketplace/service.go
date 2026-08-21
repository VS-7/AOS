package marketplace

import (
	"context"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/skill"
)

// Deps is what a Service is built from.
type Deps struct {
	// Registries is every configured registry, keyed by its config id
	// (RegistryConfig.ID). Built once, at wire time, from
	// ~/.aos/config.json's "marketplace.registries" — this package never
	// reads config itself, the same reason every other domain here takes
	// its dependencies already resolved rather than a raw config.Service.
	Registries map[string]Registry

	// Order is the id order Discovery searches in and Install falls back
	// through when a caller does not name a registry. Nil means map
	// iteration order, which Go leaves undefined — tests and anything that
	// cares about a deterministic result order should set this.
	Order []string

	Installer Installer
}

// Service is the marketplace domain: discovery and install, over whatever
// registries Deps configured.
type Service struct {
	registries map[string]Registry
	order      []string
	installer  Installer
}

// NewService builds a Service over its dependencies.
func NewService(d Deps) *Service {
	order := d.Order
	if order == nil {
		order = make([]string, 0, len(d.Registries))
		for id := range d.Registries {
			order = append(order, id)
		}
		sort.Strings(order)
	}
	return &Service{registries: d.Registries, order: order, installer: d.Installer}
}

// DiscoveryInput filters Discovery's search.
type DiscoveryInput struct {
	command.Reasoning
	Text  string `json:"text,omitempty" jsonschema:"Free-text search across name, description and tags."`
	Tag   string `json:"tag,omitempty" jsonschema:"Filter to listings carrying this tag."`
	Owner string `json:"owner,omitempty" jsonschema:"Filter to listings from this owner (the <owner> half of <owner>/<repo>)."`
}

// GetInput names one listing by the registry it came from and its source.
type GetInput struct {
	command.Reasoning
	Registry string `json:"registry,omitempty" jsonschema:"Which configured registry to read from. Empty tries every configured registry."`
	Source   string `json:"source" jsonschema:"The listing's source, \"<owner>/<repo>\"." validate:"required,notblank"`
}

// InstallInput names the package to install.
type InstallInput struct {
	command.Reasoning

	// Registry names which configured registry to fetch Source from. Empty
	// tries every configured registry, in Order, until one's Fetch
	// succeeds — the same "search until found" shape Discovery already
	// gives a caller that found a Listing without recording which registry
	// it came from.
	Registry string `json:"registry,omitempty" jsonschema:"Which configured registry to fetch from. Empty tries every configured registry."`
	Source   string `json:"source" jsonschema:"The package's source, \"<owner>/<repo>\"." validate:"required,notblank"`
	Ref      string `json:"ref,omitempty" jsonschema:"Version or ref to fetch. Empty fetches the registry's default."`

	// acceptedAll is passed straight through to
	// skill.Installer.InstallPackage — see its own doc on why nil means
	// "ask" (ADR-0007). Unexported: a function cannot arrive over MCP, CLI
	// or HTTP, so it is not part of this command's schema — every call
	// through the registry gets nil, i.e. "ask", the same default every
	// other caller of InstallPackage gets. Set only by Go code calling
	// Service.Install directly, in the same package.
	acceptedAll func(skill.Permissions) bool
}

// Discovery searches every configured registry and merges the results.
//
// A registry that fails to answer is skipped, not fatal — a person
// searching should see what the reachable registries found rather than
// nothing, per the design doc's "Registry indisponível degrada com erro
// claro e CTA, sem travar o CLI". Only when every configured registry fails
// does Discovery itself fail, with the last error as the cause.
func (s *Service) Discovery(ctx context.Context, in DiscoveryInput) ([]Listing, error) {
	if len(s.registries) == 0 {
		return nil, errNoRegistriesConfigured()
	}
	q := SearchQuery{Text: strings.TrimSpace(in.Text), Tag: strings.TrimSpace(in.Tag), Owner: strings.TrimSpace(in.Owner)}

	var out []Listing
	var lastErr error
	reached := 0
	for _, id := range s.order {
		reg, ok := s.registries[id]
		if !ok {
			continue
		}
		found, err := reg.Search(ctx, q)
		if err != nil {
			lastErr = errRegistryUnreachable(id, err)
			continue
		}
		reached++
		for _, l := range found {
			l.Registry = id
			out = append(out, l)
		}
	}
	if reached == 0 {
		return nil, lastErr
	}
	return out, nil
}

// Get retrieves one listing by its source, searching the named registry or
// every configured registry in Order.
func (s *Service) Get(ctx context.Context, in GetInput) (*Listing, error) {
	source := strings.TrimSpace(in.Source)
	if source == "" {
		return nil, errSourceRequired()
	}

	ids := s.order
	if in.Registry != "" {
		if _, ok := s.registries[in.Registry]; !ok {
			return nil, errRegistryUnknown(in.Registry)
		}
		ids = []string{in.Registry}
	}

	var lastErr error
	for _, id := range ids {
		reg := s.registries[id]
		found, err := reg.Search(ctx, SearchQuery{Text: source})
		if err != nil {
			lastErr = errRegistryUnreachable(id, err)
			continue
		}
		for _, l := range found {
			if l.Source == source {
				l.Registry = id
				return &l, nil
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errListingNotFound(source)
}

// Install fetches and installs a package, trying the named registry, or
// every configured registry in Order when Registry is empty.
func (s *Service) Install(ctx context.Context, in InstallInput) (*skill.Skill, error) {
	source := strings.TrimSpace(in.Source)
	if source == "" {
		return nil, errSourceRequired()
	}

	ids := s.order
	if in.Registry != "" {
		if _, ok := s.registries[in.Registry]; !ok {
			return nil, errRegistryUnknown(in.Registry)
		}
		ids = []string{in.Registry}
	}
	if len(ids) == 0 {
		return nil, errNoRegistriesConfigured()
	}

	var lastErr error
	for _, id := range ids {
		reg := s.registries[id]
		pkg, err := reg.Fetch(ctx, source, in.Ref)
		if err != nil {
			lastErr = errRegistryUnreachable(id, err)
			continue
		}
		return s.installer.InstallPackage(ctx, source, pkg, in.acceptedAll)
	}
	return nil, errFetchFailed(source, lastErr)
}
