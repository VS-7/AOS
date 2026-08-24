// Package app is the composition root: the one place where adapters are
// constructed and handed to the domain.
//
// It lives under internal/ rather than in cmd/ because two binaries need the
// same wiring — the daemon and, in the tests, the parity suite that runs one
// command through every surface. The rule it exists to keep is that no `new` of
// an adapter appears anywhere else.
package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/OWNER/aos/internal/adapters/activitylog"
	"github.com/OWNER/aos/internal/adapters/artifactfiles"
	"github.com/OWNER/aos/internal/adapters/bleveindex"
	"github.com/OWNER/aos/internal/adapters/cliclient"
	"github.com/OWNER/aos/internal/adapters/cloudflaredproc"
	"github.com/OWNER/aos/internal/adapters/eventlog"
	"github.com/OWNER/aos/internal/adapters/fsauth"
	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/adapters/fsthemes"
	"github.com/OWNER/aos/internal/adapters/fsworkspace"
	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/adapters/liquidengine"
	"github.com/OWNER/aos/internal/adapters/mcpclient"
	"github.com/OWNER/aos/internal/adapters/openapiclient"
	"github.com/OWNER/aos/internal/adapters/osfile"
	"github.com/OWNER/aos/internal/adapters/releasesource"
	"github.com/OWNER/aos/internal/adapters/skillfetch"
	"github.com/OWNER/aos/internal/adapters/skillfiles"
	"github.com/OWNER/aos/internal/adapters/skillhooks"
	"github.com/OWNER/aos/internal/adapters/sqlitequeue"
	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/adapters/telegramapi"
	"github.com/OWNER/aos/internal/adapters/updateinstall"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/domain/bot"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/comment"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/file"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/instruction"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/model"
	"github.com/OWNER/aos/internal/domain/project"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/template"
	"github.com/OWNER/aos/internal/domain/theme"
	"github.com/OWNER/aos/internal/domain/todo"
	"github.com/OWNER/aos/internal/domain/toolset"
	"github.com/OWNER/aos/internal/domain/tunnel"
	"github.com/OWNER/aos/internal/domain/update"
	"github.com/OWNER/aos/internal/domain/view"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/prompt"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/session"
	"github.com/OWNER/aos/internal/runtime/subconscious"
	"github.com/OWNER/aos/internal/runtime/toolexec"
	"github.com/OWNER/aos/internal/runtime/worker"
	"github.com/OWNER/aos/internal/transport/realtime"
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

	// Files backs the UI file explorer and editor. Unlike everything else
	// here, it is not reached through Registry — see its own construction
	// below for why.
	Files *file.Service

	// Hooks is the bus the nine events are emitted on. A skill registers a
	// handler here; nothing else writes to the audit log.
	Hooks *event.Service

	// Approvals is the channel a hook's "ask" reaches a person through.
	Approvals *event.Broker

	// Runtime executes turns. It is exported so a test can run one and assert
	// on the result rather than on a goroutine it has to wait for.
	Runtime *session.Runner

	// The continuity aggregates: work that outlives a conversation.
	Tasks      *task.Service
	Todos      *todo.Service
	Comments   *comment.Service
	Routines   *routine.Service
	Activities *activity.Service
	Jobs       *job.Service

	// Themes is the appearance of the application: the built-in presets and
	// the user's, with the tokens every component reads.
	Themes *theme.Service

	// CollectionRegistry is what dynamic collections are registered into. It
	// is exported for the same reason Clock is: Serve and the watcher build
	// on it, and a test needs to assert that a collection an agent created
	// is actually reachable.
	//
	// It is deliberately not called Collections — that name belongs to the
	// service, and two fields one letter apart is how a test ends up
	// asserting against the wrong one.
	CollectionRegistry *collections.Registry

	// Collections is the collection domain: declarations and their records.
	Collections *collection.Service

	// Views, Toolsets and Skills complete the ecosystem slice.
	Views    *view.Service
	Toolsets *toolset.Service
	Skills   *skill.Installer

	// The eight domains Phase 8 declared alongside the ecosystem core, built
	// in a later slice of the same phase — see docs/08 - Entrega/Roteiro de
	// Fases.md's "Fora do núcleo, declarado".
	Artifacts *artifact.Service

	// ArtifactFiles resolves a path inside one artifact's own directory —
	// Serve's HTTP layer uses it to serve an artifact's files, the same
	// containment Artifacts' own Create/Ensure already enforces. It is not
	// reached through Registry, the same reason Files (the file explorer)
	// is not.
	ArtifactFiles *artifactfiles.Files

	Goals        *goal.Service
	Instructions *instruction.Service
	Marketplace  *marketplace.Service
	Projects     *project.Service
	Templates    *template.Service
	Tunnel       tunnel.Service

	// Update is Fase 9's own domain — keeps aos, aosd and aos-desktop on one
	// verified version. Built after Queue (below) since it needs the same
	// active-work count Apply waits on.
	Update update.Service

	// Bots has no command-registry surface — configuring an external channel
	// is a human action, not an agent capability — so it is reached only
	// through Serve's webhook route and its own boot-time RegisterAll.
	Bots *bot.Registry

	// Watcher notices a collection or view declaration that reached disk
	// without going through this system's own Create — a hand edit, a
	// restored backup, a checked-out branch — and registers or re-publishes
	// it live. Serve starts it; a one-shot CLI process must not run a
	// long-lived watch loop, so New only builds it.
	Watcher *fscollections.Watcher

	// Queue is the durable store behind the worker. It is exported so a test
	// can enqueue and drain deterministically rather than waiting on a tick.
	Queue job.Queue

	// Worker drains the queue and runs the periodic tick. It is not started by
	// New: a CLI process that runs one command must not begin executing the
	// daemon's backlog.
	Worker *worker.Pool

	// Subconscious is the background observer. Exported so a test can run one
	// pass and assert on the memory it formed.
	Subconscious *subconscious.Observer

	// Events is the server-push hub. It exists whether or not anything is
	// listening, so a publisher never has to check.
	Events *realtime.Hub

	// Clock is the one every service here was built with. It is exported for
	// the same reason env is kept: Serve builds transports of its own, and a
	// transport that read the wall clock directly would disagree with the
	// domain that issued the record it is describing.
	Clock clockx.Clock

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

	// The runtime registry every dynamic collection — one an agent declares,
	// or one a skill brings — is registered into. Built once, here, and
	// threaded through collection.Service and, later, Serve and the watcher
	// (see the CollectionRegistry field's own doc comment on App).
	collReg := collections.NewRegistry()

	logger := logging.New(logging.Config{})
	active := resolver.String(env.KeyWorkspaceID, "")
	events := realtime.NewHub(logger, clock)

	// Repositories are built only now, after events exists: the four
	// ecosystem natives (collections, views, toolsets, skills) publish a
	// Changed event on every write, and NewRepoSet needs the hub to build
	// the publisher that carries it — see WithPublisher below.
	repos, err := newRepoSet(root, lock, cache, collectionPublisher{hub: events, workspace: active})
	if err != nil {
		return nil, err
	}

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
	// The dispatcher is a late binding on purpose: the runtime needs the chat
	// service to write an answer back, and the chat service needs the runtime
	// to start a turn. One of the two has to be handed to the other after both
	// exist, and a pointer set once at boot is a smaller price than a third
	// object between them.
	dispatch := &lateDispatcher{}
	chatSvc := chat.NewService(chat.Deps{
		Repo:       repos.chats,
		Directory:  newDirectory(agentSvc),
		Dispatcher: dispatch,
		Clock:      clock,
		IDs:        idgen,
		Log:        logger,
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

	// The UI file explorer and editor. It has no command group and no MCP
	// tools — see File (Go)'s "não tem grupo de comando" — so it is wired
	// straight to its own HTTP router in Serve rather than through the
	// command registry every other domain here goes through.
	fileSvc := file.NewService(file.Deps{
		FS:         osfile.New(),
		Git:        gitcli.New(),
		Workspaces: workspaceRoot{workspaceSvc},
	})

	// The appearance of the application. The built-in presets are embedded, so
	// a broken one is a build failure rather than a runtime surprise — which is
	// why this is the one service whose construction can refuse to start.
	themeSvc, err := theme.NewService(theme.Deps{
		Store: fsthemes.New(paths.Themes()),
		Log:   logger,
	})
	if err != nil {
		return nil, err
	}

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

	// The hook bus and the approval channel. The log is append-only and lives
	// beside the agent it records, which is what makes it reviewable in the
	// same place as everything else the agent owns.
	hookBus := event.NewService(event.Deps{
		Log:    eventlog.New(root),
		Clock:  clock,
		IDs:    idgen,
		Logger: logger,
	})
	broker := event.NewBroker(event.BrokerDeps{
		Clock: clock, IDs: idgen,
		Notifier: approvalNotifier{hub: events, workspace: active},
	})
	closers = append(closers, func() error { broker.Close(); return nil })

	// The continuity aggregates. The order matters in one place only: the task
	// service needs the plan, and the plan needs to know a task exists, so the
	// two are built with the todo service first and its parent set after.
	activityLog := activitylog.New(root)
	activitySvc := activity.NewService(activity.Deps{
		Log:    activityLog,
		Read:   activitylog.NewReadStore(root),
		Clock:  clock,
		IDs:    idgen,
		Sinks:  []activity.Sink{realtimeSink{hub: events, workspace: active}},
		Logger: logger,
	})

	todoSvc := todo.NewService(todo.Deps{Repo: repos.todos, Clock: clock, IDs: idgen, Log: logger})
	taskSvc := task.NewService(task.Deps{
		Repo:      repos.tasks,
		Plan:      planner{todos: todoSvc},
		Directory: assignees{agents: agentSvc},
		Worktrees: gitcli.NewWorktrees(gitcli.New(), root),
		Setup:     setupScript{agents: agentSvc, tmp: paths.Outputs(), log: logger},
		Policy: taskPolicy{
			workspaces: workspaceSvc, active: active,
			root: filepath.Join(paths.Data(), "worktrees"),
		},
		Notifier: taskActivity{activities: activitySvc, log: logger},
		Clock:    clock,
		IDs:      idgen,
		Log:      logger,
	})
	// The subcollections learn their parent now that it exists. Passing the
	// service into its own dependencies would be the other way to write this,
	// and it would let a todo move the task it belongs to.
	todoSvc.SetParent(taskSvc)
	commentSvc := comment.NewService(comment.Deps{
		Repo: repos.comments, Parent: taskSvc, Clock: clock, IDs: idgen, Log: logger,
	})

	routineSvc := routine.NewService(routine.Deps{
		Repo:      repos.routines,
		Runs:      repos.runs,
		Tokens:    tokens{},
		Directory: agentDirectory{agents: agentSvc},
		Notifier:  routineActivity{activities: activitySvc, log: logger},
		Clock:     clock,
		IDs:       idgen,
		Tick:      resolver.Duration(env.KeyJobsTick, job.DefaultTick),
		Log:       logger,
	})
	// The reactive loop closes here: a mutation publishes an activity, and the
	// activity fires the routines that were waiting for it.
	activitySvc.AddSink(routineTriggers{routines: routineSvc})

	reg := command.NewRegistry()

	// The ecosystem slice (Task 10): structured data an agent shapes at
	// runtime (collections), the screens composed over it (views), external
	// connections reached through one call (toolsets), and the packages that
	// bring all three together with agents and memories (skills).
	collectionSvc := collection.NewService(collection.Deps{
		Repo:     repos.collections,
		Registry: collReg,
		Clock:    clock,
		RecordRepos: fscollections.NewRecordRepos(root,
			fscollections.WithRecordLock(lock),
			fscollections.WithRecordIndex(cache),
			fscollections.WithRecordPublisher(collectionPublisher{hub: events, workspace: active}),
		),
		IDs: idgen,
	})
	viewSvc := view.NewService(view.Deps{
		Repo:        repos.views,
		Collections: collectionsForViews{svc: collectionSvc},
		Commands:    registryCommands{reg: reg},
		Clock:       clock,
	})
	// hooksAdapter turns a skill's declared hooks into live handlers on
	// hookBus — see internal/adapters/skillhooks. It is built here, before
	// skillInstaller, so the same instance is both the Hooks port below and
	// what reconcileHooks re-registers from disk once skillInstaller exists
	// too — see that call, further down.
	hooksAdapter := skillhooks.New(hookBus, root)
	skillInstaller := skill.NewInstaller(skill.Deps{
		Fetcher:     skillfetch.New(),
		Approver:    broker,
		Repo:        repos.skills,
		Collections: collectionSvc,
		Views:       viewSvc,
		Files:       skillfiles.New(root),
		Hooks:       hooksAdapter,
		Toolsets:    noopSkillToolsets{},
		Clock:       clock,
	})
	// A skill installed in an earlier run of this daemon has its hooks on
	// disk but nothing in the (freshly built, empty) event bus or in
	// hooksAdapter's own bookkeeping — see reconcileHooks' own doc comment
	// for why this cannot wait.
	reconcileHooks(context.Background(), skillInstaller, hooksAdapter, skillfetch.New(), root, logger)

	// Built after skillInstaller, not before: Network needs it to resolve a
	// skill's permissions.network at every rest-api/mcp-server::http call,
	// not only at install (see toolset's own "Estado atual" note on why the
	// install-time check alone was not enough).
	toolsetSvc := toolset.NewService(toolset.Deps{
		Repo: repos.toolsets,
		Adapters: toolset.Adapters{
			// custom decodes and lists but refuses Call as not yet available
			// — see toolset.Service.Call and toolset.Adapters' own doc: it
			// has no single adapter to write by definition. cli's own
			// adapter, cliclient, additionally requires the calling agent's
			// sandbox on ctx — internal/runtime/session attaches it once per
			// turn via internal/runtime/execguard, which is why a cli
			// toolset called any other way refuses rather than running
			// unguarded.
			toolset.MCPStdio: mcpclient.NewStdio,
			toolset.MCPHTTP:  mcpclient.NewHTTP,
			toolset.RESTAPI:  openapiclient.New,
			toolset.CLI:      cliclient.New,
		},
		Activities: activitySvc,
		Env:        resolver,
		Network:    skillNetwork{svc: skillInstaller},
		Clock:      clock,
		Log:        logger,
	})

	// The eight domains Phase 8 declared alongside the ecosystem core — see
	// the App field's own comment. Built here, after skillInstaller and
	// taskSvc both exist: marketplace installs through the former, and
	// goal/project both clear their reference off a task through the latter.
	// Shared with the HTTP layer below, not rebuilt there: artifactFiles.
	// Resolve is what internal/transport/artifactapi confines a served path
	// through, the same containment Ensure uses for scaffolding.
	artifactFiles := artifactfiles.New(root)
	artifactSvc := artifact.NewService(artifact.Deps{
		Repo:   repos.artifacts,
		Files:  artifactFiles,
		Hasher: artifact.Argon2Hasher{},
		Clock:  clock,
		IDs:    idgen,
		Log:    logger,
	})
	instructionSvc := instruction.NewService(instruction.Deps{
		Repo: repos.instructions, Clock: clock, Approver: broker,
	})
	templateSvc := template.NewService(template.Deps{
		Repo:   repos.templates,
		Engine: liquidengine.New(),
		// Workspaces and Files back templates_render's write:true — the same
		// workspaceRoot and osfile.FS the file explorer already uses, since
		// writing a template's rendered Output to disk is the same
		// workspace-confined filesystem operation the file domain already
		// performs, not a second implementation of it.
		Workspaces: workspaceRoot{workspaceSvc},
		Files:      osfile.New(),
		Clock:      clock,
	})
	goalSvc := goal.NewService(goal.Deps{
		Repo: repos.goals, Tasks: goalTasksAdapter{tasks: taskSvc}, Clock: clock,
	})
	projectSvc := project.NewService(project.Deps{
		Repo: repos.projects,
		Unlinkers: []project.Unlinker{
			taskProjectUnlinker{tasks: taskSvc},
			goalProjectUnlinker{goals: goalSvc},
		},
		Clock: clock,
	})
	// Read once, at boot: marketplace does not hold a live config.Service
	// itself, matching every other domain's narrowed dependencies. A config
	// file that fails to parse here already failed loudly earlier in New,
	// through configSvc's own construction path — Raw on a working service
	// with a missing/empty marketplace section just returns no registries,
	// which the domain already treats as a clean, CTA'd refusal to search.
	var marketplaceEntries []config.MarketplaceRegistry
	if cfg, err := configSvc.Raw(context.Background()); err == nil {
		marketplaceEntries = cfg.Marketplace.Registries
	}
	marketplaceRegs, marketplaceOrder := marketplaceRegistries(marketplaceEntries)
	marketplaceSvc := marketplace.NewService(marketplace.Deps{
		Registries: marketplaceRegs,
		Order:      marketplaceOrder,
		Installer:  skillInstaller,
	})
	tunnelSvc := tunnel.NewService(tunnel.Deps{
		Config: tunnelConfig{svc: configSvc},
		Runner: cloudflaredproc.New(),
		Clock:  clock,
		Log:    logger,
	})

	// Last of the eight: bot needs chatSvc, agentSvc and tunnelSvc, all
	// already built above. RegisterAll runs at boot in Serve, not here — see
	// serve.go's own comment on why, the same one-shot-CLI reason the
	// watcher and the worker are already split that way.
	botRegistry := bot.NewRegistry(bot.Deps{
		Providers: map[string]bot.Provider{"telegram": telegramapi.New()},
		Chats:     chatsForBot{svc: chatSvc},
		PublicURL: tunnelPublicURL{svc: tunnelSvc},
		Env:       resolver,
		Clock:     clock,
		Log:       logger,
	})

	// A dynamic collection a prior session created is a schema.json already on
	// disk when this process starts — not a change the watcher below will ever
	// see, since its directory walk arms fsnotify, it does not replay what was
	// already there. Reconciling once at boot is what makes it usable without
	// touching the file again.
	reconcileCollections(context.Background(), collectionSvc, collReg, logger)

	// The watcher: the schema.json case, for everything that reaches disk a
	// way other than the domain's own Create — see watch.go's own comment on
	// why Create itself needs none of this.
	watchPub := collectionPublisher{hub: events, workspace: active}
	watcher := fscollections.NewWatcher(root,
		fscollections.WithWatchPublisher(watchPub),
		fscollections.OnSchema(onSchemaChanged(collectionSvc, collReg, watchPub, logger)),
	)

	config.Register(reg, configSvc)
	registerApprovals(reg, broker)
	gateway.Register(reg, gatewaySvc)
	workspace.Register(reg, workspaceSvc)
	agent.Register(reg, agentSvc)
	memory.Register(reg, memorySvc)
	chat.Register(reg, chatSvc)
	task.Register(reg, taskSvc)
	todo.Register(reg, todoSvc)
	comment.Register(reg, commentSvc)
	routine.Register(reg, routineSvc)
	activity.Register(reg, activitySvc)
	theme.Register(reg, themeSvc)
	// The catalogue question — what can this installation actually reach —
	// answered by asking the providers rather than by a list in the build.
	model.Register(reg, model.NewService(model.Deps{
		Catalog: newModelCatalog(configSvc, filepath.Dir(paths.Root), clock),
		Log:     logger,
	}))
	collection.Register(reg, collectionSvc)
	view.Register(reg, viewSvc)
	toolset.Register(reg, toolsetSvc)
	skill.Register(reg, skillInstaller)
	artifact.Register(reg, artifactSvc)
	instruction.Register(reg, instructionSvc)
	template.Register(reg, templateSvc)
	goal.Register(reg, goalSvc)
	project.Register(reg, projectSvc)
	marketplace.Register(reg, marketplaceSvc)
	tunnel.Register(reg, tunnelSvc)

	assembler := prompt.NewAssembler(prompt.Deps{
		Clock: promptClock{clock: clock, zone: zoneFrom(configSvc, logger)},
		Reader: reader{
			workspaces: workspaceSvc, agents: agentSvc, memories: memorySvc,
			skills: skillInstaller, templates: templateSvc, views: viewSvc,
			goals: goalSvc, routines: routineSvc, projects: projectSvc,
			artifacts: artifactSvc, instructions: instructionSvc,
		},
		Log: logger,
	})
	runtime := session.New(session.Deps{
		Agents:   agentSvc,
		Chats:    chatSvc,
		Models:   models{config: configSvc, home: filepath.Dir(paths.Root)},
		Registry: reg,
		Bus:      hookBus,
		// The broker is the interactive channel. A run with nobody present
		// swaps it for the headless approver, which denies at once and says
		// that is why — see ADR-0007.
		Approver:      broker,
		Prompt:        assembler,
		Spiller:       toolexec.NewSpiller(paths.Outputs(), logger),
		Events:        publisher{hub: events, channel: workspaceForEvents(workspaceSvc, root, logger)},
		Bots:          botRegistry,
		Clock:         clock,
		IDs:           idgen,
		Log:           logger,
		WorkspaceRoot: root,
		WorkspaceID:   active,
		TmpDir:        paths.Outputs(),
	})
	dispatch.to = runtime

	// The queue and the pool that drains it. Opening the database is allowed to
	// fail: a process that cannot defer work should still be able to answer a
	// question, and the error says which capability was lost.
	var queue job.Queue
	var signatures subconscious.Signatures
	if opened, err := sqlitequeue.Open(sqlitequeue.Options{
		Path: paths.JobsDB(), Clock: clock.Now,
	}); err != nil {
		logger.Warn("continuing without a work queue: nothing will run in the background", "err", err)
	} else {
		queue = opened
		// The deduplication set shares the queue's database, which is what makes
		// it survive a restart — the divergence from the original recorded in
		// the Subconsciente (Go) note. Without a queue it degrades to the
		// in-process set, which behaves the same until the daemon restarts.
		signatures = opened.Signatures()
		closers = append(closers, opened.Close)
	}
	if signatures == nil {
		signatures = subconscious.NewMemorySignatures(clock.Now)
	}

	// The background observer. It runs on its own slot so a cheap model can
	// watch while an expensive one reasons, and it is handed to the runtime as
	// the thing that fires when a turn ends.
	observer := subconscious.New(subconscious.Deps{
		Models: subconsciousModels{
			config: configSvc, agents: agentSvc, home: filepath.Dir(paths.Root),
			build: func(provider, key string) (agentloop.LLMProvider, error) {
				return providers.Build(provider, providers.Config{
					APIKey: key, Home: filepath.Dir(paths.Root),
				})
			},
		},
		Memories:   memorySvc,
		Signatures: signatures,
		Clock:      clock.Now,
		Log:        logger,
	})
	runtime.SetObserver(observer)

	routineSvc.SetExecutor(routineExecutor{chats: chatSvc, runtime: runtime, log: logger})

	jobSvc := job.NewService(job.Deps{Queue: queue, Clock: clock, Log: logger})
	job.Register(reg, jobSvc)

	// Fase 9: same "installed next to this executable" convention
	// supervise.Resolver already uses to find aosd — aos, aosd and
	// aos-desktop are expected to live in one directory together (docs/08 -
	// Entrega/Build e Cross-Compile.md's own layout), so wherever this
	// process's own binary is IS where Apply swaps the others in too.
	binDir := paths.Root
	if self, err := os.Executable(); err == nil {
		binDir = filepath.Dir(self)
	}
	updateSvc := update.NewService(update.Deps{
		Source:     releasesource.New(resolver.String(env.KeyUpdateBaseURL, "")),
		Installer:  updateinstall.New(paths.UpdateDir(), binDir),
		Supervisor: updateSupervisor{svc: gatewaySvc},
		ActiveWork: updateActiveWork{queue: queue},
		Clock:      clock,
		Sleeper:    supervise.Sleeper{},
		Log:        logger,
		PublicKey:  releasePublicKey(),
	})
	update.Register(reg, updateSvc)

	reg.Freeze()

	var pool *worker.Pool
	if queue != nil {
		pool = worker.New(worker.Deps{
			Queue: queue,
			Handlers: map[string]job.Handler{
				kindTurn: turnHandler{runtime: runtime},
			},
			Ticks: []worker.Tick{
				{Name: "routines", Run: routineTick(routineSvc, active)},
				{Name: "activity-retention", Run: activityRetention(activitySvc)},
				{Name: "job-retention", Run: jobRetention(queue)},
			},
			Concurrency: resolver.Int(env.KeyJobsConcurrency, job.DefaultConcurrency),
			TickRate:    resolver.Duration(env.KeyJobsTick, job.DefaultTick),
			Lease:       job.DefaultLease,
			Heartbeat:   job.DefaultHeartbeat,
			Log:         logger,
		})
	}

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
		Files:      fileSvc,
		Hooks:      hookBus,
		Approvals:  broker,
		Runtime:    runtime,
		Events:     events,

		Tasks:        taskSvc,
		Todos:        todoSvc,
		Comments:     commentSvc,
		Routines:     routineSvc,
		Activities:   activitySvc,
		Jobs:         jobSvc,
		Themes:       themeSvc,
		Queue:        queue,
		Worker:       pool,
		Subconscious: observer,

		CollectionRegistry: collReg,
		Collections:        collectionSvc,
		Views:              viewSvc,
		Toolsets:           toolsetSvc,
		Skills:             skillInstaller,
		Watcher:            watcher,

		Artifacts:     artifactSvc,
		ArtifactFiles: artifactFiles,
		Goals:         goalSvc,
		Instructions:  instructionSvc,
		Marketplace:   marketplaceSvc,
		Projects:      projectSvc,
		Templates:     templateSvc,
		Tunnel:        tunnelSvc,
		Update:        updateSvc,
		Bots:          botRegistry,

		Clock:   clock,
		env:     resolver,
		closers: closers,
	}, nil
}

// workspaceRoot adapts workspace.Service to file.Workspaces: the active
// workspace's path, resolved fresh on every call rather than captured once,
// since the active workspace can change while the daemon runs.
type workspaceRoot struct{ svc *workspace.Service }

func (w workspaceRoot) Root(ctx context.Context) (string, error) {
	ws, err := w.svc.Get(ctx, workspace.GetInput{
		Reasoning: command.Reasoning{
			Reasoning: "resolving the workspace root the file explorer confines itself to",
		},
	})
	if err != nil {
		return "", err
	}
	return ws.Path, nil
}

// lateDispatcher breaks the cycle between the conversation and the runtime.
//
// It is a pointer set once at boot and never again, which is why it needs no
// lock: every read happens on a request, and the write happens before the
// first one can arrive.
type lateDispatcher struct{ to chat.Dispatcher }

func (d *lateDispatcher) Dispatch(ctx context.Context, in chat.Turn) (string, error) {
	if d.to == nil {
		return "", nil
	}
	return d.to.Dispatch(ctx, in)
}

// repoSet holds the repositories bound to one workspace root.
type repoSet struct {
	agents   *fscollections.Repo[agent.Agent]
	memories *fscollections.Repo[memory.Memory]
	chats    *fscollections.Repo[chat.Chat]
	tasks    *fscollections.Repo[task.Task]
	todos    *fscollections.Repo[todo.Todo]
	comments *fscollections.Repo[comment.Comment]
	routines *fscollections.Repo[routine.Routine]
	runs     *fscollections.Repo[routine.Run]

	// The ecosystem slice (Task 10): unlike everything above, these four
	// publish a Changed event on every write — see pub below and
	// collectionPublisher in ecosystem.go.
	collections *fscollections.Repo[collection.Collection]
	views       *fscollections.Repo[view.View]
	toolsets    *fscollections.Repo[toolset.Toolset]
	skills      *fscollections.Repo[skill.Skill]

	// The Phase 8 declared domains — see the App field's own comment.
	artifacts    *fscollections.Repo[artifact.Artifact]
	goals        *fscollections.Repo[goal.Goal]
	instructions *fscollections.Repo[instruction.Instruction]
	projects     *fscollections.Repo[project.Project]
	templates    *fscollections.Repo[template.Template]
}

// workspaceForEvents resolves the channel a turn publishes on when nothing
// pinned one — see publisher.channelFor for why that is the normal case.
//
// It matches on the directory this daemon was started against, which is the
// same thing `workspace_introspect` registers and therefore the workspace
// this process is actually serving. An installation accumulates workspaces —
// any directory ever introspected leaves one behind — so "the only one
// registered" is not a safe reading of "the one we mean": it is right until
// the second one appears, and then it silently resolves to nothing and every
// live update stops. Falling back to a lone workspace is still useful for the
// moment before this daemon's own has been registered.
//
// The answer is cached once found: a workspace's path does not move under a
// running daemon, and the alternative is a store read on every streamed chunk.
func workspaceForEvents(svc *workspace.Service, root string, log *slog.Logger) func(context.Context, string) string {
	want := filepath.Clean(root)
	var (
		mu     sync.Mutex
		cached string
	)
	return func(ctx context.Context, _ string) string {
		mu.Lock()
		defer mu.Unlock()
		if cached != "" {
			return cached
		}
		out, err := svc.List(ctx, workspace.ListInput{})
		if err != nil {
			return ""
		}
		for _, w := range out.Workspaces {
			if filepath.Clean(w.Path) == want {
				cached = w.ID
				return cached
			}
		}
		if len(out.Workspaces) == 1 {
			cached = out.Workspaces[0].ID
			return cached
		}
		// Nothing to publish against yet. Said once rather than per chunk:
		// silence here is what "the interface never updates" looks like from
		// the inside.
		log.Warn("no workspace matches this daemon's directory; live updates have no channel",
			"path", want, "registered", len(out.Workspaces))
		return ""
	}
}

func newRepoSet(root string, lock *collections.PathLock, index *fscollections.Index, pub collections.Publisher) (repoSet, error) {
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
	taskModel, err := collections.ModelOf[task.Task]("tasks")
	if err != nil {
		return repoSet{}, err
	}
	todoModel, err := collections.ModelOf[todo.Todo]("todos")
	if err != nil {
		return repoSet{}, err
	}
	commentModel, err := collections.ModelOf[comment.Comment]("comments")
	if err != nil {
		return repoSet{}, err
	}
	routineModel, err := collections.ModelOf[routine.Routine]("routines")
	if err != nil {
		return repoSet{}, err
	}
	runModel, err := collections.ModelOf[routine.Run]("runs")
	if err != nil {
		return repoSet{}, err
	}
	collectionModel, err := collections.ModelOf[collection.Collection]("collections")
	if err != nil {
		return repoSet{}, err
	}
	viewModel, err := collections.ModelOf[view.View]("views")
	if err != nil {
		return repoSet{}, err
	}
	toolsetModel, err := collections.ModelOf[toolset.Toolset]("toolsets")
	if err != nil {
		return repoSet{}, err
	}
	skillModel, err := collections.ModelOf[skill.Skill]("skills")
	if err != nil {
		return repoSet{}, err
	}
	artifactModel, err := collections.ModelOf[artifact.Artifact]("artifacts")
	if err != nil {
		return repoSet{}, err
	}
	goalModel, err := collections.ModelOf[goal.Goal]("goals")
	if err != nil {
		return repoSet{}, err
	}
	instructionModel, err := collections.ModelOf[instruction.Instruction]("instructions")
	if err != nil {
		return repoSet{}, err
	}
	projectModel, err := collections.ModelOf[project.Project]("projects")
	if err != nil {
		return repoSet{}, err
	}
	templateModel, err := collections.ModelOf[template.Template]("templates")
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
		tasks: fscollections.New(root, taskModel,
			fscollections.WithLock[task.Task](lock),
			fscollections.WithIndex[task.Task](index),
		),
		todos: fscollections.New(root, todoModel,
			fscollections.WithLock[todo.Todo](lock),
			fscollections.WithIndex[todo.Todo](index),
		),
		comments: fscollections.New(root, commentModel,
			fscollections.WithLock[comment.Comment](lock),
			fscollections.WithIndex[comment.Comment](index),
		),
		routines: fscollections.New(root, routineModel,
			fscollections.WithLock[routine.Routine](lock),
			fscollections.WithIndex[routine.Routine](index),
		),
		runs: fscollections.New(root, runModel,
			fscollections.WithLock[routine.Run](lock),
			fscollections.WithIndex[routine.Run](index),
		),
		collections: fscollections.New(root, collectionModel,
			fscollections.WithLock[collection.Collection](lock),
			fscollections.WithIndex[collection.Collection](index),
			fscollections.WithPublisher[collection.Collection](pub),
		),
		views: fscollections.New(root, viewModel,
			fscollections.WithLock[view.View](lock),
			fscollections.WithIndex[view.View](index),
			fscollections.WithPublisher[view.View](pub),
		),
		toolsets: fscollections.New(root, toolsetModel,
			fscollections.WithLock[toolset.Toolset](lock),
			fscollections.WithIndex[toolset.Toolset](index),
			fscollections.WithPublisher[toolset.Toolset](pub),
		),
		skills: fscollections.New(root, skillModel,
			fscollections.WithLock[skill.Skill](lock),
			fscollections.WithIndex[skill.Skill](index),
			fscollections.WithPublisher[skill.Skill](pub),
		),
		artifacts: fscollections.New(root, artifactModel,
			fscollections.WithLock[artifact.Artifact](lock),
			fscollections.WithIndex[artifact.Artifact](index),
			fscollections.WithPublisher[artifact.Artifact](pub),
		),
		goals: fscollections.New(root, goalModel,
			fscollections.WithLock[goal.Goal](lock),
			fscollections.WithIndex[goal.Goal](index),
			fscollections.WithPublisher[goal.Goal](pub),
		),
		// Instructions has no WithPublisher: nothing subscribes to instruction
		// changes over the realtime hub yet, matching agents/tasks/etc., not
		// the ecosystem four.
		instructions: fscollections.New(root, instructionModel,
			fscollections.WithLock[instruction.Instruction](lock),
			fscollections.WithIndex[instruction.Instruction](index),
		),
		projects: fscollections.New(root, projectModel,
			fscollections.WithLock[project.Project](lock),
			fscollections.WithIndex[project.Project](index),
			fscollections.WithPublisher[project.Project](pub),
		),
		templates: fscollections.New(root, templateModel,
			fscollections.WithLock[template.Template](lock),
			fscollections.WithIndex[template.Template](index),
			fscollections.WithPublisher[template.Template](pub),
		),
	}, nil
}
