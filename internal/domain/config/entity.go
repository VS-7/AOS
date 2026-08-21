// Package config is the global configuration of the installation.
//
// The file lives at ~/.aos/config.json and is meant to be edited by hand with
// the daemon running, exactly as in the original. What is not as in the
// original: nothing tagged secret:"true" ever leaves this package unredacted
// on a path an agent can reach (ADR-0010).
package config

// Config is the whole of ~/.aos/config.json. Field order and JSON keys follow
// the original so that a user moving between the two products recognises the
// file; the defaults do not, where the original's defaults were unsafe.
type Config struct {
	User          User          `json:"user"`
	Agents        Agents        `json:"agents"`
	Region        Region        `json:"region"`
	General       General       `json:"general"`
	Notifications Notifications `json:"notifications"`
	Security      Security      `json:"security"`
	Telemetry     Telemetry     `json:"telemetry"`
	Tunnel        Tunnel        `json:"tunnel"`
	Marketplace   Marketplace   `json:"marketplace"`
	MCP           MCP           `json:"mcp"`
}

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Provider is one model provider credential. An empty key is normal for
// providers that authenticate through a local OAuth file.
type Provider struct {
	ID  string `json:"id"`
	Key string `json:"key" secret:"true"`
}

// ModelRef points a capability slot at a concrete model.
type ModelRef struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Reasoning string `json:"reasoning"`
}

// The six capability slots of the original. An empty slot disables the
// capability with a *_MODEL_MISSING error rather than silently falling back.
const (
	SlotDefault      = "default"
	SlotSubconscious = "subconscious"
	SlotRealtime     = "realtime"
	SlotVoice        = "voice"
	SlotImage        = "image"
	SlotVideo        = "video"
)

// Slots lists the capability slots in presentation order.
var Slots = []string{SlotDefault, SlotSubconscious, SlotRealtime, SlotVoice, SlotImage, SlotVideo}

type Agents struct {
	Providers []Provider          `json:"providers"`
	Models    map[string]ModelRef `json:"models"`
}

// Region feeds the time_context block of the assembled prompt: the agent
// derives timezone conversions from the offset given, never from memory.
type Region struct {
	Language string `json:"language"`
	City     string `json:"city"`
	Country  string `json:"country"`
	Timezone string `json:"timezone"`
}

type General struct {
	PreventSleep bool `json:"preventSleep"`
}

type Notifications struct {
	Enabled bool `json:"enabled"`
}

type Security struct {
	Enabled  bool   `json:"enabled"` // default TRUE — ADR-0009
	Password string `json:"password,omitempty" secret:"true"`
	Secret   string `json:"secret" secret:"true"`
	APIToken string `json:"apiToken" secret:"true"`
}

// Telemetry is opt-in here, against the original's opt-out. A local-first
// product that promises privacy does not send anything without a yes.
type Telemetry struct {
	Enabled    bool   `json:"enabled"`
	Identifier string `json:"identifier,omitempty"`
}

type Tunnel struct {
	Enabled  bool   `json:"enabled"`
	Hostname string `json:"hostname"`
	Token    string `json:"token" secret:"true"`
	Provider string `json:"provider"`
}

type Marketplace struct {
	Registry string `json:"registry"`

	// Registries is every configured marketplace source, Git or HTTP-hosted —
	// see internal/domain/marketplace. A fresh install has none; discovery and
	// install both refuse cleanly with a CTA until at least one is added.
	Registries []MarketplaceRegistry `json:"registries,omitempty"`

	// RequireSignature is read by internal/domain/marketplace once package
	// signing exists (see that domain's design doc, "Sem publicação nesta
	// fase") — accepted here so a config file that already sets it round-trips
	// unchanged, not yet enforced anywhere.
	RequireSignature bool `json:"requireSignature,omitempty"`
}

// MarketplaceRegistry is one configured marketplace source.
type MarketplaceRegistry struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "git" | "http"
	URL  string `json:"url"`
}

// MCP selects the shape of the published tool surface (ADR-0011).
type MCP struct {
	ToolShape      string `json:"toolShape"`      // flat | composite | both
	SurfaceVersion string `json:"surfaceVersion"` // v1
}

// Tool shapes.
const (
	ToolShapeFlat      = "flat"
	ToolShapeComposite = "composite"
	ToolShapeBoth      = "both"
)

// Default returns the configuration a fresh installation starts with.
//
// Three defaults differ from the original on purpose: security.enabled is true
// (ADR-0009), telemetry.enabled is false (opt-in), and the tool shape is
// composite, which is where the ecosystem went (ADR-0011).
func Default() Config {
	return Config{
		Agents: Agents{
			Providers: []Provider{},
			Models:    map[string]ModelRef{},
		},
		Region:        Region{Language: "en-US", Timezone: "UTC"},
		General:       General{PreventSleep: true},
		Notifications: Notifications{Enabled: true},
		Security:      Security{Enabled: true},
		Telemetry:     Telemetry{Enabled: false},
		Tunnel:        Tunnel{Provider: "cloudflare"},
		MCP:           MCP{ToolShape: ToolShapeComposite, SurfaceVersion: "v1"},
	}
}

// Normalize fills in whatever a hand-edited or older file left out, without
// overwriting anything the user set. It is what makes a config written by an
// older version keep working after an upgrade.
func Normalize(c Config) Config {
	d := Default()
	if c.Agents.Models == nil {
		c.Agents.Models = map[string]ModelRef{}
	}
	if c.Agents.Providers == nil {
		c.Agents.Providers = []Provider{}
	}
	if c.Region.Language == "" {
		c.Region.Language = d.Region.Language
	}
	if c.Region.Timezone == "" {
		c.Region.Timezone = d.Region.Timezone
	}
	if c.Tunnel.Provider == "" {
		c.Tunnel.Provider = d.Tunnel.Provider
	}
	if c.MCP.ToolShape == "" {
		c.MCP.ToolShape = d.MCP.ToolShape
	}
	if c.MCP.SurfaceVersion == "" {
		c.MCP.SurfaceVersion = d.MCP.SurfaceVersion
	}
	return c
}
