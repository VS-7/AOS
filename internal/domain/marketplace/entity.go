// Package marketplace is a remote registry of installable skills: discovery
// and install from repositories, over the same skill.Installer verify/
// consent/apply path every install obeys — see skill.Installer.InstallPackage.
//
// The reverse engineering could not determine the original's remote
// registry behaviour — it depends on a service never analysed — so this is
// not a compatibility copy. It is our own registry: Git-based by default (no
// central service required), with an HTTP implementation for a hosted
// index, both configured through Deps.Registries.
package marketplace

import (
	"time"

	"github.com/OWNER/aos/internal/domain/skill"
)

// Listing is one skill a registry offers, before it is fetched.
type Listing struct {
	// Registry is the configured registry id this listing came from —
	// internal/domain/marketplace has no notion of a single default
	// registry, so Install needs to know which one to fetch the full
	// package from. Not in the original design sketch's struct, added
	// because Discovery searches every configured registry at once and an
	// Install call has to resolve back to exactly one of them.
	Registry string `json:"registry"`

	Source      string    `json:"source"` // "<owner>/<repo>"
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Tags        []string  `json:"tags,omitempty"`
	Stars       int       `json:"stars,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// Permissions is surfaced at discovery time, not only at install time,
	// so the agent and the user can filter by what a skill demands before
	// fetching it — see ADR-0015.
	Permissions skill.Permissions `json:"permissions"`
}

// SearchQuery filters a Registry's Search.
type SearchQuery struct {
	Text  string `json:"text,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Owner string `json:"owner,omitempty"`
}

// RegistryConfig names one registry Deps wires — see ~/.aos/config.json's
// "marketplace.registries" in the design doc.
type RegistryConfig struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "git" | "http"
	URL  string `json:"url"`
}
