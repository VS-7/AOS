// Command aosd owns the domain.
//
// In phase 2 it is not yet a daemon: it is the composition root that runs the
// command registry over the two surfaces this phase delivers — the terminal and
// MCP over stdio. The HTTP server, the WebSocket channel and the supervised
// lifecycle arrive in phase 4, and the domain-free `aos` client gets the same
// command tree over HTTP then.
//
// The split is not cosmetic. "Hexagonal e Regra de Dependência" forbids the CLI
// and the desktop binaries from linking domain packages, and that rule is a
// test. The binary that carries the domain is this one.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/OWNER/aos/internal/app"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/transport/clix"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

func main() { os.Exit(exitCode()) }

// isServeMode reports whether this invocation is the daemon rather than a
// command. It is a bare word rather than a flag because it is what the
// supervisor spawns, and a subcommand reads as one in a process listing.
func isServeMode(args []string) bool {
	return len(args) > 0 && args[0] == "serve"
}

// exitCode keeps os.Exit out of the function that defers, so the signal
// handler is always unregistered before the process leaves.
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
	application, err := app.New(app.Options{})
	if err != nil {
		return err
	}
	// The search index holds an open database. Leaving it open on exit leaves a
	// lock file behind that the next process has to recover from.
	defer func() { _ = application.Close() }()

	logger := logging.New(logging.Config{Level: "info", Format: "auto", TTY: isTTY()})
	ctx = logging.Into(ctx, logger)

	// Repair the permissions of any secret file looser than 0600, and say so:
	// silently tightening a file is still a change to the user's machine.
	for _, r := range corecfg.AuditSecrets(application.Paths.SecretFiles()...) {
		logger.Warn("tightened the permissions of a secret file",
			"path", r.Path, "was", r.Was.String(), "now", r.Now.String(), "enforced", r.Enforced)
	}

	if isMCPMode(args) {
		cfg, err := application.Config.Raw(ctx)
		if err != nil {
			return err
		}
		return mcpserver.ServeStdio(ctx, mcpserver.Config{
			Registry:     application.Registry,
			Shape:        app.MCPShapeFrom(cfg),
			Instructions: app.MCPInstructions(),
		})
	}

	if isServeMode(args) {
		// The daemon runs until the context ends, which happens on SIGTERM —
		// the signal the gateway sends before it resorts to killing.
		return application.Serve(ctx, app.ServeOptions{Log: logger})
	}

	root := clix.NewRoot(clix.Config{Registry: application.Registry, IsTTY: isTTY})
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func isTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// isMCPMode reports whether the process should speak MCP on stdio instead of
// running a command. It is a flag rather than a subcommand because that is how
// every MCP client is configured to launch a server.
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
