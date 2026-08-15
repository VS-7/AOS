// Package app is the composition root: the one place where adapters are
// constructed and handed to the domain.
//
// It lives under internal/ rather than in cmd/ because two binaries need the
// same wiring — the daemon and, in the tests, the parity suite that runs one
// command through every surface. The rule it exists to keep is that no `new` of
// an adapter appears anywhere else.
package app

import (
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/adapters/fsworkspace"
	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// Options select where the state lives and what time it is.
type Options struct {
	// Env resolves the settings. Nil uses the process environment.
	Env *env.Resolver

	// WorkspaceRoot is the directory that holds .aos/. Empty means the current
	// working directory, which is what makes the tool work like git: run it
	// inside a repository and it operates on that repository.
	WorkspaceRoot string

	// Clock is injected so a test can freeze time.
	Clock clockx.Clock

	// IDs is injected so a test can predict the identifier of a new record.
	IDs ids.Generator
}

// App is everything wired together.
type App struct {
	Paths     corecfg.Paths
	Workspace string
	Registry  *command.Registry

	Config     config.Service
	Agents     *agent.Service
	Memories   *memory.Service
	Workspaces *workspace.Service
}

// New builds the application.
func New(opts Options) (*App, error) {
	resolver := opts.Env
	if resolver == nil {
		resolver = env.Default()
	}
	clock := opts.Clock
	if clock == nil {
		clock = clockx.System{}
	}
	idgen := opts.IDs
	if idgen == nil {
		idgen = ids.UUID{}
	}

	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		return nil, err
	}
	if err := paths.Ensure(); err != nil {
		return nil, err
	}

	root := opts.WorkspaceRoot
	if root == "" {
		root = resolver.String(env.KeyWorkspacePath, "")
	}
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root = cwd
	}
	root = filepath.Clean(root)

	// One lock and one index per workspace, shared by every repository, so two
	// collections writing to the same directory serialise against each other.
	lock := collections.NewPathLock(filepath.Join(paths.Runtime(), "locks"))
	index := fscollections.NewIndex()

	repos, err := newRepoSet(root, lock, index)
	if err != nil {
		return nil, err
	}

	logger := logging.New(logging.Config{})

	configSvc := config.NewService(fsconfig.FromPaths(paths))
	agentSvc := agent.NewService(repos.agents, clock)
	memorySvc := memory.NewService(memory.Deps{
		Repo:  repos.memories,
		Clock: clock,
		IDs:   idgen,
		Log:   logger,
	})
	workspaceSvc := workspace.NewService(workspace.Deps{
		Store:    fsworkspace.FromPaths(paths),
		FS:       fsworkspace.NewFiles(),
		Git:      gitcli.New(),
		Seeder:   newSeeder(lock, index, clock),
		Surveyor: newSurveyor(lock, index),
		Clock:    clock,

		WorkspacesDir: paths.Workspaces(),
		Active:        resolver.String(env.KeyWorkspaceID, ""),
		WorkingDir:    root,
	})

	reg := command.NewRegistry()
	config.Register(reg, configSvc)
	workspace.Register(reg, workspaceSvc)
	agent.Register(reg, agentSvc)
	memory.Register(reg, memorySvc)
	reg.Freeze()

	return &App{
		Paths:      paths,
		Workspace:  root,
		Registry:   reg,
		Config:     configSvc,
		Agents:     agentSvc,
		Memories:   memorySvc,
		Workspaces: workspaceSvc,
	}, nil
}

// repoSet holds the repositories bound to one workspace root.
type repoSet struct {
	agents   *fscollections.Repo[agent.Agent]
	memories *fscollections.Repo[memory.Memory]
}

func newRepoSet(root string, lock *collections.PathLock, index *fscollections.Index) (repoSet, error) {
	agentModel, err := collections.ModelOf[agent.Agent]("agents")
	if err != nil {
		return repoSet{}, err
	}
	memoryModel, err := collections.ModelOf[memory.Memory]("memories")
	if err != nil {
		return repoSet{}, err
	}
	return repoSet{
		agents: fscollections.New(root, agentModel,
			fscollections.WithLock[agent.Agent](lock),
			fscollections.WithIndex[agent.Agent](index),
		),
		memories: fscollections.New(root, memoryModel,
			fscollections.WithLock[memory.Memory](lock),
			fscollections.WithIndex[memory.Memory](index),
		),
	}, nil
}
