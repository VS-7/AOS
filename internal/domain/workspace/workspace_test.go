package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/workspace"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

const repoRoot = "/home/me/project-alpha"

type harness struct {
	svc    *workspace.Service
	store  *fakeStore
	fs     *fakes.Files
	git    *fakeGit
	seeder *fakeSeeder
	survey fakeSurveyor
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		store:  newStore(),
		fs:     fakes.NewFiles(),
		git:    newGit(),
		seeder: newSeeder(),
		survey: fakeSurveyor{byRoot: map[string][]workspace.CollectionSummary{}},
	}
	h.svc = workspace.NewService(workspace.Deps{
		Store:         h.store,
		FS:            h.fs,
		Git:           h.git,
		Seeder:        h.seeder,
		Surveyor:      h.survey,
		Clock:         clockx.Fixed{At: refTime},
		WorkspacesDir: "/state/workspaces",
		WorkingDir:    repoRoot,
	})
	return h
}

func ctx() context.Context { return context.Background() }

func (h *harness) create(t *testing.T, in workspace.CreateInput) workspace.CreateOutput {
	t.Helper()
	out, err := h.svc.Create(ctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateSlugsTheNameAndStampsTheDefaults(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	w := out.Workspace
	if w.ID != "project-alpha" {
		t.Errorf("id = %q, want the slug of the name", w.ID)
	}
	if w.Color != workspace.DefaultColor {
		t.Errorf("color = %q, want the default", w.Color)
	}
	if len(w.Tasks) != len(workspace.DefaultTaskTypes) {
		t.Errorf("task types = %d, want the five defaults", len(w.Tasks))
	}
	if _, ok := w.TaskTypeOf("bug"); !ok {
		t.Error("the default taxonomy has no \"bug\" type")
	}
	if w.Git.BranchPrefix != "aos" || w.Worktrees.WorktreeLimit != 15 {
		t.Errorf("policies = %+v / %+v", w.Git, w.Worktrees)
	}
	if !w.CreatedAt.Equal(refTime) || !w.UpdatedAt.Equal(refTime) {
		t.Errorf("timestamps = %v / %v", w.CreatedAt, w.UpdatedAt)
	}
}

// TestDefaultTaskTypesAreNotShared: the defaults are a package-level slice, and
// a workspace that edited its own taxonomy in place would edit every future
// workspace's.
func TestDefaultTaskTypesAreNotShared(t *testing.T) {
	h := newHarness(t)
	first := h.create(t, workspace.CreateInput{Name: "One", Path: "/repo/one"})
	first.Workspace.Tasks[0].Label = "Mutated"

	second := h.create(t, workspace.CreateInput{Name: "Two", Path: "/repo/two"})
	if second.Workspace.Tasks[0].Label == "Mutated" {
		t.Fatal("editing one workspace's taxonomy reached the defaults")
	}
	if workspace.DefaultTaskTypes[0].Label == "Mutated" {
		t.Fatal("the package-level defaults were mutated")
	}
}

func TestCreateRejectsANameWithNoSlug(t *testing.T) {
	h := newHarness(t)
	for _, name := range []string{"", "   ", "!!!", "---"} {
		_, err := h.svc.Create(ctx(), workspace.CreateInput{Name: name, Path: repoRoot})
		if err == nil {
			t.Errorf("%q: a name that slugs to nothing has no identity", name)
			continue
		}
		if !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("%q: error = %v", name, err)
		}
		e, _ := apperr.As(err)
		if len(e.Actions) == 0 {
			t.Errorf("%q: a 400 must carry a CTA", name)
		}
	}
}

func TestCreateRefusesADuplicate(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	// Not the same string, but the same slug: this is the collision the slug
	// package documents, and the aggregate is where it has to be caught.
	_, err := h.svc.Create(ctx(), workspace.CreateInput{Name: "project alpha!", Path: "/elsewhere"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateRejectsARelativePath(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Create(ctx(), workspace.CreateInput{Name: "Alpha", Path: "./relative"})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateWithoutAPathLandsUnderTheStateDirectory(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, workspace.CreateInput{Name: "Detached"})
	if out.Workspace.Path != "/state/workspaces/detached/workspace" {
		t.Fatalf("path = %q", out.Workspace.Path)
	}
}

// TestScaffoldCreatesTheManagedLayout checks the directories, and that each one
// got the marker that makes an empty directory survive a clone.
func TestScaffoldCreatesTheManagedLayout(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	root := repoRoot + "/" + collections.Root
	if !h.fs.HasDir(root) {
		t.Fatalf("%s was not created", root)
	}
	for _, dir := range workspace.ManagedDirs {
		p := root + "/" + dir
		if !h.fs.HasDir(p) {
			t.Errorf("%s was not created", p)
		}
		if _, ok := h.fs.File(p + "/.gitkeep"); !ok {
			t.Errorf("%s has no .gitkeep", p)
		}
	}
	if len(out.Scaffold.CreatedDirs) != len(workspace.ManagedDirs)+1 {
		t.Errorf("report lists %d created dirs, want %d",
			len(out.Scaffold.CreatedDirs), len(workspace.ManagedDirs)+1)
	}
	if out.Adopted {
		t.Error("a fresh layout is created, not adopted")
	}
}

// TestScaffoldIsIdempotent is the property the whole design of this step rests
// on: introspect is meant to be run repeatedly.
func TestScaffoldIsIdempotent(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	writesBefore := h.fs.Snapshot()
	h.store.Delete(ctx(), "project-alpha") //nolint:errcheck // fake never fails

	second := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	if len(second.Scaffold.CreatedDirs) != 0 {
		t.Errorf("the second run created %v", second.Scaffold.CreatedDirs)
	}
	if !second.Adopted {
		t.Error("an existing layout must be reported as adopted")
	}
	if len(second.Scaffold.EnvFiles) != 0 {
		t.Errorf("the second run rewrote %v", second.Scaffold.EnvFiles)
	}
	for path, before := range writesBefore {
		if h.fs.Writes[path] != before {
			t.Errorf("%s was written again by an idempotent run", path)
		}
	}
}

// TestEnvSplicePreservesWhatTheUserWrote is the rule that matters most in this
// step: this system is editing a file it does not own.
func TestEnvSplicePreservesWhatTheUserWrote(t *testing.T) {
	h := newHarness(t)
	writeFile(t, h.fs, repoRoot+"/.env", "DATABASE_URL=postgres://localhost/app\nAPI_KEY=secret\n")

	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	got, _ := h.fs.File(repoRoot + "/.env")
	for _, want := range []string{"DATABASE_URL=postgres://localhost/app", "API_KEY=secret"} {
		if !hasLine(got, want) {
			t.Errorf("the user's line %q was lost:\n%s", want, got)
		}
	}
	if !hasLine(got, "AOS_WORKSPACE_ID=project-alpha") {
		t.Errorf("the managed value is missing:\n%s", got)
	}
}

// TestEnvSpliceReplacesOnlyTheManagedBlock: on a rename or a re-registration,
// the block is rewritten in place, and the lines the user put after it stay
// after it.
func TestEnvSpliceReplacesOnlyTheManagedBlock(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	// The user appends their own line after the managed block, then the
	// workspace is registered again under a different id.
	appendFile(t, h.fs, repoRoot+"/.env", "\nUSER_ADDED=later\n")
	h.create(t, workspace.CreateInput{Name: "Project Beta", Path: repoRoot})

	got, _ := h.fs.File(repoRoot + "/.env")
	if !hasLine(got, "USER_ADDED=later") {
		t.Errorf("a line after the managed block was lost:\n%s", got)
	}
	if !hasLine(got, "AOS_WORKSPACE_ID=project-beta") {
		t.Errorf("the managed block was not updated:\n%s", got)
	}
	if strings.Count(got, "AOS_WORKSPACE_ID=") != 1 {
		t.Errorf("the block was appended instead of replaced:\n%s", got)
	}
}

// TestTheSampleCarriesTheVariableWithoutTheValue: .env.sample is committed, and
// one person's workspace id is not the next person's.
func TestTheSampleCarriesTheVariableWithoutTheValue(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	sample, _ := h.fs.File(repoRoot + "/.env.sample")
	if !hasLine(sample, "AOS_WORKSPACE_ID=") {
		t.Errorf("the sample does not declare the variable:\n%s", sample)
	}
	if strings.Contains(sample, "project-alpha") {
		t.Errorf("the sample leaked a concrete workspace id:\n%s", sample)
	}
}

func TestGitIsInitialisedWhenAbsent(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	if h.git.inits != 1 || !out.Scaffold.GitInit {
		t.Fatalf("inits = %d, report = %+v", h.git.inits, out.Scaffold)
	}
	if out.Scaffold.GitWarning != "" {
		t.Errorf("unexpected warning: %q", out.Scaffold.GitWarning)
	}
}

func TestAnExistingRepositoryIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	h.git.repos[repoRoot] = true

	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	if h.git.inits != 0 || out.Scaffold.GitInit {
		t.Fatal("an existing repository must not be re-initialised")
	}
}

// TestGitFailureIsVisibleAndNotFatal is a deliberate divergence: the original
// swallows this error. A workspace that is not under version control has lost
// the property that justifies keeping agent state in the repository, and the
// person has to be able to find that out.
func TestGitFailureIsVisibleAndNotFatal(t *testing.T) {
	h := newHarness(t)
	h.git.initErr = errors.New("git: command not found")

	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	if out.Workspace.ID != "project-alpha" {
		t.Fatal("the workspace must still be created")
	}
	if out.Scaffold.GitInit {
		t.Error("nothing was initialised")
	}
	if !strings.Contains(out.Scaffold.GitWarning, "not under version control") {
		t.Errorf("warning = %q", out.Scaffold.GitWarning)
	}
}

func TestTheOrchestratorIsBornWithTheWorkspace(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	if out.Orchestrator != "atlas" {
		t.Fatalf("orchestrator = %q, want the slug of the default name", out.Orchestrator)
	}
	seed := h.seeder.byRoot[repoRoot]
	if seed.Name != workspace.DefaultOrchestratorName || seed.Role != workspace.DefaultOrchestratorRole {
		t.Errorf("seed = %+v", seed)
	}
	if !strings.Contains(seed.Instructions, "Project Alpha") {
		t.Error("the instructions do not name the workspace")
	}
	if !strings.Contains(seed.Instructions, repoRoot) {
		t.Error("the instructions do not carry the workspace path")
	}
}

// TestOrchestratorDialsBecomeProse: tone, style and autonomy exist to change
// what the agent does, so each has to produce a sentence a model can act on.
func TestOrchestratorDialsBecomeProse(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{
		Name: "Project Alpha", Path: repoRoot,
		Orchestrator: &workspace.OrchestratorSpec{
			Name: "Luara", Tone: "candid", Style: "concise", Autonomy: 0.9,
		},
	})

	seed := h.seeder.byRoot[repoRoot]
	if seed.ID != "luara" || seed.Name != "Luara" {
		t.Fatalf("seed identity = %q / %q", seed.ID, seed.Name)
	}
	for _, want := range []string{"candid", "concise", "90%"} {
		if !strings.Contains(seed.Instructions, want) {
			t.Errorf("the instructions do not mention %q:\n%s", want, seed.Instructions)
		}
	}
	// The point of the dials is behaviour, not labels.
	if !strings.Contains(seed.Instructions, "do not soften a real risk") {
		t.Error("a candid tone produced no instruction about candour")
	}
}

func TestNoDialsMeansNoBehaviourSection(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	if strings.Contains(h.seeder.byRoot[repoRoot].Instructions, "Communication & Behaviour") {
		t.Error("an empty spec produced an empty section")
	}
}

// TestAnExistingOrchestratorIsAdopted: registering a workspace over a layout
// another machine created must not produce a second orchestrator.
func TestAnExistingOrchestratorIsAdopted(t *testing.T) {
	h := newHarness(t)
	h.seeder.present[repoRoot] = "atlas"

	out := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	if out.Orchestrator != "atlas" {
		t.Fatalf("orchestrator = %q", out.Orchestrator)
	}
	if h.seeder.calls != 0 {
		t.Fatal("a second orchestrator was created")
	}
}

func TestASeedFailureFailsTheCreate(t *testing.T) {
	h := newHarness(t)
	h.seeder.err = errors.New("disk full")

	_, err := h.svc.Create(ctx(), workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	if err == nil {
		t.Fatal("a workspace with no orchestrator answers nobody")
	}
	if h.store.saves != 0 {
		t.Error("the registry must not record a workspace that was not fully created")
	}
}

func TestGetFallsBackToTheActiveWorkspace(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	if _, err := h.svc.Get(ctx(), workspace.GetInput{}); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("with no active workspace the error should say so: %v", err)
	}

	active := workspace.NewService(workspace.Deps{
		Store: h.store, FS: h.fs, Git: h.git, Seeder: h.seeder, Surveyor: h.survey,
		Clock: clockx.Fixed{At: refTime}, Active: "project-alpha",
	})
	got, err := active.Get(ctx(), workspace.GetInput{})
	if err != nil || got.ID != "project-alpha" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestListOrdersAndHidesTheArchived(t *testing.T) {
	h := newHarness(t)
	for _, name := range []string{"Zeta", "Alpha", "Mu"} {
		h.create(t, workspace.CreateInput{Name: name, Path: "/repo/" + strings.ToLower(name)})
	}
	if _, err := h.svc.Update(ctx(), workspace.UpdateInput{
		Workspace: "mu", Set: map[string]any{"archived": true},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.List(ctx(), workspace.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || out.Workspaces[0].ID != "alpha" || out.Workspaces[1].ID != "zeta" {
		t.Fatalf("list = %+v", out)
	}

	all, err := h.svc.List(ctx(), workspace.ListInput{IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 {
		t.Fatalf("with archived = %d, want 3", all.Total)
	}
}

func TestUpdateChangesOnlyWhatWasSent(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	got, err := h.svc.Update(ctx(), workspace.UpdateInput{
		Workspace: "project-alpha",
		Set:       map[string]any{"git.branchPrefix": "feat", "color": "#10b981"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Git.BranchPrefix != "feat" || got.Color != "#10b981" {
		t.Fatalf("update = %+v / %q", got.Git, got.Color)
	}
	if got.Git.ForcePush {
		t.Error("a sibling field was changed")
	}
	if got.Name != "Project Alpha" || len(got.Tasks) != len(workspace.DefaultTaskTypes) {
		t.Error("an untouched field was blanked")
	}
}

// TestUpdateCannotMoveTheIdentity: the id names the directory that holds this
// workspace's derived state, and every record that refers to it does so by id.
func TestUpdateCannotMoveTheIdentity(t *testing.T) {
	h := newHarness(t)
	before := h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot}).Workspace

	got, err := h.svc.Update(ctx(), workspace.UpdateInput{
		Workspace: "project-alpha",
		Set: map[string]any{
			"id":        "hijacked",
			"path":      "/somewhere/else",
			"createdAt": "2000-01-01T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != before.ID || got.Path != before.Path || !got.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("the server-owned fields moved: %+v", got)
	}
}

func TestUpdateRejectsAnUnknownField(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})

	_, err := h.svc.Update(ctx(), workspace.UpdateInput{
		Workspace: "project-alpha", Set: map[string]any{"git.rebasePolicy": "always"},
	})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}

// TestDeleteLeavesTheRepositoryAlone: unregistering is not a request to delete
// the agents, memories and instructions the person wrote.
func TestDeleteLeavesTheRepositoryAlone(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	filesBefore := len(h.fs.Paths())

	out, err := h.svc.Delete(ctx(), workspace.DeleteInput{Workspace: "project-alpha"})
	if err != nil || !out.Deleted {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
	if out.Path != repoRoot {
		t.Errorf("the result should say what stays on disk, got %q", out.Path)
	}
	if len(h.fs.Paths()) != filesBefore {
		t.Error("unregistering removed files from the user's repository")
	}

	if _, err := h.svc.Delete(ctx(), workspace.DeleteInput{Workspace: "project-alpha"}); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("deleting twice = %v", err)
	}
}

func TestIntrospectNamesTheWorkspaceAfterTheGitRemote(t *testing.T) {
	h := newHarness(t)
	h.git.origin = "git@github.com:someone/project-alpha.git"

	out, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workspace.Name != "project-alpha" || out.Workspace.Path != repoRoot {
		t.Fatalf("workspace = %+v", out.Workspace)
	}
}

func TestIntrospectFallsBackToTheDirectoryName(t *testing.T) {
	h := newHarness(t)
	out, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workspace.Name != "project-alpha" {
		t.Fatalf("name = %q", out.Workspace.Name)
	}
}

func TestIntrospectOnHTTPSRemote(t *testing.T) {
	h := newHarness(t)
	h.git.origin = "https://github.com/someone/Other-Repo.git"
	out, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Workspace.ID != "other-repo" {
		t.Fatalf("id = %q", out.Workspace.ID)
	}
}

// TestIntrospectIsIdempotent: the command is documented as safe to run
// repeatedly, and a second run must not fail with "already exists".
func TestIntrospectIsIdempotent(t *testing.T) {
	h := newHarness(t)
	first, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.svc.Introspect(ctx(), workspace.IntrospectInput{})
	if err != nil {
		t.Fatalf("the second run failed: %v", err)
	}
	if second.Workspace.ID != first.Workspace.ID {
		t.Fatalf("second run registered a different workspace: %q", second.Workspace.ID)
	}
	if !second.Adopted {
		t.Error("the second run should report that it adopted the existing record")
	}
	if h.store.saves != 1 {
		t.Errorf("the registry was written %d times", h.store.saves)
	}
}

func TestInventoryReportsCountsAndTotals(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	h.survey.byRoot[repoRoot] = []workspace.CollectionSummary{
		{Name: "memories", Count: 12},
		{Name: "agents", Count: 2, Keys: []string{"atlas", "reviewer"}},
	}

	got, err := h.svc.Inventory(ctx(), workspace.InventoryInput{Workspace: "project-alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 14 {
		t.Errorf("total = %d, want 14", got.Total)
	}
	if got.Collections[0].Name != "agents" || got.Collections[1].Name != "memories" {
		t.Errorf("collections are not ordered: %+v", got.Collections)
	}
	if len(got.TaskTypes) != len(workspace.DefaultTaskTypes) {
		t.Error("the inventory should carry the task taxonomy in force")
	}
}

// TestInventoryCarriesNoBodies: this is the call an agent makes at the start of
// every session, and inlining content would make it the most expensive one.
func TestInventoryCarriesNoBodies(t *testing.T) {
	h := newHarness(t)
	h.create(t, workspace.CreateInput{Name: "Project Alpha", Path: repoRoot})
	h.survey.byRoot[repoRoot] = []workspace.CollectionSummary{
		{Name: "agents", Count: 1, Keys: []string{"atlas"}},
	}

	got, err := h.svc.Inventory(ctx(), workspace.InventoryInput{Workspace: "project-alpha"})
	if err != nil {
		t.Fatal(err)
	}
	// The type is the guarantee: a summary has a name, a count and identifiers,
	// and there is nowhere for a Markdown body to go.
	for _, c := range got.Collections {
		for _, k := range c.Keys {
			if strings.Contains(k, "\n") {
				t.Errorf("a key looks like a body: %q", k)
			}
		}
	}
}

func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	h := newHarness(t)
	reg := command.NewRegistry()
	workspace.Register(reg, h.svc)

	want := map[string]command.Annotations{
		"workspace_create":     {Title: "Create a workspace"},
		"workspace_delete":     {Title: "Unregister a workspace", DestructiveHint: true},
		"workspace_get":        {Title: "Read a workspace", ReadOnlyHint: true, IdempotentHint: true},
		"workspace_introspect": {Title: "Register the current repository", IdempotentHint: true},
		"workspace_inventory":  {Title: "Survey a workspace", ReadOnlyHint: true, IdempotentHint: true},
		"workspace_list":       {Title: "List workspaces", ReadOnlyHint: true, IdempotentHint: true},
		"workspace_update":     {Title: "Update a workspace", IdempotentHint: true},
	}
	got := reg.Sorted()
	if len(got) != len(want) {
		keys := make([]string, len(got))
		for i, d := range got {
			keys[i] = d.Key()
		}
		t.Fatalf("commands = %v, want %d of them", keys, len(want))
	}
	for _, d := range got {
		w, ok := want[d.Key()]
		if !ok {
			t.Errorf("unexpected command %q", d.Key())
			continue
		}
		if d.Annotations() != w {
			t.Errorf("%s annotations = %+v, want %+v", d.Key(), d.Annotations(), w)
		}
	}
}
