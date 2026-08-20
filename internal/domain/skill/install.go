package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/view"
)

// Applier writes a verified, consented package into the workspace and
// returns the Skill record it produced.
//
// It is a port, not a private method of Installer, precisely so a test can
// replace the whole write path with a double and assert on Install's
// ordering and on its behaviour when applying fails partway — see
// skill_test.go. The default implementation, built by NewInstaller when Deps
// leaves it nil, is defaultApplier below.
//
// Apply is all-or-nothing: a failure at any step must leave the workspace as
// it found it, not just leave the skill's own record unwritten. Registering
// the skill last (see Install's own comment) only protects the skill's own
// registration; a collection created and registered before a later view
// failed is otherwise an orphan nothing owns — registered, on disk, and
// answering reads and writes as if it were real. defaultApplier below is
// what closes that.
type Applier interface {
	Apply(ctx context.Context, id string, pkg Package) (*Skill, error)
}

// Deps is what the Installer is built from.
type Deps struct {
	Fetcher  Fetcher
	Verifier Verifier // nil uses NewVerifier().
	Approver event.Approver

	Repo        Repository
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

	repo        Repository
	collections Collections
	views       Views
	hooks       Hooks
	toolsets    Toolsets
	clock       Clock
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
		repo: d.Repo, collections: d.Collections, views: d.Views, hooks: d.Hooks, toolsets: d.Toolsets,
		clock: d.Clock,
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
// something already checked, not to a promise. The collision check runs
// next, for the same reason: a person should not be asked to approve an
// install that is going to be refused anyway. Consent comes before Apply
// runs: nothing touches the workspace until a person has agreed, because an
// agent calling skills_install does not authorise itself (ADR-0007). Apply
// itself registers the skill's own record last, so a partial failure leaves
// an unregistered directory — something a person can delete — rather than a
// half-registered skill, which is something they cannot reason about; and
// Apply is itself all-or-nothing, so "partial failure" never means an
// orphaned collection or view survives it either — see Applier's own doc.
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

	// A collection whose name is already registered is refused, naming the
	// collision, rather than silently replacing it — collections.Registry.
	// Register replaces by design (a skill reinstalling over its own prior
	// registration depends on that), so the refusal belongs here, before
	// anything is written, not inside Register. Checked before every
	// collection the package declares, not just the first, so one install
	// either brings all of them or none of them.
	for _, c := range pkg.Collections {
		if i.collections.Exists(c.ID) {
			return nil, errCollectionNameTaken(c.ID)
		}
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
// about to disappear. The collections and views the skill brought are
// removed next, through their own domains — collection.Service.Delete and
// view.Service.Delete, addressed with this skill's id via DeleteInput.Skill,
// now that both can resolve a skill-scoped record (they could not when this
// package first shipped; see the report for Finding 2). Routing through
// them rather than only unregistering is what also takes the collection's
// and the view's own declaration files, not merely the in-memory
// registration a collection carries — Uninstall no longer has to trust that
// CascadeDelete alone will catch what the registry does not track. Removing
// the skill's own record is last, and — because the "skills" native carries
// CascadeDelete: true — is also what takes the rest of the directory with
// it: the agents, memories, routines, templates, instructions and goals
// this domain never touched directly.
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

	for _, ref := range skl.Metadata.Views {
		if err := i.views.Delete(ctx, view.DeleteInput{ID: ref.ID, Skill: id}); err != nil {
			return errUninstallFailed(id, "removing the view "+ref.ID, err)
		}
	}
	for _, ref := range skl.Metadata.Collections {
		if err := i.collections.Delete(ctx, collection.DeleteInput{ID: ref.ID, Skill: id}); err != nil {
			return errUninstallFailed(id, "removing the collection "+ref.ID, err)
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
//
// It tracks what it created as it goes and undoes it, in reverse order, the
// moment any step fails — files before views before collections, mirroring
// the order Apply wrote them in: files were the last thing placed before the
// skill's own record, views can name a collection as their source and
// nothing should be asked to remove a collection while a view still points
// at it, and collections come last because nothing else here depends on
// them. A rollback failure is not escalated over the original error: the
// original error is what the caller needs to act on, and a second failure
// while cleaning up after it is recorded for whoever reads the logs, not
// layered into the message that already explains what went wrong.
type defaultApplier struct {
	repo        Repository
	collections Collections
	views       Views
	files       Files
	clock       Clock
}

func (a *defaultApplier) Apply(ctx context.Context, id string, pkg Package) (*Skill, error) {
	var createdCollections, createdViews, writtenFiles []string
	rollback := func() {
		for i := len(writtenFiles) - 1; i >= 0; i-- {
			_ = a.files.Remove(ctx, id, writtenFiles[i])
		}
		for i := len(createdViews) - 1; i >= 0; i-- {
			_ = a.views.Delete(ctx, view.DeleteInput{ID: createdViews[i], Skill: id})
		}
		for i := len(createdCollections) - 1; i >= 0; i-- {
			_ = a.collections.Delete(ctx, collection.DeleteInput{ID: createdCollections[i], Skill: id})
		}
	}

	for _, in := range pkg.Collections {
		in.Scope = collection.ScopeSkill
		in.Skill = id
		if _, err := a.collections.Create(ctx, in); err != nil {
			rollback()
			return nil, errApplyFailed(id, "collections", err)
		}
		createdCollections = append(createdCollections, in.ID)
	}

	for _, in := range pkg.Views {
		in.Scope = "skill"
		in.Skill = id
		if _, err := a.views.Create(ctx, in); err != nil {
			rollback()
			return nil, errApplyFailed(id, "views", err)
		}
		createdViews = append(createdViews, in.ID)
	}

	files := make([]RawFile, 0, len(pkg.Files)+len(pkg.Toolsets))
	files = append(files, pkg.Files...)
	for _, ts := range pkg.Toolsets {
		files = append(files, ts.RawFile)
	}
	if len(files) > 0 {
		if err := a.files.Write(ctx, id, files); err != nil {
			rollback()
			return nil, errApplyFailed(id, "files", err)
		}
		// Recorded only now that Write has already returned success: Write
		// is expected to be all-or-nothing for its own batch (see Files'
		// doc comment), so nothing here is added to the undo list on a
		// failed Write, and nothing there is left for rollback to miss.
		for _, f := range files {
			writtenFiles = append(writtenFiles, f.Path)
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
	// reason about. A failure here still rolls back: without a SKILL.md,
	// CascadeDelete never fires for this id, so any collection or view
	// already created above would otherwise survive as exactly the orphan
	// this whole function exists to prevent.
	if err := a.repo.Create(ctx, skl); err != nil {
		rollback()
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
