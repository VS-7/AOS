package clix

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/pkg/skill"
)

// skillCommand is `aos self skill`: the published skill, from the binary onto
// the machine.
//
// The skill is compiled in (pkg/skill.Files), so this works offline and always
// installs the skill that matches the commands this build actually has —
// which is the property that makes a skill worth installing: an agent reading
// `references/tasks.md` is reading about the `tasks` group this daemon serves.
func skillCommand(cfg Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "skill",
		Short: "install the " + skill.Name + " skill into coding agents",
		Long: "The skill teaches any agent harness that reads SKILL.md files — Claude Code, " +
			"Codex, Cursor, Gemini CLI, OpenCode, and anything following the .agents convention — " +
			"how to operate this system. It ships inside this binary.",
	}
	c.AddCommand(skillInstallCommand(cfg))
	c.AddCommand(skillTargetsCommand(cfg))
	c.AddCommand(skillShowCommand(cfg))
	return c
}

func skillInstallCommand(cfg Config) *cobra.Command {
	var to []string
	var dirs []string
	var all bool

	c := &cobra.Command{
		Use:   "install",
		Short: "write the skill into one or more agents' skills directories",
		Long: "With --to, installs into the named agents (" + strings.Join(skill.TargetIDs(), ", ") + "), " +
			"creating their skills directory if needed. With --all, or with no flags, installs into " +
			"every agent that appears to be installed on this machine. --dir names any other " +
			"directory; the skill goes into <dir>/" + skill.Name + "/.",
		Example: "  aos self skill install\n" +
			"  aos self skill install --to claude-code --to codex\n" +
			"  aos self skill install --dir ./.claude/skills",
		Args: cobra.NoArgs,
	}
	c.Flags().StringArrayVar(&to, "to", nil, "an agent to install into (repeatable): "+strings.Join(skill.TargetIDs(), "|"))
	c.Flags().StringArrayVar(&dirs, "dir", nil, "a skills directory to install into (repeatable)")
	c.Flags().BoolVar(&all, "all", false, "install into every agent detected on this machine")

	c.RunE = func(cmd *cobra.Command, _ []string) error {
		out := resolveOutput(cmd, cfg)
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		targets := append([]string{}, dirs...)
		for _, id := range to {
			target, ok := skill.LookupTarget(home, skill.Name, id)
			if !ok {
				return writeError(cfg.Err, errUnknownSkillTarget(id), out)
			}
			targets = append(targets, target.Dir)
		}
		if all || (len(to) == 0 && len(dirs) == 0) {
			for _, target := range skill.Targets(home, skill.Name) {
				if target.Present {
					targets = append(targets, target.Dir)
				}
			}
		}
		if len(targets) == 0 {
			return writeError(cfg.Err, errNoSkillTarget(), out)
		}

		result, err := skill.Install(skill.Files, dedupe(targets), skill.Name)
		if err != nil {
			return err
		}
		return writeResult(cfg.Out, command.Wrap(result, nil), out)
	}
	return c
}

func skillTargetsCommand(cfg Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "targets",
		Short: "list the agents this machine could hold the skill for, and where",
		Args:  cobra.NoArgs,
	}
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return writeResult(cfg.Out, command.Wrap(skill.Targets(home, skill.Name), nil), resolveOutput(cmd, cfg))
	}
	return c
}

func skillShowCommand(cfg Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "show [reference]",
		Short: "print SKILL.md, or one of its reference files",
		Long: "Without an argument prints SKILL.md. With one — \"tasks\", \"memories\" — prints " +
			"references/<name>.md, the group's own documentation as an agent reads it.",
		Args: cobra.MaximumNArgs(1),
	}
	c.RunE = func(_ *cobra.Command, args []string) error {
		path := "SKILL.md"
		if len(args) == 1 {
			path = "references/" + strings.TrimSuffix(args[0], ".md") + ".md"
		}
		content, err := skill.Files.ReadFile(path)
		if err != nil {
			return fmt.Errorf("no such reference in the skill: %s", path)
		}
		_, err = fmt.Fprint(cfg.Out, string(content))
		return err
	}
	return c
}

func dedupe(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func errUnknownSkillTarget(id string) error {
	return apperr.New("SKILL_UNKNOWN_TARGET").
		Causer("clix.self.skill.install").
		Msgf("%q is not an agent this command knows how to install into", id).
		Issue("target", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "one of " + strings.Join(skill.TargetIDs(), ", ") + " — or name a directory with --dir",
			Command: "aos self skill targets",
		})
}

func errNoSkillTarget() error {
	return apperr.New("SKILL_NO_TARGET").
		Causer("clix.self.skill.install").
		Msgf("no coding agent was detected on this machine, so there is nowhere to install the skill").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "name one with --to, or a directory with --dir",
			Command: "aos self skill install --to claude-code",
		})
}
