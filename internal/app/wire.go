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

	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/config"
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
}

// App is everything wired together.
type App struct {
	Paths     corecfg.Paths
	Workspace string
	Registry  *command.Registry
	Config    config.Service
	Agents    *agent.Service
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

	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		return nil, err
	}
	if err := paths.Ensure(); err != nil {
		return nil, err
	}

	workspace := opts.WorkspaceRoot
	if workspace == "" {
		workspace = resolver.String(env.KeyWorkspacePath, "")
	}
	if workspace == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workspace = cwd
	}
	workspace = filepath.Clean(workspace)

	// One lock and one index per workspace, shared by every repository, so two
	// collections writing to the same directory serialise against each other.
	lock := collections.NewPathLock(filepath.Join(paths.Runtime(), "locks"))
	index := fscollections.NewIndex()

	agentModel, err := collections.ModelOf[agent.Agent]("agents")
	if err != nil {
		return nil, err
	}
	agentRepo := fscollections.New(workspace, agentModel,
		fscollections.WithLock[agent.Agent](lock),
		fscollections.WithIndex[agent.Agent](index),
	)

	configSvc := config.NewService(fsconfig.FromPaths(paths))
	agentSvc := agent.NewService(agentRepo, clock)

	reg := command.NewRegistry()
	config.Register(reg, configSvc)
	agent.Register(reg, agentSvc)
	reg.Freeze()

	return &App{
		Paths:     paths,
		Workspace: workspace,
		Registry:  reg,
		Config:    configSvc,
		Agents:    agentSvc,
	}, nil
}
