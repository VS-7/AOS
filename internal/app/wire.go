// Package app is the composition root: the one place where adapters are
// constructed and handed to the domain.
//
// It lives under internal/ rather than in cmd/ because two binaries need the
// same wiring — the daemon and, in the tests, the parity suite that runs one
// command through every surface. The rule it exists to keep is that no `new` of
// an adapter appears anywhere else.
package app

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/adapters/bleveindex"
	"github.com/OWNER/aos/internal/adapters/fsauth"
	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/adapters/fsworkspace"
	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/gateway"
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
	Chats      *chat.Service
	Auth       *auth.Service
	Gateway    *gateway.Service

	// env is kept so that Serve reads the same layered settings the rest of
	// the wiring did, rather than a second resolver that could disagree.
	env *env.Resolver

	// closers releases what New opened. It is a slice rather than a single
	// handle because the list grows with each phase, and a caller that has to
	// remember which resources exist is a caller that will forget one.
	closers []func() error
}

// Close releases everything the application opened.
//
// It is safe to call more than once and reports every failure rather than the
// first: a caller shutting down wants to know about all of them, and stopping
// at the first would leave the rest open.
func (a *App) Close() error {
	var errs []error
	for _, close := range a.closers {
		if err := close(); err != nil {
			errs = append(errs, err)
		}
	}
	a.closers = nil
	return errors.Join(errs...)
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

	// One lock and one record cache per workspace, shared by every repository,
	// so two collections writing to the same directory serialise against each
	// other and read the same parsed front matter.
	lock := collections.NewPathLock(filepath.Join(paths.Runtime(), "locks"))
	cache := fscollections.NewIndex()

	repos, err := newRepoSet(root, lock, cache)
	if err != nil {
		return nil, err
	}

	logger := logging.New(logging.Config{})
	active := resolver.String(env.KeyWorkspaceID, "")

	// The search index is per workspace and lives outside the user's repository
	// so it is never committed (ADR-0013). Without an active workspace there is
	// nowhere to put it, and recall scans — which is the documented fallback,
	// not a degraded mode.
	var searchIndex memory.Index
	var closers []func() error
	if active != "" {
		opened, err := bleveindex.Open(paths.Index(active))
		if err != nil {
			// A search index that will not open is a reason to scan, not a
			// reason to refuse to start.
			logger.Warn("continuing without a search index", "workspace", active, "err", err)
		} else {
			searchIndex = opened
			closers = append(closers, opened.Close)
		}
	}

	configSvc := config.NewService(fsconfig.FromPaths(paths))
	agentSvc := agent.NewService(repos.agents, clock)
	memorySvc := memory.NewService(memory.Deps{
		Repo:  repos.memories,
		Clock: clock,
		IDs:   idgen,
		Index: searchIndex,
		Log:   logger,
	})
	chatSvc := chat.NewService(chat.Deps{
		Repo:      repos.chats,
		Directory: newDirectory(agentSvc),
		Clock:     clock,
		IDs:       idgen,
		Log:       logger,
		// No dispatcher yet: a message is persisted and its recipient resolved,
		// and the turn starts when the agent runtime exists. The result already
		// reports that nothing was dispatched, so nothing here pretends
		// otherwise.
	})
	workspaceSvc := workspace.NewService(workspace.Deps{
		Store:    fsworkspace.FromPaths(paths),
		FS:       fsworkspace.NewFiles(),
		Git:      gitcli.New(),
		Seeder:   newSeeder(lock, cache, clock),
		Surveyor: newSurveyor(lock, cache),
		Clock:    clock,

		WorkspacesDir: paths.Workspaces(),
		Active:        active,
		WorkingDir:    root,
	})

	authSvc := auth.NewService(auth.Deps{
		Store: fsauth.FromPaths(paths),
		Clock: clock,
		IDs:   idgen,
	})

	// Supervision is bound to the same installation the rest of the process
	// serves: the record, the lock and the log all live under the state
	// directory, so two installations on one machine do not fight.
	gatewaySvc := gateway.NewService(gateway.Deps{
		Processes: supervise.NewProcesses(),
		Health:    supervise.NewHealth(),
		Store:     supervise.NewStore(filepath.Join(paths.GatewayDir(), "gateway.json")),
		Locker:    supervise.NewLock(paths.GatewayLock()),
		Resolver: supervise.Resolver{
			Explicit: resolver.String("DAEMON_PATH", ""),
			Args:     []string{"serve"},
			Log:      filepath.Join(paths.GatewayDir(), "gateway.log"),
		},
		Clock:   clock,
		Sleeper: supervise.Sleeper{},
		Log:     logger,
		Host:    resolver.String(env.KeyServerHost, env.DefaultServerHost),
		Port:    resolver.Int(env.KeyServerPort, build.Port),
	})

	reg := command.NewRegistry()
	config.Register(reg, configSvc)
	gateway.Register(reg, gatewaySvc)
	workspace.Register(reg, workspaceSvc)
	agent.Register(reg, agentSvc)
	memory.Register(reg, memorySvc)
	chat.Register(reg, chatSvc)
	reg.Freeze()

	return &App{
		Paths:      paths,
		Workspace:  root,
		Registry:   reg,
		Config:     configSvc,
		Agents:     agentSvc,
		Memories:   memorySvc,
		Workspaces: workspaceSvc,
		Chats:      chatSvc,
		Auth:       authSvc,
		Gateway:    gatewaySvc,
		env:        resolver,
		closers:    closers,
	}, nil
}

// repoSet holds the repositories bound to one workspace root.
type repoSet struct {
	agents   *fscollections.Repo[agent.Agent]
	memories *fscollections.Repo[memory.Memory]
	chats    *fscollections.Repo[chat.Chat]
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
	chatModel, err := collections.ModelOf[chat.Chat]("chats")
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
		chats: fscollections.New(root, chatModel,
			fscollections.WithLock[chat.Chat](lock),
			fscollections.WithIndex[chat.Chat](index),
		),
	}, nil
}
