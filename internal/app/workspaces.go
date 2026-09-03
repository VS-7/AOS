package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// scopes is the set of workspaces this process is serving, built as they are
// asked for.
//
// A workspace is a directory: its agents, memories, chats, tasks, goals,
// collections, skills and artifacts are files under `<path>/.aos/`, and the
// repositories that read them are bound to that path when they are
// constructed. So "which workspace is this call for" is not a parameter any of
// those services take — it is which set of services the call reaches.
//
// The daemon used to have exactly one set, bound to the directory it started
// in, and nothing could change that for the life of the process. The workspace
// registry still worked: `workspace_create` wrote a record, `workspace_list`
// returned it, the interface offered it in the switcher. Nothing else did.
// Every agent, task and chat command went on reading and writing the one
// directory the process had opened, whatever workspace the caller named — so
// creating a second workspace and switching to it showed the first one's
// contents, and an agent created "in" the second was written into the first.
//
// This is the other half: one set of services per workspace, made the first
// time somebody addresses it and kept afterwards.
type scopes struct {
	mu sync.Mutex

	// primary is the workspace this process opened, and the one every call
	// that names no workspace lands in.
	primary *App

	// byPath, not byID: two ids can name one directory (a workspace
	// registered from a path and again from a symlink to it), and one set of
	// repositories over a directory is what serialises writes to it. Keying
	// by id would give the same files two locks and two record caches.
	byPath map[string]*App

	// build makes the services for one directory. It is a field so a test can
	// substitute a cheaper one, and so the recursion is obvious: it is New,
	// called again with a different root.
	build func(id, root string) (*App, error)
}

// workspaceRegistry wraps a registry so every call reaches the workspace it
// names — see command.Route, and scopes' own doc for what it fixes.
//
// The published surface is the primary's: same commands, same schemas, same
// documentation. A workspace cannot add or remove capabilities, only own
// different data.
func (a *App) workspaceRegistry() *command.Registry {
	return command.Route(a.Registry, a.registryFor)
}

// registryFor resolves the registry a call should run in from the workspace
// its context names.
func (a *App) registryFor(ctx context.Context) (*command.Registry, error) {
	target, err := a.scopeFor(ctx)
	if err != nil {
		return nil, err
	}
	return target.own, nil
}

// scopeFor resolves the services a call should run against.
//
// A call that names no workspace runs in the primary. That is what keeps every
// existing caller working: the CLI standing in a project, a test that built an
// App over a temporary directory, an agent taking a turn on behalf of the
// process's own workspace.
//
// So does a call naming a workspace this installation does not have. The
// header is an inference about what the caller probably means — a browser
// sends the cookie it was left with, which can name a workspace somebody has
// since deleted — and routing is not the place to turn that inference into a
// refusal. Every command that genuinely needs a workspace record already
// resolves one (workspace.Service.resolve) and refuses there, with the
// domain's own error and its own call to action; commands that need none, like
// gateway_status or views_components, went on working before this routing
// existed and go on working now.
func (a *App) scopeFor(ctx context.Context) (*App, error) {
	if a.scopes == nil {
		return a, nil
	}
	who := identity.From(ctx)
	id := who.WorkspaceID
	if id == "" {
		// Nobody named a workspace, but the caller said where it is standing.
		//
		// A terminal inside a registered repository, or a coding agent
		// reaching AOS over MCP from one, means *that* workspace — and until
		// now every such call silently addressed the daemon's primary scope
		// instead. `aos tasks list` run inside repo B listed repo A's tasks
		// and reported nothing missing.
		//
		// The audit that fixed which directory `workspace_introspect`
		// registers left this half undone: X-Working-Dir arrived, and only
		// that one command read it.
		id = a.workspaceAt(ctx, who.WorkingDir)
		if id == "" {
			return a, nil
		}
	}

	target, err := a.scopes.forID(ctx, a.Workspaces, id)
	if errors.Is(err, apperr.ErrNotFound) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	return target, nil
}

// workspaceAt names the registered workspace a directory belongs to, or "".
//
// The longest registered path that is a prefix of dir wins, so a workspace
// nested inside another resolves to the inner one — which is what somebody
// standing in it means.
func (a *App) workspaceAt(ctx context.Context, dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)

	out, err := a.Workspaces.List(ctx, workspace.ListInput{
		Reasoning: command.Reasoning{
			Reasoning: "resolving which workspace the caller's directory belongs to",
		},
	})
	if err != nil {
		return ""
	}

	best, bestLen := "", 0
	for _, w := range out.Workspaces {
		root := filepath.Clean(w.Path)
		if root == "." || root == string(filepath.Separator) {
			continue
		}
		if dir != root && !strings.HasPrefix(dir, root+string(filepath.Separator)) {
			continue
		}
		if len(root) > bestLen {
			best, bestLen = w.ID, len(root)
		}
	}
	return best
}

// forID finds or builds the services for one workspace id.
func (s *scopes) forID(ctx context.Context, registry *workspace.Service, id string) (*App, error) {
	// The record says where the workspace is. It is read through the domain
	// service rather than the store so an id nobody registered is refused
	// here, with the domain's own error, instead of silently becoming a
	// directory nobody asked for.
	//
	// The lookup is deliberately outside the lock below: it touches the
	// filesystem, and holding the mutex across it would serialise every
	// request in the process behind one directory read.
	found, err := registry.Get(ctx, workspace.GetInput{
		Workspace: id,
		Reasoning: command.Reasoning{
			Reasoning: "resolving which workspace's data this call addresses",
		},
	})
	if err != nil {
		return nil, err
	}
	root := filepath.Clean(found.Path)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.primary != nil && filepath.Clean(s.primary.Workspace) == root {
		return s.primary, nil
	}
	if already, ok := s.byPath[root]; ok {
		return already, nil
	}

	built, err := s.build(id, root)
	if err != nil {
		return nil, errWorkspaceUnavailable(id, root, err)
	}
	if s.byPath == nil {
		s.byPath = map[string]*App{}
	}
	s.byPath[root] = built
	return built, nil
}

// close releases every workspace opened after the primary. The primary is
// closed by whoever built it.
func (s *scopes) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for path, opened := range s.byPath {
		if err := opened.Close(); err != nil {
			errs = append(errs, err)
		}
		delete(s.byPath, path)
	}
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func errWorkspaceUnavailable(id, root string, err error) error {
	return apperr.New("WORKSPACE_UNAVAILABLE").
		Causer("app.scopes.forID").
		Msgf("the workspace %q could not be opened", id).
		Issue("workspace", id).
		Issue("path", root).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label:   "check that the directory still exists and is readable, then try again",
			Command: build.Name + " workspace list",
		}).
		Wrap(err)
}
