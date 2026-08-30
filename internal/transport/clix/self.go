package clix

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
)

// selfCommand groups the built-ins under a namespace of their own.
//
// In the original, the framework's builtin `skills` group shadows the domain
// group of the same name, so `fractal skills --help` shows `add` and `list` and
// the domain operations — create, install, discovery — cannot be reached from
// the CLI at all (defect #15). A namespace makes the collision impossible.
func selfCommand(cfg Config) *cobra.Command {
	self := &cobra.Command{
		Use:   "self",
		Short: "built-in commands of the CLI itself",
		Long: "Commands that operate the CLI rather than the domain. They live under " +
			"a namespace so that they can never shadow a domain group.",
	}
	self.AddCommand(completionsCommand())
	self.AddCommand(toolsCommand(cfg))
	self.AddCommand(llmsCommand(cfg))
	self.AddCommand(skillCommand(cfg))
	return self
}

func completionsCommand() *cobra.Command {
	c := &cobra.Command{
		Use:       "completions <bash|zsh|fish|powershell>",
		Short:     "print a shell completion script",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactArgs(1),
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
		case "zsh":
			return root.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unknown shell %q", args[0])
		}
	}
	return c
}

// manifester is a daemon that can hand over the whole published surface —
// documentation and input schemas included — rather than only the keys the
// command tree is built from. *daemonclient.Client is one.
//
// It is an optional interface rather than a method on wailsvc.Caller because
// routing a call and describing the surface are different jobs, and a caller
// that can only do the first is still a useful caller.
type manifester interface {
	Manifest(ctx context.Context) (command.Manifest, error)
}

// publishedSurface is the surface this binary should describe: the daemon's,
// merged with whatever this binary links in itself.
//
// The merge is the correction for defect #1. `self tools` promised "the tool
// surface exactly as it is published to MCP clients" and read the local
// registry, which in the terminal binary holds the four gateway commands —
// so a model or a client that trusted it saw under 5% of what exists. The
// local registry is still merged in because it is genuinely part of the
// surface this binary offers: `gateway start` has to work with no daemon
// running, which is exactly when there is nothing to ask.
func publishedSurface(ctx context.Context, cfg Config) command.Manifest {
	local := command.ManifestOf(cfg.Registry, build.Current().Version)

	source, ok := cfg.Daemon.(manifester)
	if !ok || cfg.Daemon == nil {
		return local
	}
	remote, err := source.Manifest(ctx)
	if err != nil || len(remote.Groups) == 0 {
		// A daemon that is not running leaves the terminal describing what it
		// can describe truthfully, which is its own commands.
		return local
	}
	return mergeManifests(local, remote)
}

// mergeManifests folds the local groups into the daemon's, keeping the local
// definition of a command both publish.
//
// `gateway` is the whole of that case: the daemon publishes it too, and the
// local one is the one that can actually start a process on this machine.
func mergeManifests(local, remote command.Manifest) command.Manifest {
	byGroup := map[string]*command.ManifestGroup{}
	order := []string{}
	add := func(g command.ManifestGroup, preferExisting bool) {
		existing, seen := byGroup[g.Name]
		if !seen {
			copied := g
			byGroup[g.Name] = &copied
			order = append(order, g.Name)
			return
		}
		known := map[string]bool{}
		for _, c := range existing.Commands {
			known[c.Key] = true
		}
		for _, c := range g.Commands {
			if known[c.Key] && preferExisting {
				continue
			}
			existing.Commands = append(existing.Commands, c)
		}
		if existing.Doc == "" {
			existing.Doc, existing.Summary, existing.Hint = g.Doc, g.Summary, g.Hint
		}
	}
	for _, g := range local.Groups {
		add(g, true)
	}
	for _, g := range remote.Groups {
		add(g, true)
	}

	sort.Strings(order)
	out := command.Manifest{Version: remote.Version, Groups: make([]command.ManifestGroup, 0, len(order))}
	if out.Version == "" {
		out.Version = local.Version
	}
	for _, name := range order {
		g := *byGroup[name]
		sort.Slice(g.Commands, func(i, j int) bool { return g.Commands[i].Name < g.Commands[j].Name })
		out.Groups = append(out.Groups, g)
	}
	return out
}

// toolsCommand prints the published tool surface. It is what `mcp doctor` and a
// human debugging a client both need: the exact list, in the exact order.
func toolsCommand(cfg Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "tools",
		Short: "list the tool surface exactly as it is published to MCP clients",
		Long: "List every command this installation publishes: the ones this binary links in " +
			"and the ones the daemon serves, which is nearly all of them. With no daemon " +
			"answering, only the local ones can be listed.",
	}
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		type row struct {
			Tool     string `json:"tool"`
			Summary  string `json:"summary"`
			Registry bool   `json:"agentRegistry"`
			Local    bool   `json:"local"`
		}
		surface := publishedSurface(cmd.Context(), cfg)
		commands := surface.Commands()
		sort.Slice(commands, func(i, j int) bool { return commands[i].Key < commands[j].Key })

		rows := make([]row, 0, len(commands))
		for _, d := range commands {
			rows = append(rows, row{
				Tool: d.Key, Summary: d.Summary,
				Registry: d.Registry, Local: d.Local,
			})
		}
		return writeResult(cfg.Out, command.Wrap(rows, nil), resolveOutput(cmd, cfg))
	}
	return c
}

// llmsCommand renders the manifest a model reads to learn the surface in one
// request, mirroring the original's --llms and --llms-full.
func llmsCommand(cfg Config) *cobra.Command {
	var full bool
	c := &cobra.Command{
		Use:   "llms",
		Short: "print a manifest of the whole surface, for a model to read",
	}
	c.Flags().BoolVar(&full, "full", false, "include the full documentation and input schema of every command")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		surface := publishedSurface(cmd.Context(), cfg)

		var b strings.Builder
		version := surface.Version
		if version == "" {
			version = build.Current().Version
		}
		fmt.Fprintf(&b, "# %s %s\n\n", build.DisplayName, version)

		for _, group := range surface.Groups {
			fmt.Fprintf(&b, "## %s\n\n", group.Name)
			if group.Summary != "" {
				fmt.Fprintf(&b, "%s\n\n", group.Summary)
			}
			if full && group.Doc != "" {
				fmt.Fprintf(&b, "%s\n\n", group.Doc)
			}
			cmds := make([]command.ManifestCommand, len(group.Commands))
			copy(cmds, group.Commands)
			sort.Slice(cmds, func(i, j int) bool { return cmds[i].Key < cmds[j].Key })
			for _, d := range cmds {
				fmt.Fprintf(&b, "- `%s` — %s\n", d.Key, d.Summary)
				if !full {
					continue
				}
				if d.Doc != "" {
					fmt.Fprintf(&b, "\n%s\n", d.Doc)
				}
				if d.InputSchema == nil {
					continue
				}
				schema, err := json.MarshalIndent(d.InputSchema, "  ", "  ")
				if err == nil {
					fmt.Fprintf(&b, "\n  ```json\n  %s\n  ```\n", schema)
				}
			}
			b.WriteString("\n")
		}
		_, err := fmt.Fprint(cfg.Out, b.String())
		return err
	}
	return c
}
