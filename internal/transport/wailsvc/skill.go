package wailsvc

import (
	"context"
	"os"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/pkg/skill"
)

// SkillTargets lists the coding agents the skill can be installed into on
// this machine, and which of them already hold it. It is what the
// "Add skill to…" menu in the Developers settings is drawn from.
func (s *SystemService) SkillTargets(context.Context) ([]skill.Target, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errNoHome(err)
	}
	return skill.Targets(home, skill.Name), nil
}

// InstallSkill writes the skill compiled into this application into one
// agent's skills directory — "claude-code", "codex", … — or, for "all",
// into every agent that appears to be installed. The desktop equivalent of
// `aos self skill install`, and the same code underneath.
func (s *SystemService) InstallSkill(_ context.Context, target string) (skill.InstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return skill.InstallResult{}, errNoHome(err)
	}
	target = strings.TrimSpace(target)

	var dirs []string
	switch target {
	case "", "all":
		for _, t := range skill.Targets(home, skill.Name) {
			if t.Present {
				dirs = append(dirs, t.Dir)
			}
		}
		if len(dirs) == 0 {
			return skill.InstallResult{}, errNoSkillTargets()
		}
	default:
		t, ok := skill.LookupTarget(home, skill.Name, target)
		if !ok {
			return skill.InstallResult{}, errUnknownSkillTarget(target)
		}
		dirs = []string{t.Dir}
	}
	return skill.Install(skill.Files, dirs, skill.Name)
}

func errNoHome(cause error) error {
	return apperr.New("SYSTEM_NO_HOME").
		Causer("wailsvc.SystemService").
		Msgf("the home directory could not be determined").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnknownSkillTarget(target string) error {
	return apperr.New("SKILL_UNKNOWN_TARGET").
		Causer("wailsvc.SystemService.InstallSkill").
		Msgf("%q is not an agent this application knows how to install the skill into", target).
		Issue("target", target).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "one of " + strings.Join(skill.TargetIDs(), ", ")})
}

func errNoSkillTargets() error {
	return apperr.New("SKILL_NO_TARGET").
		Causer("wailsvc.SystemService.InstallSkill").
		Msgf("no coding agent was detected on this machine").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "pick one agent explicitly, or install from the terminal into any directory",
			Command: "aos self skill install --dir <path>",
		})
}
