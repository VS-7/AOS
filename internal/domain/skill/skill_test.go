package skill_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/skillfetch"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/view"
)

func ctx() context.Context { return context.Background() }

var refTime = time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

// fixtureDir points at the crm-skill fixture built for this task, under
// internal/app/testdata — not under this package's own testdata, so that
// Task 11's delivery test, which runs from internal/app, uses the exact same
// directory rather than a second copy of it.
func fixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "app", "testdata", "crm-skill")
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not *apperr.Error: %v", err, err)
	}
	return app.Code
}

// acceptAll is the AcceptedAll a caller supplies when it already has
// consent — a CLI run with --yes, or a test that is not exercising the
// approval channel itself.
func acceptAll(skill.Permissions) bool { return true }

// ---------------------------------------------------------------------------
// Building a Package without touching a filesystem: what the manifest-
// refusal tests exercise. VerifyManifest is pure, so these need no adapter.
// ---------------------------------------------------------------------------

func packageWith(perm skill.Permissions, opts ...func(*skill.Package)) skill.Package {
	pkg := skill.Package{Manifest: skill.Manifest{Name: "test", Permissions: perm}}
	for _, opt := range opts {
		opt(&pkg)
	}
	return pkg
}

func withCollection(id string) func(*skill.Package) {
	return func(p *skill.Package) {
		p.Collections = append(p.Collections, collection.CreateInput{
			ID: id, Format: collection.FormatMarkdown,
			Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
		})
	}
}

func withToolsetCommand(cmd string) func(*skill.Package) {
	return func(p *skill.Package) {
		p.Toolsets = append(p.Toolsets, skill.ToolsetDecl{ID: "t", Type: "cli", Command: cmd})
	}
}

func withToolsetURL(u string) func(*skill.Package) {
	return func(p *skill.Package) {
		p.Toolsets = append(p.Toolsets, skill.ToolsetDecl{ID: "t", Type: "mcp-server::http", BaseURL: u})
	}
}

func withAgent(id string) func(*skill.Package) {
	return func(p *skill.Package) {
		p.Files = append(p.Files, skill.RawFile{Path: "agents/" + id + "/AGENT.md", Content: []byte("---\nname: x\n---\n")})
	}
}

// The manifest is not documentation: content that exceeds it is refused, and
// the refusal names the excess. A package that declared no exec permission
// and ships a toolset that runs a binary is exactly the case this closes.
func TestContentExceedingTheManifestIsRefusedNamingTheExcess(t *testing.T) {
	pkg := packageWith(
		skill.Permissions{Collections: []string{"contacts"}},
		withCollection("contacts"),
		withCollection("deals"), // not declared
	)
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_SKILL_MANIFEST_EXCEEDED" {
		t.Fatalf("code = %q", app.Code)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "deals") {
		t.Fatalf("the refusal does not name the excess: %v", app.Issues)
	}
}

// The package ships an mcp-server::stdio-shaped toolset that spawns `curl`,
// and declares no exec permission for it. Two doors, both closed by default:
// the binary has to be in the agent's sandbox allowlist *and* declared in the
// skill's manifest.
func TestAToolsetRunningAnUndeclaredBinaryIsRefused(t *testing.T) {
	pkg := packageWith(
		skill.Permissions{Exec: []string{"gh"}},
		withToolsetCommand("curl"),
	)
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_SKILL_MANIFEST_EXCEEDED" {
		t.Fatalf("code = %q", app.Code)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "curl") {
		t.Fatalf("the refusal does not name the binary: %v", app.Issues)
	}
}

func TestAHostOutsideThePermissionsNetworkListIsRefused(t *testing.T) {
	pkg := packageWith(
		skill.Permissions{Network: []string{"api.github.com"}},
		withToolsetURL("https://evil.example.com/mcp"),
	)
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "evil.example.com") {
		t.Fatalf("the refusal does not name the host: %v", app.Issues)
	}
}

// An agent the package brings but the manifest never named is excess too:
// Permissions.Agents is the third door ADR-0015 closes.
func TestAnUndeclaredAgentIsRefused(t *testing.T) {
	pkg := packageWith(skill.Permissions{Agents: []string{"closer"}}, withAgent("shadow"))
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "shadow") {
		t.Fatalf("the refusal does not name the agent: %v", app.Issues)
	}
}

// The mirror of the refusal tests: a package that ships nothing beyond what
// its manifest declares is accepted, and the Diff it returns carries the
// manifest's own permissions forward for the approval prompt.
func TestContentFullyDeclaredByTheManifestIsAccepted(t *testing.T) {
	pkg := packageWith(
		skill.Permissions{Collections: []string{"contacts"}, Agents: []string{"closer"}, Exec: []string{"gh"}},
		withCollection("contacts"),
		withAgent("closer"),
		withToolsetCommand("gh"),
	)
	diff, err := skill.NewVerifier().VerifyManifest(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Permissions.Collections) != 1 || diff.Permissions.Collections[0] != "contacts" {
		t.Fatalf("diff.Permissions = %+v", diff.Permissions)
	}
}

func TestDiffRenderListsEachDeclaredCategory(t *testing.T) {
	d := skill.Diff{Permissions: skill.Permissions{
		Agents: []string{"closer"}, Collections: []string{"contacts"},
	}}
	out := d.Render()
	if !strings.Contains(out, "agents: closer") {
		t.Fatalf("render = %q, missing agents", out)
	}
	if !strings.Contains(out, "collections: contacts") {
		t.Fatalf("render = %q, missing collections", out)
	}
}

func TestDiffRenderWithNoPermissionsSaysSo(t *testing.T) {
	if got := (skill.Diff{}).Render(); got != "this skill declares no permissions" {
		t.Fatalf("render = %q", got)
	}
}

// Routines and toolsets are rendered too — the two categories that are not a
// plain string list.
func TestDiffRenderIncludesRoutinesAndToolsets(t *testing.T) {
	d := skill.Diff{Permissions: skill.Permissions{
		Routines: 2,
		Toolsets: []skill.ToolsetPerm{{Type: "cli", Command: "gh"}},
	}}
	out := d.Render()
	if !strings.Contains(out, "routines: 2") {
		t.Fatalf("render = %q, missing routines", out)
	}
	if !strings.Contains(out, "toolsets: cli(gh)") {
		t.Fatalf("render = %q, missing toolsets", out)
	}
}

// A toolset whose base URL does not even parse as a URL is not flagged as
// reaching an undeclared host: there is no host to compare, and Adapter.Connect
// is where a genuinely broken URL is refused, not here.
func TestAToolsetWithAnUnparsableURLIsNotFlaggedAsExcess(t *testing.T) {
	pkg := packageWith(skill.Permissions{Network: []string{"api.github.com"}}, withToolsetURL("://not a url"))
	if _, err := skill.NewVerifier().VerifyManifest(pkg); err != nil {
		t.Fatalf("an unparsable URL should not be treated as an undeclared host: %v", err)
	}
}

// ---------------------------------------------------------------------------
// A harness for Install/Uninstall: an Installer wired over in-memory fakes,
// with the real local fetcher pointed at the committed crm-skill fixture.
// ---------------------------------------------------------------------------

// newInstaller builds an Installer with sensible defaults, overridable one
// dependency at a time. reg is the collections.Registry the default
// Collections dependency (a real collection.Service) registers into — nil
// builds a fresh one — passed separately, rather than through an option,
// because it has to exist before the default Collections dependency is
// built, and an option applied afterward would be too late to matter.
func newInstaller(t *testing.T, reg *collections.Registry, opts ...func(*skill.Deps)) (*skill.Installer, *fakeFiles) {
	t.Helper()
	if reg == nil {
		reg = collections.NewRegistry()
	}
	clock := clockx.Fixed{At: refTime}
	files := newFakeFiles()

	d := skill.Deps{
		Fetcher:  skillfetch.New(),
		Approver: approvingApprover{},
		Repo:     fakes.NewRepo[skill.Skill]("skills"),
		Registry: reg,
		Collections: collection.NewService(collection.Deps{
			Repo:     fakes.NewRepo[collection.Collection]("collections"),
			Registry: reg,
			Clock:    clock,
		}),
		Views:    newFakeViews(),
		Files:    files,
		Hooks:    noopHooks{},
		Toolsets: noopToolsets{},
		Clock:    clock,
	}
	for _, opt := range opts {
		opt(&d)
	}
	return skill.NewInstaller(d), files
}

func withApprover(a event.Approver) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Approver = a }
}

func withVerifier(v skill.Verifier) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Verifier = v }
}

func withApplier(a skill.Applier) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Applier = a }
}

func withHooks(h skill.Hooks) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Hooks = h }
}

func withToolsets(ts skill.Toolsets) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Toolsets = ts }
}

func withRepo(r skill.Repository) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Repo = r }
}

func withFetcher(f skill.Fetcher) func(*skill.Deps) {
	return func(d *skill.Deps) { d.Fetcher = f }
}

// mustInstall installs the committed crm-skill fixture with consent already
// given, failing the test on any error.
func mustInstall(t *testing.T, inst *skill.Installer) *skill.Skill {
	t.Helper()
	installed, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t), AcceptedAll: acceptAll})
	if err != nil {
		t.Fatal(err)
	}
	return installed
}

// fakeFiles is the in-memory skill.Files a test inspects afterward: the
// files an Apply actually placed, or the empty set a refused install left
// behind.
type fakeFiles struct {
	mu      sync.Mutex
	written map[string][]skill.RawFile
}

func newFakeFiles() *fakeFiles { return &fakeFiles{written: map[string][]skill.RawFile{}} }

func (f *fakeFiles) Write(_ context.Context, id string, files []skill.RawFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written[id] = append(f.written[id], files...)
	return nil
}

// paths lists every file this fake was ever asked to write, as
// "{skillID}/{path}".
func (f *fakeFiles) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for id, files := range f.written {
		for _, file := range files {
			out = append(out, id+"/"+file.Path)
		}
	}
	return out
}

// fakeViews is an in-memory skill.Views: no validation, just a map, because
// what this package's own tests exercise is the ordering and the
// registration a skill install performs, not view.Service's own validation —
// that already has its own test suite in internal/domain/view.
type fakeViews struct {
	mu    sync.Mutex
	views map[string]view.View
}

func newFakeViews() *fakeViews { return &fakeViews{views: map[string]view.View{}} }

func (f *fakeViews) Create(_ context.Context, in view.CreateInput) (*view.View, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v := view.View{
		ID: in.ID, Name: in.Name, Title: in.Title, Description: in.Description,
		Scope: in.Scope, Skill: in.Skill, Source: in.Source, Tree: in.Tree,
	}
	f.views[in.ID] = v
	out := v
	return &out, nil
}

func (f *fakeViews) Get(_ context.Context, in view.GetInput) (*view.View, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.views[in.ID]
	if !ok {
		return nil, apperr.New("VIEW_NOT_FOUND").Status(apperr.StatusNotFound).Issue("id", in.ID)
	}
	out := v
	return &out, nil
}

type noopHooks struct{}

func (noopHooks) Deregister(context.Context, string) error { return nil }

type noopToolsets struct{}

func (noopToolsets) Close(context.Context, string) error { return nil }

type approvingApprover struct{}

func (approvingApprover) RequestApproval(context.Context, event.ApprovalRequest) (event.ApprovalResult, error) {
	return event.ApprovalResult{Approved: true}, nil
}

// denyingApprover records the risk it was asked at, so a test can assert an
// agent calling skills_install cannot lower its own stakes.
type denyingApprover struct{ lastRisk event.RiskLevel }

func (d *denyingApprover) RequestApproval(_ context.Context, req event.ApprovalRequest) (event.ApprovalResult, error) {
	d.lastRisk = req.Risk
	return event.ApprovalResult{Approved: false, Reason: "not approved in this test"}, nil
}

type recordingVerifier struct{ order *[]string }

func (r recordingVerifier) VerifyManifest(pkg skill.Package) (skill.Diff, error) {
	*r.order = append(*r.order, "verified")
	return skill.Diff{Permissions: pkg.Manifest.Permissions}, nil
}

type recordingApprover struct{ order *[]string }

func (r recordingApprover) RequestApproval(context.Context, event.ApprovalRequest) (event.ApprovalResult, error) {
	*r.order = append(*r.order, "asked")
	return event.ApprovalResult{Approved: true}, nil
}

type recordingApplier struct{ order *[]string }

func (r recordingApplier) Apply(_ context.Context, id string, pkg skill.Package) (*skill.Skill, error) {
	*r.order = append(*r.order, "applied")
	return &skill.Skill{ID: id, Name: pkg.Manifest.Name}, nil
}

// failingApplier is a full stand-in for the real write path, used to prove
// that a failure while applying leaves nothing registered — without needing
// the real Collections/Views/Files dependencies to fail in a particular,
// hard-to-arrange way.
type failingApplier struct{ step string }

func (f failingApplier) Apply(context.Context, string, skill.Package) (*skill.Skill, error) {
	return nil, fmt.Errorf("applying failed while %s", f.step)
}

func applierFailingAt(step string) skill.Applier { return failingApplier{step: step} }

type recordingHooks struct{ order *[]string }

func (r recordingHooks) Deregister(context.Context, string) error {
	*r.order = append(*r.order, "hooks deregistered")
	return nil
}

type recordingToolsets struct{ order *[]string }

func (r recordingToolsets) Close(context.Context, string) error {
	*r.order = append(*r.order, "toolsets closed")
	return nil
}

// recordingRepo wraps a real fakes.Repo, so the ordering test can prove
// exactly when the skill's own record — and with it, per CascadeDelete, the
// whole directory — is removed, while everything else about persistence
// still behaves like the real thing.
type recordingRepo struct {
	inner *fakes.Repo[skill.Skill]
	order *[]string
}

func (r *recordingRepo) Get(ctx context.Context, key collections.Key) (*skill.Skill, error) {
	return r.inner.Get(ctx, key)
}

func (r *recordingRepo) List(ctx context.Context, q collections.Query) ([]skill.Skill, error) {
	return r.inner.List(ctx, q)
}

func (r *recordingRepo) Create(ctx context.Context, v *skill.Skill) error {
	return r.inner.Create(ctx, v)
}

func (r *recordingRepo) Delete(ctx context.Context, key collections.Key) error {
	*r.order = append(*r.order, "files removed")
	return r.inner.Delete(ctx, key)
}

type failingHooks struct{ err error }

func (f failingHooks) Deregister(context.Context, string) error { return f.err }

type stubFetcher struct{ pkg skill.Package }

func (s stubFetcher) Fetch(context.Context, string, string) (skill.Package, error) { return s.pkg, nil }

type failingFetcher struct{ err error }

func (f failingFetcher) Fetch(context.Context, string, string) (skill.Package, error) {
	return skill.Package{}, f.err
}

// ---------------------------------------------------------------------------
// Consent: an agent calling skills_install does not authorise itself.
// ---------------------------------------------------------------------------

// A denial is refused and writes nothing — and the request that was denied
// was asked at high risk, which is the whole reason ADR-0007's channel exists
// for this call in particular.
func TestInstallingWithoutConsentIsRefusedAndWritesNothing(t *testing.T) {
	approver := &denyingApprover{}
	inst, files := newInstaller(t, nil, withApprover(approver))

	_, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t)})
	if code := codeOf(t, err); code != "AOS_SKILL_INSTALL_NOT_APPROVED" {
		t.Fatalf("code = %q", code)
	}
	if approver.lastRisk != event.RiskHigh {
		t.Fatalf("risk = %q, want high", approver.lastRisk)
	}
	if wrote := files.paths(); len(wrote) != 0 {
		t.Fatalf("a refused install left files behind: %v", wrote)
	}
	list, err := inst.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 0 {
		t.Fatalf("a refused install registered a skill anyway: %+v", list.Skills)
	}
}

// The order matters: nothing touches the workspace before the manifest is
// verified and a human has consented.
func TestNothingIsWrittenBeforeVerificationAndConsent(t *testing.T) {
	var order []string
	inst, _ := newInstaller(t, nil,
		withVerifier(recordingVerifier{&order}),
		withApprover(recordingApprover{&order}),
		withApplier(recordingApplier{&order}),
	)

	if _, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t)}); err != nil {
		t.Fatal(err)
	}
	want := []string{"verified", "asked", "applied"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

// A caller that already has consent — AcceptedAll returning true — never
// reaches the approval channel at all.
func TestAcceptedAllSkipsTheApprovalChannel(t *testing.T) {
	asked := false
	approver := event.Approver(recordingApproverFunc(func() { asked = true }))
	inst, _ := newInstaller(t, nil, withApprover(approver))

	if _, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t), AcceptedAll: acceptAll}); err != nil {
		t.Fatal(err)
	}
	if asked {
		t.Fatal("AcceptedAll returning true still reached the approval channel")
	}
}

type recordingApproverFunc func()

func (f recordingApproverFunc) RequestApproval(context.Context, event.ApprovalRequest) (event.ApprovalResult, error) {
	f()
	return event.ApprovalResult{Approved: true}, nil
}

// Registration is last, so a partial failure leaves an unregistered directory
// rather than a half-registered skill.
func TestAFailureWhileApplyingLeavesNothingRegistered(t *testing.T) {
	reg := collections.NewRegistry()
	inst, _ := newInstaller(t, reg,
		withApprover(approvingApprover{}),
		withApplier(applierFailingAt("views")),
	)

	if _, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t)}); err == nil {
		t.Fatal("a failing apply reported success")
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection stayed registered after the install failed")
	}
	if _, err := inst.Get(ctx(), "crm"); err == nil {
		t.Fatal("the skill is listed as installed after a failed install")
	}
}

// ---------------------------------------------------------------------------
// Uninstall: hooks and toolsets go before the files, so nothing is left
// registered pointing at a directory that is gone.
// ---------------------------------------------------------------------------

func TestUninstallDeregistersHooksAndToolsetsBeforeRemovingFiles(t *testing.T) {
	var order []string
	inst, _ := newInstaller(t, nil,
		withHooks(recordingHooks{&order}),
		withToolsets(recordingToolsets{&order}),
		withRepo(&recordingRepo{inner: fakes.NewRepo[skill.Skill]("skills"), order: &order}),
	)
	installed := mustInstall(t, inst)

	if err := inst.Uninstall(ctx(), skill.UninstallInput{ID: installed.ID}); err != nil {
		t.Fatal(err)
	}
	want := []string{"hooks deregistered", "toolsets closed", "files removed"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v — nothing may stay registered pointing at a directory that is gone", order, want)
	}
}

// A skill-scoped collection and view go with it.
// The registry a skill-scoped collection's records live behind is in-memory
// state, independent of the files CascadeDelete removes — Uninstall has to
// clear it itself, or a collection whose declaration is already gone would
// still resolve for a write. Views carry no such registry (see Views' own
// doc comment in port.go), so this is a collections-only guarantee; the
// view's file going away with the rest of the skill's directory is exercised
// at the filesystem level by Task 11's delivery test, not here.
func TestUninstallUnregistersTheCollectionTheSkillBrought(t *testing.T) {
	reg := collections.NewRegistry()
	inst, _ := newInstaller(t, reg)
	installed := mustInstall(t, inst)

	if _, ok := reg.Lookup("contacts"); !ok {
		t.Fatal("the skill did not bring its collection")
	}
	if _, err := inst.Views().Get(ctx(), view.GetInput{ID: "contacts-table"}); err != nil {
		t.Fatalf("the skill's view is not reachable right after install: %v", err)
	}

	if err := inst.Uninstall(ctx(), skill.UninstallInput{ID: installed.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the skill-scoped collection outlived the skill")
	}
}

func TestUninstallingAnUnknownSkillIsRefused(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	if err := inst.Uninstall(ctx(), skill.UninstallInput{ID: "nope"}); err == nil {
		t.Fatal("uninstalling an unknown skill reported success")
	}
}

// A failure deregistering hooks stops Uninstall before anything about the
// skill is removed — the same "nothing torn down until the earlier step
// succeeded" guarantee the ordering test exercises, from the failure side.
func TestUninstallFailureDeregisteringHooksLeavesTheSkillRegistered(t *testing.T) {
	inst, _ := newInstaller(t, nil, withHooks(failingHooks{err: errors.New("boom")}))
	installed := mustInstall(t, inst)

	err := inst.Uninstall(ctx(), skill.UninstallInput{ID: installed.ID})
	if code := codeOf(t, err); code != "AOS_SKILL_UNINSTALL_FAILED" {
		t.Fatalf("code = %q", code)
	}
	if _, err := inst.Get(ctx(), installed.ID); err != nil {
		t.Fatal("the skill was removed even though hooks failed to deregister")
	}
}

// ---------------------------------------------------------------------------
// The happy path, end to end against the real local fetcher and the real
// collection domain: installing a skill installs a team.
// ---------------------------------------------------------------------------

func TestInstallBringsTheAgentCollectionAndView(t *testing.T) {
	inst, files := newInstaller(t, nil)
	installed := mustInstall(t, inst)

	if installed.ID != "crm" {
		t.Fatalf("id = %q", installed.ID)
	}
	if !installed.Active {
		t.Fatal("a freshly installed skill should be active")
	}
	if installed.Version != "1.0.0" {
		t.Fatalf("version = %q", installed.Version)
	}
	// Source and Commit stay empty for a local fetch: there is no provenance
	// to record, and inventing one would be worse than leaving it blank.
	if installed.Source != "" || installed.Commit != "" {
		t.Fatalf("source = %q, commit = %q, want both empty", installed.Source, installed.Commit)
	}
	if len(installed.Metadata.Collections) != 1 || installed.Metadata.Collections[0].ID != "contacts" {
		t.Fatalf("metadata.collections = %+v", installed.Metadata.Collections)
	}
	if len(installed.Metadata.Views) != 1 || installed.Metadata.Views[0].ID != "contacts-table" {
		t.Fatalf("metadata.views = %+v", installed.Metadata.Views)
	}

	var sawAgent bool
	for _, p := range files.paths() {
		if p == "crm/agents/closer/AGENT.md" {
			sawAgent = true
		}
	}
	if !sawAgent {
		t.Fatalf("the agent file was not written: %v", files.paths())
	}

	if _, err := inst.Views().Get(ctx(), view.GetInput{ID: "contacts-table"}); err != nil {
		t.Fatalf("the view is not reachable after install: %v", err)
	}

	got, err := inst.Get(ctx(), "crm")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description == "" {
		t.Fatal("the description should carry over from the manifest")
	}
}

// The defect class this plan has shipped four times: a Skill crossing the
// service boundary must not share its slices with what the repository holds.
func TestGetReturnsAnIndependentCopy(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	installed := mustInstall(t, inst)

	got, err := inst.Get(ctx(), installed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Permissions.Agents) == 0 {
		t.Fatal("fixture setup: expected at least one declared agent")
	}
	got.Permissions.Agents[0] = "mutated"
	got.Metadata.Collections = append(got.Metadata.Collections, skill.Ref{ID: "extra"})

	again, err := inst.Get(ctx(), installed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Permissions.Agents[0] == "mutated" {
		t.Fatal("Get shared Permissions.Agents with the caller's earlier copy")
	}
	if len(again.Metadata.Collections) != 1 {
		t.Fatalf("Get shared Metadata.Collections with the caller's earlier copy: %+v", again.Metadata.Collections)
	}
}

func TestGetOfAnUnknownSkillIsRefused(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	_, err := inst.Get(ctx(), "nope")
	if code := codeOf(t, err); code != "AOS_SKILL_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
}

func TestListWithNothingInstalledIsEmpty(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	out, err := inst.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 0 {
		t.Fatalf("total = %d, want 0", out.Total)
	}
}

func TestListReturnsEveryInstalledSkill(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	installed := mustInstall(t, inst)

	out, err := inst.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || len(out.Skills) != 1 || out.Skills[0].ID != installed.ID {
		t.Fatalf("list = %+v", out)
	}
}

func TestInstallWithoutASourceIsRefused(t *testing.T) {
	inst, _ := newInstaller(t, nil)
	_, err := inst.Install(ctx(), skill.InstallInput{AcceptedAll: acceptAll})
	if code := codeOf(t, err); code != "AOS_SKILL_SOURCE_REQUIRED" {
		t.Fatalf("code = %q", code)
	}
}

func TestInstallPropagatesAFetchFailure(t *testing.T) {
	want := errors.New("network down")
	inst, _ := newInstaller(t, nil, withFetcher(failingFetcher{err: want}))
	_, err := inst.Install(ctx(), skill.InstallInput{Source: "x", AcceptedAll: acceptAll})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want it to wrap %v", err, want)
	}
}

func TestInstallRefusesAManifestWithNoName(t *testing.T) {
	inst, _ := newInstaller(t, nil, withFetcher(stubFetcher{pkg: skill.Package{}}))
	_, err := inst.Install(ctx(), skill.InstallInput{Source: "x", AcceptedAll: acceptAll})
	if code := codeOf(t, err); code != "AOS_SKILL_MANIFEST_INVALID" {
		t.Fatalf("code = %q", code)
	}
}

func TestInstallRefusesAManifestNameThatIsNotAValidDirectorySegment(t *testing.T) {
	pkg := skill.Package{Manifest: skill.Manifest{Name: "Not Valid!"}}
	inst, _ := newInstaller(t, nil, withFetcher(stubFetcher{pkg: pkg}))
	_, err := inst.Install(ctx(), skill.InstallInput{Source: "x", AcceptedAll: acceptAll})
	if code := codeOf(t, err); code != "AOS_SKILL_MANIFEST_INVALID" {
		t.Fatalf("code = %q", code)
	}
}

// A collection the manifest declares that does not itself validate — no
// fields, here — fails inside collection.Service.Create, and the failure
// comes back naming the step it happened in rather than as a bare
// collection-domain error.
func TestApplyFailureIsWrappedAndNamesTheStep(t *testing.T) {
	pkg := skill.Package{
		Manifest: skill.Manifest{
			Name:        "broken",
			Permissions: skill.Permissions{Collections: []string{"empty"}},
		},
		Collections: []collection.CreateInput{{ID: "empty", Format: collection.FormatMarkdown}},
	}
	inst, _ := newInstaller(t, nil, withFetcher(stubFetcher{pkg: pkg}))
	_, err := inst.Install(ctx(), skill.InstallInput{Source: "x", AcceptedAll: acceptAll})
	if code := codeOf(t, err); code != "AOS_SKILL_APPLY_FAILED" {
		t.Fatalf("code = %q", code)
	}
}
