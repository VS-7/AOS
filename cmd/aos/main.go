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
	"github.com/OWNER/aos/internal/transport/mcpproxy"
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
	supervisor := newGateway(resolver, paths, logger)
	gateway.Register(reg, supervisor)
	reg.Freeze()

	// `aos --mcp` is what an MCP client is configured to spawn. This binary
	// cannot host the tool surface itself — the registry needs the domain,
	// and the domain is in the daemon — so it forwards: the daemon's own
	// /mcp mount, mirrored on stdio, with the daemon started first if nothing
	// is answering. See internal/transport/mcpproxy for why this is a proxy
	// rather than `aosd --mcp` in disguise.
	if isMCPMode(args) {
		return serveMCP(ctx, resolver, paths, supervisor, logger)
	}

	// The daemon is built before the tree, not after: the built-ins that
	// *describe* the surface — `self tools`, `self llms` — have to ask it too.
	// Reading this binary's own registry instead is what made them answer with
	// four commands out of ~140 (defect #1).
	daemon := newDaemonClient(resolver, paths)

	cfg := clix.Config{Registry: reg, Daemon: daemon, IsTTY: isTTY, Out: os.Stdout, Err: os.Stderr}
	root := clix.NewRoot(cfg)

	// The rest of the tree — every domain command — is not linked into this
	// binary (see the package doc): it arrives from a running daemon, if one
	// answers within a few seconds. A daemon that is not up yet leaves root
	// exactly as NewRoot built it, which is what lets `gateway start` itself
	// run with nothing attached.
	discover, cancel := context.WithTimeout(ctx, 3*time.Second)
	attachErr := clix.AttachDaemon(discover, root, daemon, cfg)
	cancel()

	root.SetArgs(args)
	// A failed discovery is not reported here — `gateway start` has to work
	// with nothing attached, and printing a warning before every one of those
	// would be noise. It is reported at the only moment it explains anything:
	// when the command somebody asked for turns out to be one of the missing
	// ones.
	return clix.ExplainMissingTree(root.ExecuteContext(ctx), attachErr)
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
		// An external agent operating this installation through the terminal
		// says so with AOS_AGENT_ID, and is then that agent for every call —
		// which is what `agents_me` answers and what a memory belongs to.
		Agent: resolver.String(env.KeyAgentID, ""),
		// Where this terminal is standing. The daemon's own working directory
		// is wherever the supervisor launched it, so without this
		// `workspace introspect` registered that instead of the repository the
		// person was in.
		WorkingDir: workingDir(),
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

// isMCPMode reports whether the process should speak MCP on stdio. A flag
// rather than a subcommand because that is how every MCP client is
// configured to launch a server, and it is the same spelling `aosd` accepts.
func isMCPMode(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--mcp" {
			return true
		}
	}
	return false
}

// serveMCP runs the stdio proxy over the daemon this installation's gateway
// supervises, starting the daemon when nothing is answering yet — an editor
// that spawns `aos --mcp` on a machine where the application is not open
// should get the workspace, not an error about a process it has never heard
// of. Nothing here writes to stdout: that stream is the MCP channel.
func serveMCP(ctx context.Context, resolver *env.Resolver, paths corecfg.Paths, supervisor *gateway.Service, logger *slog.Logger) error {
	probe, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if state, err := supervisor.Status(probe, gateway.StatusInput{}); err != nil || !state.Healthy {
		if _, err := supervisor.Start(probe, gateway.StartInput{}); err != nil {
			logger.Warn("the daemon is not running and could not be started", "err", err)
		}
	}

	host := resolver.String(env.KeyServerHost, env.DefaultServerHost)
	port := resolver.Int(env.KeyServerPort, build.Port)
	token := resolver.String(env.KeyToken, "")
	if token == "" {
		if raw, err := os.ReadFile(paths.LocalToken()); err == nil {
			token = strings.TrimSpace(string(raw))
		}
	}
	return mcpproxy.Serve(ctx, mcpproxy.Options{
		Endpoint:  fmt.Sprintf("http://%s:%d/mcp", host, port),
		Token:     token,
		Workspace: resolver.String(env.KeyWorkspaceID, ""),
		Agent:     resolver.String(env.KeyAgentID, ""),
		Name:      build.Name,
		Version:   build.Current().Version,
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

// workingDir is where this terminal is standing, or empty when the directory
// cannot be read — a deleted working directory is not a reason to refuse every
// command, and the daemon falls back to its own resolution.
func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}
