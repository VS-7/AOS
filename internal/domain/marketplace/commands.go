package marketplace

import (
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/skill"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "marketplace",
	Tool:    "Marketplace",
	Summary: "A remote registry of installable skills — discover and install from repositories.",
	Doc: `A skill's own domain (skills_install) installs from a local package already
on disk. marketplace is the layer above it: a registry of packages that are
not on disk yet, configured once (Git-based by default, HTTP for a hosted
index) and searched or fetched by "<owner>/<repo>".

## Commands
- **discovery** — search every configured registry
- **get** — read one listing's full manifest before installing it
- **install** — fetch and install, going through the same verify/consent
  path skills_install does

## When to use
- **Before installing something you have not seen configured locally:**
  discovery first — a listing's Permissions are visible before any bytes are
  fetched, per ADR-0015

## When NOT to use
- Not for a package already on disk — that is skills_install directly`,
	Hint: `install still asks for consent unless the caller already has it — the same
ADR-0007 boundary skills_install obeys. A registry that does not answer is
skipped, not fatal: discovery returns whatever the reachable registries
found.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[DiscoveryInput, []Listing]{
		Group:   "marketplace",
		Name:    "discovery",
		Summary: "Search every configured marketplace registry.",
		Doc:     "Search across every configured registry, merging results. A registry that does not answer is skipped, not fatal.",
		Examples: []command.Example{
			{Description: "search by free text", Input: DiscoveryInput{Text: "crm"}},
			{Description: "search by owner", Input: DiscoveryInput{Owner: "acme"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Search the marketplace", ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		Handler:     svc.Discovery,
	})

	command.MustRegister(reg, command.Command[GetInput, *Listing]{
		Group:   "marketplace",
		Name:    "get",
		Summary: "Read one listing's full manifest.",
		Doc:     "Read a listing in full — its manifest and the permissions it declares — before installing it.",
		Examples: []command.Example{
			{Description: "read a listing before installing it", Input: GetInput{Source: "acme/crm"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a marketplace listing", ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[InstallInput, *skill.Skill]{
		Group:   "marketplace",
		Name:    "install",
		Summary: "Fetch and install a skill from a registry.",
		Doc: `Fetches "<owner>/<repo>" from the named registry, or every configured
registry until one answers, then verifies and installs it exactly as
skills_install does — including asking for consent unless the caller
already has it (ADR-0007).`,
		Examples: []command.Example{
			{Description: "install from any configured registry", Input: InstallInput{Source: "acme/crm"}},
		},
		Registry: true,
		Annotations: command.Annotations{
			Title: "Install from the marketplace", DestructiveHint: true, OpenWorldHint: true,
		},
		Handler: svc.Install,
	})
}
