package workspace

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/patch"
	"github.com/OWNER/aos/internal/core/slug"
)

// Deps is everything the service is built from. It is a struct rather than a
// parameter list because seven positional arguments at a call site say nothing
// about which is which, and the composition root is the one place a reader goes
// to find out how the system is put together.
type Deps struct {
	Store    Store
	FS       Scaffolder
	Git      Git
	Seeder   Seeder
	Surveyor Surveyor
	Clock    Clock

	// WorkspacesDir is where a workspace created without a path gets one.
	WorkspacesDir string

	// Active is the workspace a command addresses when it names none.
	Active string

	// WorkingDir is where `introspect` looks when given no path.
	WorkingDir string
}

// Service is the workspace aggregate.
type Service struct {
	store    Store
	fs       Scaffolder
	git      Git
	seeder   Seeder
	surveyor Surveyor
	clock    Clock

	workspacesDir string
	active        string
	workingDir    string
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{
		store:         d.Store,
		fs:            d.FS,
		git:           d.Git,
		seeder:        d.Seeder,
		surveyor:      d.Surveyor,
		clock:         d.Clock,
		workspacesDir: d.WorkspacesDir,
		active:        d.Active,
		workingDir:    d.WorkingDir,
	}
}

// List returns the registered workspaces, ordered by id.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	found, err := s.store.List(ctx)
	if err != nil {
		return ListOutput{}, errStoreFailed("read", err)
	}
	out := make([]Workspace, 0, len(found))
	for _, w := range found {
		if w.Archived && !in.IncludeArchived {
			continue
		}
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return ListOutput{Workspaces: out, Total: len(out)}, nil
}

// Get reads one workspace, defaulting to the active one.
func (s *Service) Get(ctx context.Context, in GetInput) (*Workspace, error) {
	return s.resolve(ctx, in.Workspace)
}

// Create registers a workspace and lays it out inside the repository.
//
// The registry entry is written last. Every step before it is idempotent, so a
// failure halfway leaves a repository with some directories in it and nothing
// claiming to own them — which is recoverable by running the command again.
// The reverse order would leave a registered workspace pointing at a layout
// that was never created.
func (s *Service) Create(ctx context.Context, in CreateInput) (CreateOutput, error) {
	id := slug.Generate(in.Name)
	if id == "" {
		return CreateOutput{}, errInvalidName(in.Name)
	}
	if existing, err := s.store.Get(ctx, id); err == nil && existing != nil {
		return CreateOutput{}, errAlreadyExists(id)
	} else if err != nil && !errors.Is(err, apperr.ErrNotFound) {
		return CreateOutput{}, errStoreFailed("read", err)
	}

	root := strings.TrimSpace(in.Path)
	if root == "" {
		root = path.Join(s.workspacesDir, id, "workspace")
	}
	if !path.IsAbs(root) {
		return CreateOutput{}, errRelativePath(root)
	}
	root = path.Clean(root)

	now := s.clock.Now()
	colour := in.Color
	if colour == "" {
		colour = DefaultColor
	}
	w := Workspace{
		ID:        id,
		Name:      in.Name,
		Path:      root,
		Logo:      in.Logo,
		Color:     colour,
		Tasks:     defaultTaskTypes(),
		Labels:    []Label{},
		Worktrees: DefaultWorktrees(),
		Git:       DefaultGit(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	report, err := s.scaffold(ctx, &w)
	if err != nil {
		return CreateOutput{}, err
	}
	// Nothing created means the layout was already there: this is a repository
	// that has been a workspace before, and it is adopted rather than reset.
	adopted := len(report.CreatedDirs) == 0

	report.GitInit, report.GitWarning = s.ensureGit(ctx, &w)

	orchestrator, err := s.ensureOrchestrator(ctx, &w, in.Orchestrator)
	if err != nil {
		return CreateOutput{}, err
	}

	if err := s.store.Save(ctx, &w); err != nil {
		return CreateOutput{}, errStoreFailed("written", err)
	}

	return CreateOutput{
		Workspace:    w,
		Orchestrator: orchestrator,
		Scaffold:     report,
		Adopted:      adopted,
	}, nil
}

// ensureOrchestrator gives the workspace exactly one orchestrator: the one
// already in the repository if there is one, a new one otherwise.
//
// Adoption is the case that matters. Registering a workspace over a directory
// that another machine already set up must not produce a second orchestrator —
// there is at most one per workspace, and two would make routing ambiguous.
func (s *Service) ensureOrchestrator(ctx context.Context, w *Workspace, spec *OrchestratorSpec) (string, error) {
	if s.seeder == nil {
		return "", nil
	}
	existing, found, err := s.seeder.FindOrchestrator(ctx, w.Path)
	if err != nil {
		return "", errSeedFailed(w.ID, err)
	}
	if found {
		return existing, nil
	}
	id, err := s.seeder.SeedOrchestrator(ctx, buildOrchestrator(w, spec))
	if err != nil {
		return "", errSeedFailed(w.ID, err)
	}
	return id, nil
}

// Update patches the workspace record by dotted path.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Workspace, error) {
	current, err := s.resolve(ctx, in.Workspace)
	if err != nil {
		return nil, err
	}

	next, err := patch.Apply(*current, in.Set)
	if err != nil {
		var unknown *patch.UnknownPathError
		if errors.As(err, &unknown) {
			return nil, errUnknownField(unknown.Path)
		}
		var bad *patch.ValueError
		if errors.As(err, &bad) {
			return nil, errUnknownField(bad.Path)
		}
		return nil, err
	}

	// Identity and the audit trail are the server's, not the caller's: a patch
	// that reached them would let a rename orphan every record that refers to
	// this workspace by id.
	next.ID = current.ID
	next.Path = current.Path
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = s.clock.Now()

	if err := s.store.Save(ctx, &next); err != nil {
		return nil, errStoreFailed("written", err)
	}
	return &next, nil
}

// Delete unregisters a workspace.
//
// It removes the registry entry and the derived state under the installation
// directory. What it does not touch is the user's repository: the .aos/
// directory in it holds agents, memories and instructions the person wrote, and
// unregistering a workspace is not a request to delete their work.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.Workspace)
	current, err := s.store.Get(ctx, id)
	if err != nil || current == nil {
		return DeleteOutput{Workspace: id}, errNotFound(id)
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return DeleteOutput{Workspace: id}, errStoreFailed("written", err)
	}
	return DeleteOutput{Workspace: id, Deleted: true, Path: current.Path}, nil
}

// Introspect registers the repository the caller is standing in, and returns
// the existing registration when there is one.
//
// The name comes from the Git remote, falling back to the directory. This is
// the zero-configuration entry point: run it at the root of a project and the
// project becomes a workspace.
func (s *Service) Introspect(ctx context.Context, in IntrospectInput) (CreateOutput, error) {
	root := strings.TrimSpace(in.Path)
	if root == "" {
		root = s.workingDir
	}
	root = path.Clean(root)

	// A repository that is already registered is returned as it is. Running
	// this twice is a normal thing to do, and the second run must not fail.
	known, err := s.store.List(ctx)
	if err != nil {
		return CreateOutput{}, errStoreFailed("read", err)
	}
	for i := range known {
		if known[i].Path == root {
			return CreateOutput{Workspace: known[i], Adopted: true}, nil
		}
	}

	return s.Create(ctx, CreateInput{Name: s.nameFor(ctx, root), Path: root})
}

// nameFor derives a workspace name from the repository: the origin remote
// first, then the directory. Both can fail to produce anything usable, and the
// fallback is a name that at least registers.
func (s *Service) nameFor(ctx context.Context, root string) string {
	if s.git != nil {
		if origin, err := s.git.OriginURL(ctx, root); err == nil {
			if name := repoNameFromURL(origin); name != "" {
				return name
			}
		}
	}
	if base := path.Base(root); base != "" && base != "." && base != "/" {
		return base
	}
	return "workspace"
}

// repoNameFromURL takes the last segment of a Git URL, in either the SSH or the
// HTTPS form, and drops the .git suffix.
func repoNameFromURL(url string) string {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return ""
	}
	last := trimmed
	if i := strings.LastIndexAny(trimmed, ":/"); i >= 0 {
		last = trimmed[i+1:]
	}
	return strings.TrimSuffix(last, ".git")
}

// Inventory reports what a workspace holds, without loading a body.
func (s *Service) Inventory(ctx context.Context, in InventoryInput) (Inventory, error) {
	w, err := s.resolve(ctx, in.Workspace)
	if err != nil {
		return Inventory{}, err
	}
	out := Inventory{
		Workspace: w.ID,
		Name:      w.Name,
		Path:      w.Path,
		TaskTypes: w.Tasks,
	}
	if s.surveyor == nil {
		return out, nil
	}
	summaries, err := s.surveyor.Survey(ctx, w.Path)
	if err != nil {
		return Inventory{}, errStoreFailed("read", err)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	out.Collections = summaries
	for _, c := range summaries {
		out.Total += c.Count
	}
	return out, nil
}

// resolve loads a workspace by id: the one given explicitly, the one this
// installation was started against (Deps.Active — a single-process CLI/MCP
// run, or a daemon an operator deliberately pinned to one workspace), or the
// one the caller is scoped to (the x-workspace-id a browser or the desktop's
// own client sends with every call — see ambientIdentity in httpapi).
//
// Active outranks ambient identity rather than the other way around: it is a
// deliberate, administrative setting, where a request header is an inference
// about what the caller probably means. A daemon serving several workspaces
// leaves Active unset, which is what lets ambient identity resolve anything
// at all here.
func (s *Service) resolve(ctx context.Context, id string) (*Workspace, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		target = s.active
	}
	if target == "" {
		target = identity.From(ctx).WorkspaceID
	}
	if target == "" {
		return nil, errNotFound("")
	}
	w, err := s.store.Get(ctx, target)
	if err != nil || w == nil {
		return nil, errNotFound(target)
	}
	return w, nil
}

// AuthorizeWorkspace reports whether a caller may read a workspace's events.
//
// Membership is the rule, with one deliberate exception: a workspace with no
// members at all is a single-user installation, which is what every local
// installation is until somebody adds a second account. Refusing those would
// mean nobody could open a socket on their own machine.
//
// The workspace has to exist either way. Without that check this would be the
// original's behaviour with extra steps: an unknown id would authorise, because
// there would be no membership list to fail against.
func (s *Service) AuthorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	w, err := s.store.Get(ctx, strings.TrimSpace(workspaceID))
	if err != nil || w == nil {
		return errNotFound(workspaceID)
	}
	if len(w.Members) == 0 {
		return nil
	}
	for _, m := range w.Members {
		if m.UserID == userID && userID != "" {
			return nil
		}
	}
	return errAccessDenied(workspaceID, userID)
}
