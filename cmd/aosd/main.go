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
	"github.com/OWNER/aos/internal/core/build"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/logging"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/transport/clix"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

func main() { os.Exit(exitCode()) }

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
			Shape:        shapeFrom(cfg),
			Instructions: instructions(),
		})
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

func shapeFrom(cfg config.Config) mcpserver.Shape {
	switch cfg.MCP.ToolShape {
	case config.ToolShapeFlat:
		return mcpserver.ShapeFlat
	case config.ToolShapeBoth:
		return mcpserver.ShapeBoth
	default:
		return mcpserver.ShapeComposite
	}
}

func instructions() string {
	return build.DisplayName + " exposes the workspace as tools. Every call requires " +
		"`_reasoning`: say why the tool is being called now, what outcome you expect, " +
		"and the immediate next step. Call a composite tool with `schema: true` to read " +
		"an action's contract before executing it."
}
