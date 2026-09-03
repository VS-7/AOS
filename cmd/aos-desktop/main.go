// Command aos-desktop is the desktop application.
//
// It is a window over the daemon, not a second copy of it. That is the
// dependency rule holding: a client binary may not link domain code, so
// everything the interface does goes over HTTP to the one process that owns the
// workspace. The gateway is the documented exception — it supervises an
// operating-system process and has to run locally to do that.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/transport/daemonclient"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// The frontend bundle, compiled into the binary. There is no directory to ship
// alongside the executable and nothing to go missing on somebody's machine.
//
// An embed pattern cannot reach outside its own package, so `task build:desktop`
// copies frontend/dist here first. The copy is generated and ignored by Git —
// committing a build output would put a stale interface in the history.
//
// What *is* versioned is dist/.gitkeep, and only because this directive is a
// compile error when it matches nothing: an ignored, generated directory made
// the whole module unbuildable from a clean checkout — CI, gopls and every
// editor included, not just this command. The `all:` prefix is what makes the
// pattern see a name that starts with a dot. When the real bundle is absent
// the window says so on its own: Wails looks for index.html in this FS and
// reports that it could not find one.
//
//go:embed all:dist
var assets embed.FS

func main() {
	log := logging.New(logging.Config{})
	resolver := env.Default()

	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		log.Error("the state directory could not be resolved", "err", err)
		os.Exit(1)
	}
	if err := paths.Ensure(); err != nil {
		log.Error("the state directory could not be created", "err", err)
		os.Exit(1)
	}

	// The directory this window is for, when there is one to be had.
	//
	// A working directory is only taken when it could plausibly be somebody's
	// project. An application bundle opened from Finder is started in "/", and
	// passing that on made the daemon serve the top of the disk: every
	// workspace-relative path resolved somewhere that cannot be read or
	// written, and agents, tasks, chats, goals and collections all answered
	// with a refusal for as long as the window was open.
	//
	// Empty is a real answer and the common one for an installed application:
	// it means this window names no directory, and the daemon opens the
	// workspace the installation already has (or makes the first one). See
	// internal/app's resolveWorkspaceRoot for the other end of this.
	root := strings.TrimSpace(resolver.String(env.KeyWorkspacePath, ""))
	if root == "" {
		if cwd, err := os.Getwd(); err == nil && corecfg.CanHoldWorkspace(cwd) {
			root = cwd
		}
	}
	if root != "" {
		root = filepath.Clean(root)
	}

	host := resolver.String(env.KeyServerHost, env.DefaultServerHost)
	port := resolver.Int(env.KeyServerPort, build.Port)
	address := fmt.Sprintf("http://%s:%d", host, port)

	daemon := daemonclient.New(daemonclient.Options{
		BaseURL: address,
		// No token at construction: the first boot no longer self-provisions
		// an account (see AuthService) — the window opens signed out, and
		// AuthService.Login/Onboarding fills this in once a person has.
		// AOS_TOKEN still overrides it, for pointing this window at a daemon
		// it did not start and is already signed into.
		Token:     resolver.String("TOKEN", ""),
		Workspace: resolver.String(env.KeyWorkspaceID, ""),
	})

	supervisor := gateway.NewService(gateway.Deps{
		Processes: supervise.NewProcesses(),
		Health:    supervise.NewHealth(),
		Store:     supervise.NewStore(filepath.Join(paths.GatewayDir(), "gateway.json")),
		Locker:    supervise.NewLock(paths.GatewayLock()),
		Resolver: supervise.Resolver{
			Explicit: resolver.String("DAEMON_PATH", ""),
			Args:     []string{"serve"},
			Log:      filepath.Join(paths.GatewayDir(), "gateway.log"),

			// The daemon is told which directory to serve rather than left to
			// infer it from a working directory it inherited from this
			// process — which, for an application launched from Finder, the
			// Dock or Spotlight, is "/". A daemon started there served the top
			// of the disk and refused every workspace-scoped command in the
			// application. Both are set: Dir for anything that reads the
			// working directory, and the environment variable because it is
			// what the daemon actually consults first.
			Dir: root,
			Env: daemonEnv(root),
		},
		Clock:   clockx.System{},
		Sleeper: supervise.Sleeper{},
		Log:     log,
		Host:    host,
		Port:    port,
	})

	// The window's event channel, relayed by this process — see
	// forwardRealtime for why the window cannot open it itself.
	realtimeCtx, stopRealtime := context.WithCancel(context.Background())
	// The exit status travels through a variable rather than an os.Exit at
	// the failure site, because an os.Exit there would skip the stop below and
	// leave the daemon holding an event socket for a window that is gone.
	// Registered first, so it runs last.
	exitCode := 0
	defer func() { os.Exit(exitCode) }()
	defer stopRealtime()
	// Assigned once the window exists, below, and before anything that could
	// call it is started — the `go ensureDaemon` below and AuthService's
	// afterAuth both happen after.
	var emitRealtime func(event any)
	// The relay follows the workspace. It used to be started once and never
	// again (sync.Once), so switching workspace in the interface left the
	// window listening to the events of the one it opened with: the board
	// and the conversations of the workspace you were actually looking at
	// never updated on their own. Re-pointing means dropping the old socket
	// and opening one for the new id.
	var realtimeMu sync.Mutex
	realtimeWorkspace := ""
	var stopCurrentRealtime context.CancelFunc
	startRealtime := func(workspaceID string) {
		if workspaceID == "" {
			return
		}
		realtimeMu.Lock()
		defer realtimeMu.Unlock()
		if workspaceID == realtimeWorkspace {
			return
		}
		if stopCurrentRealtime != nil {
			stopCurrentRealtime()
		}
		streamCtx, cancel := context.WithCancel(realtimeCtx)
		realtimeWorkspace, stopCurrentRealtime = workspaceID, cancel
		go forwardRealtime(streamCtx, address, workspaceID, daemon.Token, func(event any) {
			if emitRealtime != nil {
				emitRealtime(event)
			}
		}, log)
	}

	// The interface's own workspace choice, applied to the client every call
	// goes through and to the relay. Without this the bridge kept addressing
	// the workspace the window opened with, whatever the person picked.
	domainSvc := wailsvc.NewDomain(daemon)

	platform := &wailsPlatform{}
	// The system service starts without a workspace and is told which one it
	// is for as soon as that is known — see adopt below, and
	// SystemService.SetWorkspaceRoot. Handing it the working directory instead,
	// as this used to, confined "inside the workspace" to "/" for an
	// application launched from Finder, which confines nothing.
	systemSvc := wailsvc.NewSystem(platform, daemon, "")
	// Where this window was started, offered to the onboarding wizard as the
	// default folder for the first workspace. Registering it outright is what
	// this process used to do, before the person had named anything.
	systemSvc.SetLaunchDirectory(root)
	// What actually restarts the daemon. Inside the daemon `gateway_restart`
	// refuses — it would signal its own pid — so the button in Settings ›
	// Daemon has to reach the process that launched it, which is this one.
	systemSvc.SetSupervisor(daemonSupervisor{supervisor})

	// adopt is the one place the resolved workspace is applied, so the two
	// callers below — the daemon supervisor's first attempt and the one after
	// sign-in — cannot apply different halves of it.
	adopt := func(opened workspaceRef) {
		daemon.SetWorkspace(opened.ID)
		systemSvc.SetWorkspaceRoot(opened.Path)
		startRealtime(opened.ID)
	}

	// What the interface picks is adopted the same way, and that means all
	// three halves of it.
	//
	// This used to start the realtime relay and nothing else, so the file
	// root stayed wherever the window opened: after switching workspace, the
	// explorer, the editor and every path the shell was asked to open were
	// still confined to the previous one. It is also the path a first run
	// arrives by now — onboarding no longer registers anything, so the
	// workspace the wizard creates reaches this process here, and only here.
	domainSvc.OnWorkspaceChange(func(id string) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// Re-point what does not need the path first: a slow or failing
			// lookup must not leave the window addressing the old workspace.
			startRealtime(id)
			w, err := readWorkspace(ctx, daemon, id)
			if err != nil {
				log.Warn("the workspace the interface opened could not be read", "workspace", id, "err", err)
				return
			}
			adopt(workspaceRef{ID: w.ID, Path: w.Path})
		}()
	})

	desktop := application.New(application.Options{
		Name:        build.DisplayName,
		Description: "An operating system for AI agents.",
		Services: []application.Service{
			application.NewService(systemSvc),
			application.NewService(domainSvc),
			application.NewService(wailsvc.NewAuth(daemon, func(ctx context.Context, event wailsvc.AuthEvent) {
				// A successful login or onboarding is the first moment this
				// client can call anything past /api/auth — workspace
				// registration needed a token it didn't have until now.
				//
				// Which door it was decides whether anything may be
				// registered: after onboarding the wizard is still holding the
				// name and the copilot's settings, so this only adopts. See
				// openWorkspace.
				opened, err := openWorkspace(ctx, daemon, root, event)
				if err == nil {
					adopt(opened)
					return
				}
				if event == wailsvc.AuthOnboarding {
					// Expected on a first run, and not a problem: the wizard
					// creates the workspace moments later and says so through
					// DomainService.SetWorkspace.
					log.Debug("no workspace to adopt yet; the wizard is creating it", "path", root)
					return
				}
				log.Warn("could not open a workspace", "path", root, "err", err)
			})),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// The one daemon path the window has to serve itself: an <img>
			// cannot carry a bearer, and the bridge answers strings. See
			// bridgeContent.
			Middleware: bridgeContent(daemon, log),
		},
		LogLevel: slog.LevelWarn,
	})

	window := desktop.Window.NewWithOptions(windowOptions(address))
	platform.window = window
	emitRealtime = func(event any) { window.EmitEvent(RealtimeEventName, event) }

	// Files dragged onto the window.
	//
	// The interface cannot receive these itself. Every platform routes a file
	// drag to the native window before the WebView sees it — on macOS the
	// window is registered as the dragging destination outright — so the
	// HTML5 `drop` handlers the interface was ported with never fire, and a
	// file dropped onto the composer did nothing at all. Wails turns the drop
	// into a window event carrying the paths; this hands it on under a name
	// the interface listens for, the same way realtime events are relayed.
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		if event == nil || event.Context() == nil {
			return
		}
		paths := event.Context().DroppedFiles()
		if len(paths) == 0 {
			return
		}
		// Resolved against the workspace before it leaves this process: the
		// interface reads files through the daemon, which addresses them
		// relative to the workspace root, and this is the side that knows
		// where that is.
		window.EmitEvent(FilesDroppedEventName, systemSvc.ResolveDropped(paths))
	})

	// The deep link: aos://task/123 reaches the window rather than starting a
	// second copy of the application. macOS delivers a registered scheme as the
	// application's open-file event.
	desktop.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile,
		func(event *application.ApplicationEvent) {
			if event == nil || event.Context() == nil {
				return
			}
			window.EmitEvent("deeplink", event.Context().Filename())
		})

	// The daemon is asked to be running, not started blindly. Two things
	// supervising one process is how you end up with two of it.
	go ensureDaemon(supervisor, daemon, root, adopt, log)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		desktop.Quit()
	}()

	if err := desktop.Run(); err != nil {
		log.Error("the window closed with an error", "err", err)
		exitCode = 1
	}
}

// daemonSupervisor adapts the gateway service to the narrow slice the window's
// bridge needs: bring the daemon back.
type daemonSupervisor struct{ svc *gateway.Service }

func (d daemonSupervisor) Restart(ctx context.Context) error {
	_, err := d.svc.Restart(ctx, gateway.RestartInput{})
	return err
}

// ensureDaemon starts the daemon if nothing is already answering, then tries
// to register the workspace this desktop instance is for — which only
// succeeds once a person has signed in through AuthService, since a fresh
// installation has no account and no token yet. That first attempt is worth
// making anyway: a daemon a previous run of this same binary already
// authenticated against (AOS_TOKEN, or a login that outlived this process)
// can register the workspace immediately, with nobody watching a screen they
// don't need to see.
//
// A failure here does not stop the window from opening: an interface that says
// it cannot reach the daemon is more useful than an application that refuses to
// start and does not say why.
func ensureDaemon(supervisor *gateway.Service, client *daemonclient.Client, root string, adopt func(workspaceRef), log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	healthy := false
	if state, err := supervisor.Status(ctx, gateway.StatusInput{}); err == nil && state.Healthy {
		healthy = true
	} else if _, err := supervisor.Start(ctx, gateway.StartInput{}); err != nil {
		log.Error("the daemon is not running and could not be started", "err", err)
	} else {
		healthy = true
	}
	if !healthy {
		return
	}

	// The startup probe has the same permissions a sign-in does: an
	// installation that already has an account and a workspace is adopted, and
	// a window launched inside a repository registers it. Only onboarding is
	// restricted, and only because onboarding has a wizard still deciding.
	if opened, err := openWorkspace(ctx, client, root, wailsvc.AuthLogin); err != nil {
		log.Warn("could not open a workspace yet", "path", root, "err", err)
	} else {
		adopt(opened)
	}
}

// daemonEnv is the environment the supervised daemon is started with: this
// process's own, plus the workspace directory when this window names one.
//
// The whole environment is copied rather than a short list assembled, because
// the daemon reads the same layered settings this process does — the state
// directory, the port, the log level, a provider key — and a child handed only
// what somebody remembered to forward is a child that behaves differently for
// reasons nobody can see.
func daemonEnv(root string) []string {
	if root == "" {
		// Nil means "inherit", which is what os/exec does with no Env set, and
		// is the right answer when there is nothing to add.
		return nil
	}
	key := env.Key(env.KeyWorkspacePath) + "="
	out := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, key) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, key+root)
}

// openWorkspace decides which workspace this window addresses, and returns its
// id.
//
// It asks what is registered before registering anything. An installed
// application is opened without a directory in mind — there is no working
// directory that means anything, which is why root is usually empty here — and
// what the person expects is the workspace they were using, not a new one
// somewhere they did not choose. Only when this window does name a directory
// is that directory registered, which is the behaviour of opening a project:
// run the desktop inside a repository and the repository becomes the
// workspace.
//
// event is what separates the two doors, and it is the correction for the
// defect where the copilot was always called Atlas.
//
// AuthService runs this hook *inside* Onboarding, before the answer reaches
// the window — so on a first run this used to register a workspace while the
// wizard was still holding the name, the tone, the style and the autonomy the
// person had just chosen. The wizard's own workspace_create then found a
// workspace already there and did nothing, and the agent kept the default
// name for good: an agent's id is its file name, and agents_update cannot
// change an id.
//
// So onboarding adopts and never creates. The wizard owns the first workspace,
// because it is the only party that knows what to call it.
func openWorkspace(ctx context.Context, client *daemonclient.Client, root string, event wailsvc.AuthEvent) (opened workspaceRef, err error) {
	if event != wailsvc.AuthOnboarding && root != "" {
		return introspectWorkspace(ctx, client, root)
	}

	registered, err := listWorkspaces(ctx, client)
	if err != nil {
		return workspaceRef{}, err
	}
	for _, w := range registered {
		if !w.Archived {
			return workspaceRef{ID: w.ID, Path: w.Path}, nil
		}
	}

	if event == wailsvc.AuthOnboarding {
		// Nothing to adopt, and nothing to create: the wizard is about to do
		// that with the answers this side does not have. It reports which
		// workspace it made through DomainService.SetWorkspace, and adoptByID
		// picks it up from there.
		return workspaceRef{}, errNoWorkspaceYet()
	}

	// Nothing registered: the daemon serves a directory it chose for itself,
	// and asking it to introspect with no path is asking for exactly that one.
	return introspectWorkspace(ctx, client, "")
}

// errNoWorkspaceYet is the answer while the onboarding wizard is still
// deciding. It is not a failure — the caller logs it at debug and waits for
// the interface to say which workspace it created.
func errNoWorkspaceYet() error {
	return apperr.New("DESKTOP_NO_WORKSPACE_YET").
		Causer("main.openWorkspace").
		Msgf("this installation has no workspace yet; the onboarding wizard is creating it").
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "finish onboarding — the workspace it creates is the one this window will open",
		})
}

// workspaceRef is the workspace this window addresses: the id every call is
// scoped by, and the directory the system service confines paths to.
type workspaceRef struct {
	ID   string
	Path string
}

type registeredWorkspace struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Archived bool   `json:"archived"`
}

func listWorkspaces(ctx context.Context, client *daemonclient.Client) ([]registeredWorkspace, error) {
	input, err := json.Marshal(map[string]string{
		"_reasoning": "the desktop is starting and needs to know which workspaces exist",
	})
	if err != nil {
		return nil, err
	}
	raw, err := client.Invoke(ctx, "workspace_list", input)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data struct {
			Workspaces []registeredWorkspace `json:"workspaces"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("%s", envelope.Error.Message)
	}
	return envelope.Data.Workspaces, nil
}

// readWorkspace reads one registered workspace, for the id the interface
// reports. It is workspace_get rather than a scan of workspace_list because
// the interface has already resolved which one it means.
func readWorkspace(ctx context.Context, client *daemonclient.Client, id string) (registeredWorkspace, error) {
	input, err := json.Marshal(map[string]string{
		"workspace":  id,
		"_reasoning": "the window is following the workspace the interface opened",
	})
	if err != nil {
		return registeredWorkspace{}, err
	}
	raw, err := client.Invoke(ctx, "workspace_get", input)
	if err != nil {
		return registeredWorkspace{}, err
	}
	var envelope struct {
		Data  registeredWorkspace `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return registeredWorkspace{}, err
	}
	if envelope.Error != nil {
		return registeredWorkspace{}, fmt.Errorf("%s", envelope.Error.Message)
	}
	return envelope.Data, nil
}

// introspectWorkspace registers root as a workspace (idempotent — a directory
// already registered comes back unchanged, see workspace_introspect's own
// doc) and returns its id, so this desktop instance can address it without
// anyone picking a workspace by hand.
//
// An empty root is not an omission: workspace_introspect then registers the
// directory the daemon is serving, which is the one it resolved for itself.
func introspectWorkspace(ctx context.Context, client *daemonclient.Client, root string) (workspaceRef, error) {
	payload := map[string]string{
		"_reasoning": "the desktop is starting and needs to know which workspace it is for",
	}
	if root != "" {
		payload["path"] = root
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return workspaceRef{}, err
	}
	raw, err := client.Invoke(ctx, "workspace_introspect", input)
	if err != nil {
		return workspaceRef{}, err
	}
	var envelope struct {
		Data struct {
			Workspace registeredWorkspace `json:"workspace"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return workspaceRef{}, err
	}
	if envelope.Error != nil {
		return workspaceRef{}, fmt.Errorf("%s", envelope.Error.Message)
	}
	return workspaceRef{
		ID:   envelope.Data.Workspace.ID,
		Path: envelope.Data.Workspace.Path,
	}, nil
}
