// Package artifact serves static web applications a skill or an agent
// publishes — dashboards, reports, landing pages — from the daemon itself,
// with no deploy step. See docs/04 - Domínio/Artifact (Go).md.
package artifact

import "time"

// Visibility controls who may read an artifact's files.
type Visibility string

const (
	// Private is reachable only by an authenticated member of the workspace
	// that owns it.
	Private Visibility = "private"

	// Workspace is reachable by any authenticated member of the workspace.
	Workspace Visibility = "workspace"

	// ByPassword is reachable by anyone holding the password set on the
	// artifact — the mechanism for sharing a link outside the workspace.
	ByPassword Visibility = "by_password"
)

// visibilities lists every member of the union, in declaration order.
var visibilities = []Visibility{Private, Workspace, ByPassword}

// Valid reports whether v is one of the three declared visibilities.
func (v Visibility) Valid() bool {
	for _, known := range visibilities {
		if v == known {
			return true
		}
	}
	return false
}

// Artifact is one static web application, registered in a workspace and
// served by the daemon at /v/{workspace}/artifacts/{id}/*.
type Artifact struct {
	// ID identifies this artifact. It lives in the path, exactly like every
	// other native collection record: .aos/artifacts/{id}.artifact.md.
	ID string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this artifact, used in its serving URL."`

	Name        string `yaml:"name" json:"name" jsonschema:"Human-readable name."`
	Description string `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"What this artifact is, read by whoever decides whether to open it."`

	// Entrypoint is the file served at the artifact's root — relative to the
	// artifact's own directory, resolved and guarded against traversal by the
	// transport. Create scaffolds a minimal one when none is given.
	Entrypoint string `yaml:"entrypoint" json:"entrypoint" jsonschema:"HTML file served as the artifact's root, relative to its own directory."`

	Visibility Visibility `yaml:"visibility" json:"visibility" jsonschema:"One of: private, workspace, by_password."`

	// Skill names the skill that owns this artifact, when it was installed by
	// one — empty for an artifact created directly in the workspace.
	Skill string `yaml:"skill,omitempty" json:"skill,omitempty" jsonschema:"Skill that owns this artifact, if any."`

	// PasswordHash replaces the original's process-derived password, which
	// was generated fresh on every boot and never persisted — a shared link
	// with visibility by_password stopped working after any restart. Here the
	// hash is written to disk, so the link survives one. Fixes defect #19.
	PasswordHash string `yaml:"passwordHash,omitempty" json:"-"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When this artifact was created."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last changed."`

	// Content is unused by Artifact itself — an artifact is a directory of
	// files, not one Markdown body — but the collection tag is required by
	// the frontmatter+body model every native record is stored as. It always
	// reads back empty and is never written.
	Content string `yaml:"-" json:"-" collection:"content"`
}

// Clone returns an Artifact that shares no mutable state with a. Nothing in
// this struct is a slice or map today, but every sibling domain's entity
// carries its own Clone, and a struct that grows one later is safer for
// already calling it everywhere a value crosses a boundary.
func (a Artifact) Clone() Artifact { return a }
