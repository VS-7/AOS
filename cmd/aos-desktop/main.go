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
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/OWNER/aos/internal/adapters/supervise"
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

	root := resolver.String(env.KeyWorkspacePath, "")
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	root = filepath.Clean(root)

	host := resolver.String(env.KeyServerHost, env.DefaultServerHost)
	port := resolver.Int(env.KeyServerPort, build.Port)
	address := fmt.Sprintf("http://%s:%d", host, port)

	daemon := daemonclient.New(daemonclient.Options{
		BaseURL:   address,
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
		},
		Clock:   clockx.System{},
		Sleeper: supervise.Sleeper{},
		Log:     log,
		Host:    host,
		Port:    port,
	})

	platform := &wailsPlatform{}
	desktop := application.New(application.Options{
		Name:        build.DisplayName,
		Description: "An operating system for AI agents.",
		Services: []application.Service{
			application.NewService(wailsvc.NewSystem(platform, daemon, root)),
			application.NewService(wailsvc.NewDomain(daemon)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		LogLevel: slog.LevelWarn,
	})

	window := desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     build.DisplayName,
		Width:     1440,
		Height:    900,
		MinWidth:  960,
		MinHeight: 600,

		// Frameless with a translucent background, as in the original. The
		// interface draws its own chrome, and the sidebar leaves room for the
		// traffic lights macOS still draws over it.
		Frameless:        true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		BackgroundType:   application.BackgroundTypeTranslucent,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 40,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	platform.window = window

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
	go ensureDaemon(supervisor, log)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		desktop.Quit()
	}()

	if err := desktop.Run(); err != nil {
		log.Error("the window closed with an error", "err", err)
		os.Exit(1)
	}
}

// ensureDaemon starts the daemon if nothing is already answering.
//
// A failure here does not stop the window from opening: an interface that says
// it cannot reach the daemon is more useful than an application that refuses to
// start and does not say why.
func ensureDaemon(supervisor *gateway.Service, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if state, err := supervisor.Status(ctx, gateway.StatusInput{}); err == nil && state.Healthy {
		return
	}
	if _, err := supervisor.Start(ctx, gateway.StartInput{}); err != nil {
		log.Error("the daemon is not running and could not be started", "err", err)
	}
}
