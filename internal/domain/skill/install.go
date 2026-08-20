package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/event"
)

// Applier writes a verified, consented package into the workspace and
// returns the Skill record it produced.
//
// It is a port, not a private method of Installer, precisely so a test can
// replace the whole write path with a double and assert on Install's
// ordering and on its behaviour when applying fails partway — see
// skill_test.go. The default implementation, built by NewInstaller when Deps
// leaves it nil, is defaultApplier below.
type Applier interface {
	Apply(ctx context.Context, id string, pkg Package) (*Skill, error)
}

// Deps is what the Installer is built from.
type Deps struct {
	Fetcher  Fetcher
	Verifier Verifier // nil uses NewVerifier().
	Approver event.Approver

	Repo        Repository
	Registry    Registry
	Collections Collections
	Views       Views
	Files       Files
	Hooks       Hooks
	Toolsets    Toolsets

	// Applier overrides the default write path. Nil composes one from Repo,
	// Collections, Views, Files and Clock — see defaultApplier.
	Applier Applier

	Clock Clock
}

// Installer is the skill aggregate: Install and Uninstall, plus the read
// surface (Get, List, Views) a caller needs once a skill is in place.
type Installer struct {
	fetcher  Fetcher
	verifier Verifier
	approver event.Approver
	applier  Applier

	repo     Repository
	registry Registry
	views    Views
	hooks    Hooks
	toolsets Toolsets
}

// NewInstaller wires the installer over its ports.
func NewInstaller(d Deps) *Installer {
	verifier := d.Verifier
	if verifier == nil {
		verifier = NewVerifier()
	}
	applier := d.Applier
	if applier == nil {
		applier = &defaultApplier{
			repo: d.Repo, collections: d.Collections, views: d.Views, files: d.Files, clock: d.Clock,
		}
	}
	return &Installer{
		fetcher: d.Fetcher, verifier: verifier, approver: d.Approver, applier: applier,
		repo: d.Repo, registry: d.Registry, views: d.Views, hooks: d.Hooks, toolsets: d.Toolsets,
	}
}

// InstallInput names the package to install and how consent was obtained.
type InstallInput struct {
	Source string
	Ref    string

	// AcceptedAll is how a caller that already has consent — a CLI run with
	// --yes, or a test — says so. nil means "ask": a func field rather than a
	// method on this type keeps that decision at the call site instead of
	// inside the input, which is what lets the same InstallInput be built by
	// a command handler that never has an answer to give and by a test that
	// always does.
	//
	// This is the one place ADR-0007 is not optional: an agent calling
	// skills_install does not authorise itself, and a nil AcceptedAll is what
	// keeps the default "ask".
	AcceptedAll func(Permissions) bool
}

// UninstallInput names the skill to remove.
type UninstallInput struct {
	ID string
}

// Install fetches, verifies and applies a skill package.
//
// The order is the decision, not an implementation detail. Fetch reads the
// package; VerifyManifest checks its content against what it declared,
// before anyone is asked to approve anything — a human should consent to
// something already checked, not to a promise. Consent comes before Apply
// runs: nothing touches the workspace until a person has agreed, because an
// agent calling skills_install does not authorise itself (ADR-0007). Apply
// itself registers the skill's own record last, so a partial failure leaves
// an unregistered directory — something a person can delete — rather than a
// half-registered skill, which is something they cannot reason about.
func (i *Installer) Install(ctx context.Context, in InstallInput) (*Skill, error) {
	source := strings.TrimSpace(in.Source)
	if source == "" {
		return nil, errSourceRequired()
	}

	pkg, err := i.fetcher.Fetch(ctx, source, in.Ref)
	if err != nil {
		return nil, err
	}

	diff, err := i.verifier.VerifyManifest(pkg)
	if err != nil {
		return nil, err
	}

	if in.AcceptedAll == nil || !in.AcceptedAll(diff.Permissions) {
		res, err := i.approver.RequestApproval(ctx, event.ApprovalRequest{
			ToolName: "skills_install",
			Risk:     event.RiskHigh,
			Reason:   diff.Render(),
		})
		if err != nil {
			return nil, err
		}
		if !res.Approved {
			return nil, errInstallNotApproved(source, res.Reason)
		}
	}

	id, err := skillID(pkg.Manifest)
	if err != nil {
		return nil, err
	}

	return i.applier.Apply(ctx, id, pkg)
}

// Uninstall removes an installed skill.
//
// The order inverts Install's, for the reason Install's own comment gives in
// reverse: hooks and toolsets are torn down first, so nothing keeps
// intercepting a tool call or holding a connection on behalf of a directory
// about to disappear. The collections the skill brought are unregistered
// next — their files go with the skill's directory either way, but nothing
// else clears the in-memory registry a dynamic collection lives in, and
// unregistering after the files were removed would leave a window where the
// registry still resolves a name nothing on disk answers to. Views carry no
// such registry, so there is nothing to unregister for them. Removing the
// skill's own record is last, and — because the "skills" native carries
// CascadeDelete: true — is also what takes the rest of the directory with
// it: the agents, memories, routines, templates, instructions, goals and
// views this domain never touched directly.
func (i *Installer) Uninstall(ctx context.Context, in UninstallInput) error {
	id := strings.TrimSpace(in.ID)
	skl, err := i.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := i.hooks.Deregister(ctx, id); err != nil {
		return errUninstallFailed(id, "deregistering hooks", err)
	}
	if err := i.toolsets.Close(ctx, id); err != nil {
		return errUninstallFailed(id, "closing toolsets", err)
	}

	for _, ref := range skl.Metadata.Collections {
		if err := i.registry.Unregister(ref.ID); err != nil {
			return errUninstallFailed(id, "unregistering the collection "+ref.ID, err)
		}
	}

	return i.repo.Delete(ctx, collections.Key{"id": id})
}

// skillID derives a skill's directory name from its manifest: the declared
// name, lowercased, validated the same way collection.DescriptorFor validates
// a collection's own id — a directory name is not free text, and a manifest
// that names something unsafe is refused before Apply ever runs.
func skillID(m Manifest) (string, error) {
	id := strings.ToLower(strings.TrimSpace(m.Name))
	if id == "" {
		return "", errManifestInvalid("the manifest names no skill")
	}
	for _, r := range id {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return "", errManifestInvalid(fmt.Sprintf("the skill name %q is not usable as a directory name", id))
		}
	}
	return id, nil
}

// defaultApplier is the real write path: a skill's own collections and views
// go through their own domains, for the validation only those domains carry;
// everything else — agents, memories, routines, templates, instructions,
// goals, references, and each toolset's own declaration — is relocated byte
// for byte through Files. The skill's own record is written last.
type defaultApplier struct {
	repo        Repository
	collections Collections
	views       Views
	files       Files
	clock       Clock
}

func (a *defaultApplier) Apply(ctx context.Context, id string, pkg Package) (*Skill, error) {
	for _, in := range pkg.Collections {
		in.Scope = collection.ScopeSkill
		in.Skill = id
		if _, err := a.collections.Create(ctx, in); err != nil {
			return nil, errApplyFailed(id, "collections", err)
		}
	}

	for _, in := range pkg.Views {
		in.Scope = "skill"
		in.Skill = id
		if _, err := a.views.Create(ctx, in); err != nil {
			return nil, errApplyFailed(id, "views", err)
		}
	}

	files := make([]RawFile, 0, len(pkg.Files)+len(pkg.Toolsets))
	files = append(files, pkg.Files...)
	for _, ts := range pkg.Toolsets {
		files = append(files, ts.RawFile)
	}
	if len(files) > 0 {
		if err := a.files.Write(ctx, id, files); err != nil {
			return nil, errApplyFailed(id, "files", err)
		}
	}

	now := a.clock.Now()
	skl := &Skill{
		ID:          id,
		Name:        pkg.Manifest.Name,
		Description: pkg.Manifest.Description,
		Active:      true,
		Version:     pkg.Manifest.Version,
		Source:      pkg.Manifest.Source,
		Permissions: pkg.Manifest.Permissions,
		Metadata:    metadataOf(pkg),
		CreatedAt:   now,
		UpdatedAt:   now,
		Content:     pkg.Content,
	}
	// Registered last, deliberately: a partial failure above leaves an
	// unregistered directory rather than a half-registered skill — the first
	// is something a person can delete, the second is something they cannot
	// reason about.
	if err := a.repo.Create(ctx, skl); err != nil {
		return nil, errApplyFailed(id, "skill", err)
	}
	out := skl.Clone()
	return &out, nil
}

// metadataOf builds the inventory Uninstall reads back: exactly what this
// install applied, not what the manifest merely asked for.
func metadataOf(pkg Package) Metadata {
	m := Metadata{}
	for _, c := range pkg.Collections {
		m.Collections = append(m.Collections, Ref{ID: c.ID})
	}
	for _, v := range pkg.Views {
		m.Views = append(m.Views, Ref{ID: v.ID})
	}
	for _, ts := range pkg.Toolsets {
		m.Toolsets = append(m.Toolsets, ToolsetRef{ID: ts.ID, Type: ts.Type})
	}
	return m
}
