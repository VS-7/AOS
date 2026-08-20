// Package skill is an installable package of capability.
//
// A skill is not documentation: it brings agents, memories, routines,
// collections, views, hooks and instructions together, and installing one
// installs a team. The persistence for all of that already exists —
// internal/core/collections/registry.go declares the native "skills" record
// plus the second patterns that let a skill ship its own agents, memories,
// routines, collections, views and toolsets. This package is the domain over
// that storage: what a package is allowed to contain (ADR-0015), who has to
// agree before it is written, and in what order the writing happens.
package skill

import "time"

// Skill is one installed package: the identity fields an install carries
// forward from its manifest, the permissions it was verified and consented
// to, and the inventory of what it actually brought.
type Skill struct {
	// ID is the skill's own directory name: .aos/skills/{id}/SKILL.md. It
	// comes from the manifest's name, slugified, not from the front matter
	// verbatim — the same reasoning collection.DescriptorFor applies to a
	// collection's id: a directory name is not free text.
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this skill. Also its directory name, so lowercase, digits, hyphen and underscore only."`

	Name        string `yaml:"name" json:"name" jsonschema:"Human name of the skill."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this skill is for."`

	// Active gates a skill's hooks and toolsets without removing it: turning
	// a skill off keeps its agents, memories and configuration in place while
	// its live behaviour stops.
	Active bool `yaml:"active" json:"active" jsonschema:"Whether this skill's hooks and toolsets are live. False keeps it installed but inert."`

	Version string `yaml:"version,omitempty" json:"version,omitempty" jsonschema:"Semver of the installed package, from its manifest."`

	// Source and Commit are provenance: where the package came from and at
	// what commit. A local directory has neither — there is nothing to
	// record — and Install leaves both empty rather than inventing a value,
	// which would claim a provenance that does not exist.
	Source string `yaml:"source,omitempty" json:"source,omitempty" jsonschema:"Where the package came from, \"owner/repo\" for a registry source. Empty for a local directory."`
	Commit string `yaml:"commit,omitempty" json:"commit,omitempty" jsonschema:"Commit the package was fetched at. Empty for a local directory, for the same reason Source is."`

	// Permissions is the manifest this skill was verified and consented to —
	// see ADR-0015. Content beyond it is refused at install, never trimmed
	// and installed anyway.
	Permissions Permissions `yaml:"permissions" json:"permissions" jsonschema:"The manifest this skill was verified and consented to."`

	// Metadata is what the install actually found and applied: the ids of the
	// collections, views and toolsets this skill brought, so Uninstall knows
	// what to take away again without re-reading the package.
	Metadata Metadata `yaml:"metadata" json:"metadata" jsonschema:"Inventory of what this skill actually installed."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When this skill was installed."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`

	// Content is the SKILL.md body: what the skill is and how to use it, read
	// by an agent deciding whether to reach for it.
	Content string `yaml:"-" json:"content" collection:"content" jsonschema:"The SKILL.md body: what the skill is and how to use it."`
}

// Clone returns a Skill that shares no mutable state with s.
//
// This codebase has shipped the same defect four times — WithoutBody,
// Catalog, recordFrom and a view's Source.Filter — a map or slice crossing an
// in-memory boundary by reference, so a caller's edit reaches whatever the
// service or its repository is still holding underneath it. Permissions and
// Metadata are both full of slices; every place this package hands a Skill
// out or takes one in calls Clone at the boundary rather than trusting a
// repository — fake or real — not to alias it.
func (s Skill) Clone() Skill {
	c := s
	c.Permissions = s.Permissions.clone()
	c.Metadata = s.Metadata.clone()
	return c
}

// Permissions is the manifest a package declares: what it intercepts, what it
// runs, what it reaches, and what it installs. Content the package ships that
// exceeds this is refused at install — see ADR-0015 and Verifier.
type Permissions struct {
	Hooks       []string      `yaml:"hooks,omitempty" json:"hooks,omitempty" jsonschema:"Event types this skill's hooks intercept."`
	Toolsets    []ToolsetPerm `yaml:"toolsets,omitempty" json:"toolsets,omitempty" jsonschema:"Toolset connections this skill declares."`
	Exec        []string      `yaml:"exec,omitempty" json:"exec,omitempty" jsonschema:"Binaries this skill's toolsets may run. Enters the sandbox allowlist on install."`
	Network     []string      `yaml:"network,omitempty" json:"network,omitempty" jsonschema:"Hosts this skill's toolsets may reach."`
	Agents      []string      `yaml:"agents,omitempty" json:"agents,omitempty" jsonschema:"Ids of the agents this skill installs."`
	Routines    int           `yaml:"routines,omitempty" json:"routines,omitempty" jsonschema:"Number of autonomous routines this skill installs."`
	Collections []string      `yaml:"collections,omitempty" json:"collections,omitempty" jsonschema:"Ids of the collections this skill declares."`
}

func (p Permissions) clone() Permissions {
	return Permissions{
		Hooks:       cloneStrings(p.Hooks),
		Toolsets:    cloneToolsetPerms(p.Toolsets),
		Exec:        cloneStrings(p.Exec),
		Network:     cloneStrings(p.Network),
		Agents:      cloneStrings(p.Agents),
		Routines:    p.Routines,
		Collections: cloneStrings(p.Collections),
	}
}

// ToolsetPerm is one connection a package's manifest allows: an exact binary
// or base URL, never a wildcard — ADR-0015 is explicit that the check is the
// installer's, not the author's word for it.
type ToolsetPerm struct {
	Type    string `yaml:"type" json:"type" jsonschema:"Connection type this permission covers: one of the five toolset.Type values."`
	Command string `yaml:"command,omitempty" json:"command,omitempty" jsonschema:"Exact binary this permission allows for mcp-server::stdio or cli. Never a wildcard."`
	BaseURL string `yaml:"baseUrl,omitempty" json:"baseUrl,omitempty" jsonschema:"Base URL this permission allows for mcp-server::http or rest-api."`
}

// Metadata is the inventory an install actually produced: not what the
// manifest asked for, but what got applied, so Uninstall can take exactly
// that away again.
type Metadata struct {
	Rules        []Rule       `yaml:"rules,omitempty" json:"rules,omitempty" jsonschema:"Declarative constraints this skill carries as a whole."`
	Resources    []Resource   `yaml:"resources,omitempty" json:"resources,omitempty" jsonschema:"Reference material this skill ships."`
	Toolsets     []ToolsetRef `yaml:"toolsets,omitempty" json:"toolsets,omitempty" jsonschema:"Toolsets this skill installed."`
	Collections  []Ref        `yaml:"collections,omitempty" json:"collections,omitempty" jsonschema:"Collections this skill installed."`
	Views        []Ref        `yaml:"views,omitempty" json:"views,omitempty" jsonschema:"Views this skill installed."`
	Hooks        []Ref        `yaml:"hooks,omitempty" json:"hooks,omitempty" jsonschema:"Hooks this skill registered."`
	Artifacts    []Ref        `yaml:"artifacts,omitempty" json:"artifacts,omitempty" jsonschema:"Artifacts this skill installed."`
	Goals        []Ref        `yaml:"goals,omitempty" json:"goals,omitempty" jsonschema:"Goals this skill installed."`
	Templates    []Ref        `yaml:"templates,omitempty" json:"templates,omitempty" jsonschema:"Templates this skill installed."`
	Instructions []Ref        `yaml:"instructions,omitempty" json:"instructions,omitempty" jsonschema:"Instructions this skill installed."`
}

func (m Metadata) clone() Metadata {
	return Metadata{
		Rules:        cloneRules(m.Rules),
		Resources:    cloneResources(m.Resources),
		Toolsets:     cloneToolsetRefs(m.Toolsets),
		Collections:  cloneRefs(m.Collections),
		Views:        cloneRefs(m.Views),
		Hooks:        cloneRefs(m.Hooks),
		Artifacts:    cloneRefs(m.Artifacts),
		Goals:        cloneRefs(m.Goals),
		Templates:    cloneRefs(m.Templates),
		Instructions: cloneRefs(m.Instructions),
	}
}

// Ref names one item of a skill's inventory by the id it is addressed by on
// its own domain — a collection id, a view id.
type Ref struct {
	ID string `yaml:"id" json:"id" jsonschema:"Identifier of the item, as it is addressed on its own domain."`
}

// ToolsetRef names one toolset a skill installed.
type ToolsetRef struct {
	ID   string `yaml:"id" json:"id" jsonschema:"Identifier of the toolset."`
	Type string `yaml:"type" json:"type" jsonschema:"Connection type: one of the five toolset.Type values."`
}

// Resource is reference material a skill ships: content a hook or an agent
// can load on demand, or — when Always is set — content loaded into context
// permanently the moment the skill is active.
type Resource struct {
	URI         string `yaml:"uri" json:"uri" jsonschema:"Where this resource's content is: a path relative to the skill's own directory."`
	MimeType    string `yaml:"mimeType,omitempty" json:"mimeType,omitempty" jsonschema:"MIME type of the resource's content."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this resource contains, for whoever decides whether to load it."`
	Always      bool   `yaml:"always,omitempty" json:"always,omitempty" jsonschema:"Whether this resource is loaded into context permanently rather than on demand."`
	Rules       []Rule `yaml:"rules,omitempty" json:"rules,omitempty" jsonschema:"Declarative constraints attached to this resource."`
}

// Rule is a small declarative constraint a resource, or the skill as a
// whole, carries. Nothing in this build interprets one yet — a skill's hooks
// are themselves declarative (ADR-0015), and a Rule is the same idea applied
// to a resource: a fact recorded, not code executed.
type Rule struct {
	ID      string `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"Identifier of this rule, when it needs to be referenced."`
	Content string `yaml:"content" json:"content" jsonschema:"The rule text."`
}

func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func cloneToolsetPerms(s []ToolsetPerm) []ToolsetPerm {
	if s == nil {
		return nil
	}
	out := make([]ToolsetPerm, len(s))
	copy(out, s)
	return out
}

func cloneToolsetRefs(s []ToolsetRef) []ToolsetRef {
	if s == nil {
		return nil
	}
	out := make([]ToolsetRef, len(s))
	copy(out, s)
	return out
}

func cloneRefs(s []Ref) []Ref {
	if s == nil {
		return nil
	}
	out := make([]Ref, len(s))
	copy(out, s)
	return out
}

func cloneRules(s []Rule) []Rule {
	if s == nil {
		return nil
	}
	out := make([]Rule, len(s))
	copy(out, s)
	return out
}

func cloneResources(s []Resource) []Resource {
	if s == nil {
		return nil
	}
	out := make([]Resource, len(s))
	for i, r := range s {
		r.Rules = cloneRules(r.Rules)
		out[i] = r
	}
	return out
}
