// Command aos is the client: the terminal a person uses.
//
// It links no domain package except the gateway, and that is enforced by a test
// ("Hexagonal e Regra de Dependência"). The reason is not purity — it is that
// this binary must be able to start the daemon, and supervising a process is
// the one job that cannot be delegated to the process being supervised.
//
// Everything else it can do, it does by asking the daemon. The command tree for
// those arrives with the published surface manifest; until then this binary
// carries supervision and the build stamp.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/transport/clix"
	"github.com/OWNER/aos/internal/transport/daemonclient"
)

func main() { os.Exit(exitCode()) }

// exitCode keeps os.Exit out of the function that defers, so the signal handler
// is always unregistered before the process leaves.
func exitCode() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		if !clix.Silent(err) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string) error {
	// `version` answers without touching the filesystem, so it works on a
	// machine where the state directory cannot be created — which is exactly
	// the machine somebody is asking the version on.
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			return printVersion(args[1:], os.Stdout)
		}
	}

	resolver := env.Default()
	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}

	logger := logging.New(logging.Config{
		Level:  resolver.String(env.KeyLogLevel, env.DefaultLogLevel),
		Format: resolver.String(env.KeyLogFormat, env.DefaultLogFormat),
		TTY:    isTTY(),
	})
	ctx = logging.Into(ctx, logger)

	reg := command.NewRegistry()
	gateway.Register(reg, newGateway(resolver, paths, logger))
	reg.Freeze()

	cfg := clix.Config{Registry: reg, IsTTY: isTTY, Out: os.Stdout, Err: os.Stderr}
	root := clix.NewRoot(cfg)

	// The rest of the tree — every domain command — is not linked into this
	// binary (see the package doc): it arrives from a running daemon, if one
	// answers within a few seconds. A daemon that is not up yet leaves root
	// exactly as NewRoot built it, which is what lets `gateway start` itself
	// run with nothing attached.
	daemon := newDaemonClient(resolver, paths)
	discover, cancel := context.WithTimeout(ctx, 3*time.Second)
	clix.AttachDaemon(discover, root, daemon, cfg)
	cancel()

	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

// newDaemonClient points at the daemon this installation's gateway would
// itself start: same host and port newGateway supervises, same TOKEN
// environment variable the desktop honours, falling back to local.token —
// the credential onboarding writes once, on this machine, for exactly this
// reader (see authapi.Config.Paths' own doc comment).
func newDaemonClient(resolver *env.Resolver, paths corecfg.Paths) *daemonclient.Client {
	host := resolver.String(env.KeyServerHost, env.DefaultServerHost)
	port := resolver.Int(env.KeyServerPort, build.Port)
	address := fmt.Sprintf("http://%s:%d", host, port)

	token := resolver.String(env.KeyToken, "")
	if token == "" {
		if raw, err := os.ReadFile(paths.LocalToken()); err == nil {
			token = strings.TrimSpace(string(raw))
		}
	}

	return daemonclient.New(daemonclient.Options{
		BaseURL:   address,
		Token:     token,
		Workspace: resolver.String(env.KeyWorkspaceID, ""),
	})
}

// newGateway builds the supervisor over the installation this binary is
// pointed at. It is the one place this binary constructs an adapter, and the
// dependency rule allows it for the reason in the package comment.
func newGateway(resolver *env.Resolver, paths corecfg.Paths, logger *slog.Logger) *gateway.Service {
	return gateway.NewService(gateway.Deps{
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
		Log:     logger,
		Host:    resolver.String(env.KeyServerHost, env.DefaultServerHost),
		Port:    resolver.Int(env.KeyServerPort, build.Port),
	})
}

func printVersion(args []string, out *os.File) error {
	info := build.Current()
	for _, a := range args {
		if a == "--json" {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}
	}
	_, err := fmt.Fprintln(out, info.String())
	return err
}

func isTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
