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
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/adapters/activitylog"
	"github.com/OWNER/aos/internal/adapters/bleveindex"
	"github.com/OWNER/aos/internal/adapters/eventlog"
	"github.com/OWNER/aos/internal/adapters/fsauth"
	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/adapters/fsworkspace"
	"github.com/OWNER/aos/internal/adapters/gitcli"
	"github.com/OWNER/aos/internal/adapters/sqlitequeue"
	"github.com/OWNER/aos/internal/adapters/supervise"
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
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/comment"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/todo"
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
	events := realtime.NewHub(logger, clock)

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

	assembler := prompt.NewAssembler(prompt.Deps{
		Clock:  promptClock{clock: clock, zone: zoneFrom(configSvc, logger)},
		Reader: reader{workspaces: workspaceSvc, agents: agentSvc, memories: memorySvc},
		Log:    logger,
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
		Events:        publisher{hub: events},
		Clock:         clock,
		IDs:           idgen,
		Log:           logger,
		WorkspaceRoot: root,
		WorkspaceID:   active,
		TmpDir:        paths.Outputs(),
	})
	dispatch.to = runtime

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
		Signatures: subconscious.NewMemorySignatures(clock.Now),
		Clock:      clock.Now,
		Log:        logger,
	})
	runtime.SetObserver(observer)

	routineSvc.SetExecutor(routineExecutor{chats: chatSvc, runtime: runtime, log: logger})

	// The queue and the pool that drains it. Opening the database is allowed to
	// fail: a process that cannot defer work should still be able to answer a
	// question, and the error says which capability was lost.
	var queue job.Queue
	if opened, err := sqlitequeue.Open(sqlitequeue.Options{
		Path: paths.JobsDB(), Clock: clock.Now,
	}); err != nil {
		logger.Warn("continuing without a work queue: nothing will run in the background", "err", err)
	} else {
		queue = opened
		closers = append(closers, opened.Close)
	}

	jobSvc := job.NewService(job.Deps{Queue: queue, Clock: clock, Log: logger})
	job.Register(reg, jobSvc)
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
		Queue:        queue,
		Worker:       pool,
		Subconscious: observer,

		env:     resolver,
		closers: closers,
	}, nil
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
	}, nil
}
