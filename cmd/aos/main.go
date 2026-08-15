// Command aos is the CLI and, with --mcp, the stdio MCP server.
//
// Phase 0 ships only `version`: the command tree is derived from the command
// registry, which arrives with the Command Layer in phase 2. Until then this
// main stays deliberately dependency-free, so that the first binary of the
// project proves the build stamp works and nothing else.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/OWNER/aos/internal/core/build"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, out *os.File) error {
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "version", "--version", "-v":
		return printVersion(args[1:], out)
	case "", "help", "--help", "-h":
		return printUsage(out)
	default:
		return fmt.Errorf("unknown command %q (try `%s help`)", cmd, build.Name)
	}
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

func printUsage(out *os.File) error {
	_, err := fmt.Fprintf(out, `%s — %s

Usage:
  %s version [--json]    print the build stamp
  %s help                show this message
`, build.DisplayName, build.Current().String(), build.Name, build.Name)
	return err
}
