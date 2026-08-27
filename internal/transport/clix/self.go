package clix

import (
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

// toolsCommand prints the published tool surface. It is what `mcp doctor` and a
// human debugging a client both need: the exact list, in the exact order.
func toolsCommand(cfg Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "tools",
		Short: "list the tool surface exactly as it is published to MCP clients",
	}
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		type row struct {
			Tool     string `json:"tool"`
			Summary  string `json:"summary"`
			Registry bool   `json:"agentRegistry"`
			Local    bool   `json:"local"`
		}
		var rows []row
		for _, d := range cfg.Registry.Sorted() {
			rows = append(rows, row{
				Tool: d.Key(), Summary: d.Summary(),
				Registry: d.InRegistry(), Local: d.Local(),
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
		var b strings.Builder
		fmt.Fprintf(&b, "# %s %s\n\n", build.DisplayName, build.Current().Version)

		for _, group := range cfg.Registry.Groups() {
			fmt.Fprintf(&b, "## %s\n\n", group.Name)
			if group.Summary != "" {
				fmt.Fprintf(&b, "%s\n\n", group.Summary)
			}
			if full && group.Doc != "" {
				fmt.Fprintf(&b, "%s\n\n", group.Doc)
			}
			cmds := make([]command.Descriptor, len(group.Commands))
			copy(cmds, group.Commands)
			sort.Slice(cmds, func(i, j int) bool { return cmds[i].Key() < cmds[j].Key() })
			for _, d := range cmds {
				fmt.Fprintf(&b, "- `%s` — %s\n", d.Key(), d.Summary())
				if !full {
					continue
				}
				if d.Doc() != "" {
					fmt.Fprintf(&b, "\n%s\n", d.Doc())
				}
				schema, err := json.MarshalIndent(d.InputSchema(), "  ", "  ")
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
