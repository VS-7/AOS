package skill

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/view"
)

// Repository persists the Skill record itself: the SKILL.md at
// .aos/skills/{id}/SKILL.md. The "skills" native carries CascadeDelete: true
// (internal/core/collections/registry.go), so one Delete here removes the
// agents, memories, routines, collections, views and toolsets a skill
// brought along with it — the whole directory, not just the manifest file.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Skill, error)
	List(ctx context.Context, q collections.Query) ([]Skill, error)
	Create(ctx context.Context, v *Skill) error
	Delete(ctx context.Context, key collections.Key) error
}

// Collections is the slice of the collection domain this package needs. A
// skill's own collection is declared through it rather than written
// directly, so it gets exactly the schema validation and the record-pattern
// registration a hand-declared collection gets — Create both persists the
// declaration and registers the pattern its records live under, which is
// what makes an installed skill's collection usable in the same session, with
// no restart.
//
// Delete is here too, addressed with DeleteInput.Skill: collection.Service.Get
// and Delete originally resolved a collection by {"id": id} alone, which
// could not find a skill-scoped declaration once it lived under its own
// second, skill-qualified pattern. That gap is closed at the source now —
// GetInput and DeleteInput both take an optional Skill — so routing through
// collection.Service.Delete is the direct call rather than a workaround: one
// call removes both the in-memory registration and the declaration file,
// instead of this package unregistering the pattern by hand and trusting
// CascadeDelete to catch the file. Apply's own rollback (see defaultApplier)
// is the other reason Delete has to be here: undoing a collection this
// package just created, because a later step in the same install failed,
// needs exactly this method.
type Collections interface {
	Create(ctx context.Context, in collection.CreateInput) (*collection.Collection, error)
	Delete(ctx context.Context, in collection.DeleteInput) error
}

// Views is the slice of the view domain this package needs, for the same
// reason Collections is: Create validates a view's tree against its source
// collection and against the command registry before anything is written, a
// check this package has no business re-deriving. Delete is here for the
// same two reasons Collections.Delete is — Uninstall removing what the skill
// brought, and Apply's rollback undoing what it just created.
type Views interface {
	Create(ctx context.Context, in view.CreateInput) (*view.View, error)
	Get(ctx context.Context, in view.GetInput) (*view.View, error)
	Delete(ctx context.Context, in view.DeleteInput) error
}

// RawFile is one piece of a package's content this domain does not itself
// interpret: an agent's AGENT.md and its memories and routines, a template,
// an instruction, a goal, a reference, or a toolset's declaration. Path is
// relative to the skill's own root ("agents/closer/AGENT.md") and becomes the
// same relative path under the installed skill's directory — the native
// collections that read agents, memories, routines, templates, instructions
// and goals already know their own second, skill-scoped pattern (registered
// once, in Phase 1); relocating the bytes is all installing one requires.
type RawFile struct {
	Path    string
	Content []byte
}

// Files places a package's raw content on disk. It is a port because
// internal/domain may not import os: this domain decides what belongs under
// a skill's directory and in what order it is written; an adapter decides how
// a byte slice becomes a file.
//
// Write is expected to be all-or-nothing for the batch it is given: Apply's
// rollback only records a file as written once Write has already returned
// success for it, so a Write that fails partway through its own batch and
// leaves some of it on disk is a contract violation this package has no way
// to detect or undo.
//
// Remove is the undo Apply's rollback needs for everything Write placed —
// an agent (with its memories and routines nested inside), a template, an
// instruction, a goal, a reference, a toolset's declaration. It is
// addressed the same way Write's files are: skillID plus the file's own
// skill-relative Path, called once per file, in reverse of the order Write
// received them — the same shape Collections.Delete and Views.Delete undo
// what Create brought into being with.
type Files interface {
	Write(ctx context.Context, skillID string, files []RawFile) error
	Remove(ctx context.Context, skillID, path string) error
}

// Hooks deregisters the handlers a skill's own hooks became when it was
// installed. Uninstall calls it before the skill's files are removed, so
// nothing keeps intercepting a tool call on behalf of a directory that no
// longer exists.
type Hooks interface {
	Deregister(ctx context.Context, skillID string) error
}

// Toolsets closes every toolset connection a skill's declarations point at,
// during Uninstall, for the same reason Hooks does. One call rather than one
// per configured toolset: this domain does not track which of a skill's
// declared toolsets survived an earlier partial uninstall, and the boundary
// this closes is "is anything of this skill's still reachable", not an
// enumeration kept in sync here.
type Toolsets interface {
	Close(ctx context.Context, skillID string) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }

// Manifest is the parsed front matter of SKILL.md: the identity fields an
// install carries onto the Skill record, plus the permissions ADR-0015
// requires a package to declare.
type Manifest struct {
	Name        string
	Description string
	Version     string
	Source      string
	Permissions Permissions
}

// ToolsetDecl is one toolset a package declares: both what VerifyManifest
// checks (Command against Permissions.Exec, the host of BaseURL against
// Permissions.Network) and what Apply writes, unmodified, as RawFile.
type ToolsetDecl struct {
	ID      string
	Type    string
	Command string
	BaseURL string
	RawFile RawFile
}

// Package is a skill's content, read but not yet applied. Fetch produces it;
// VerifyManifest checks it against Manifest.Permissions; only a consented
// Install ever writes it anywhere.
type Package struct {
	Manifest Manifest
	Content  string // the SKILL.md body

	// Collections and Views decode into the exact inputs their own domains
	// validate: a skill's collections/{id}/schema.json or
	// views/{id}.view.json is a collection.CreateInput or a
	// view.CreateInput on disk, so Apply hands them to collection.Service
	// and view.Service unchanged instead of re-deriving what those services
	// already check.
	Collections []collection.CreateInput
	Views       []view.CreateInput
	Toolsets    []ToolsetDecl

	// Files is everything else the package brings — agents (with their
	// memories and routines), templates, instructions, goals, references —
	// relocated byte for byte under the installed skill's directory.
	Files []RawFile
}

// Fetcher retrieves a skill package from a source: a local directory today
// (skillfetch.Local), a registry address later. ref selects a version — a git
// ref, a tag — and a fetcher with no such concept, like the local one,
// refuses a non-empty one rather than silently ignoring it.
type Fetcher interface {
	Fetch(ctx context.Context, source, ref string) (Package, error)
}

// Diff is what VerifyManifest found: the manifest's own permissions, ready to
// render into the approval request a human reads before Install writes
// anything.
type Diff struct {
	Permissions Permissions
}

// Render is the text a person reads in the approval prompt: one line per
// non-empty permission category. It is deliberately plain — a diff a human
// has to parse before deciding is a diff they will stop reading.
func (d Diff) Render() string {
	var lines []string
	add := func(label string, items []string) {
		if len(items) > 0 {
			lines = append(lines, label+": "+strings.Join(items, ", "))
		}
	}
	add("hooks", d.Permissions.Hooks)
	add("exec", d.Permissions.Exec)
	add("network", d.Permissions.Network)
	add("agents", d.Permissions.Agents)
	add("collections", d.Permissions.Collections)
	if d.Permissions.Routines > 0 {
		lines = append(lines, "routines: "+strconv.Itoa(d.Permissions.Routines))
	}
	if len(d.Permissions.Toolsets) > 0 {
		items := make([]string, len(d.Permissions.Toolsets))
		for i, t := range d.Permissions.Toolsets {
			items[i] = fmt.Sprintf("%s(%s%s)", t.Type, t.Command, t.BaseURL)
		}
		lines = append(lines, "toolsets: "+strings.Join(items, ", "))
	}
	if len(lines) == 0 {
		return "this skill declares no permissions"
	}
	return strings.Join(lines, "\n")
}

// Verifier checks a package's content against its own declared manifest.
// Content that exceeds it is refused, naming the excess — see ADR-0015.
type Verifier interface {
	VerifyManifest(pkg Package) (Diff, error)
}
