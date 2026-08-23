// Command genskill produces pkg/skill/SKILL.md and pkg/skill/references/,
// the skill that teaches an agent to operate this system — see
// docs/09 - Skill/Especificação da Skill.md.
//
// skillMD below is the one piece pkg/skill.Generate cannot derive from the
// registry: SKILL.md is curated (session protocol, routing, hard rules),
// not mechanical — see docs/09 - Skill/SKILL (gerada).md, whose own
// "Decisões" section is the reason this lives here rather than as a
// checked-in template file: hand-editing prose in a .go string is the same
// motion as hand-editing prose anywhere else in this codebase's own
// GroupDoc.Doc fields, not a special case.
package main

import (
	"fmt"
	"os"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/pkg/skill"
)

func main() {
	out := "pkg/skill"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	// Same reasoning as tools/genschema: the registry is built the way the
	// daemon builds it, so what gets published is what the application
	// actually has, not a second description that could drift.
	built, err := app.New(app.Options{WorkspaceRoot: os.TempDir()})
	if err != nil {
		fail(err)
	}
	defer func() { _ = built.Close() }()

	if err := generate(built.Registry, out); err != nil {
		fail(err)
	}

	groups := built.Registry.Groups()
	fmt.Fprintf(os.Stderr, "genskill: SKILL.md + %d references → %s\n", len(groups), out)

	// Disclosed, not enforced — see skill.MissingSections' own doc comment
	// on why this warns instead of failing the build: fixing it means
	// reviewing what every domain's Doc should actually say, not a
	// mechanical patch.
	missing := skill.MissingSections(registryAdapter{built.Registry})
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "genskill: %d group(s) missing required sections (docs/09 - Skill/Especificação da Skill.md):\n", len(missing))
		for _, g := range groups {
			if sections, ok := missing[g.Name]; ok {
				fmt.Fprintf(os.Stderr, "  %-14s missing %v\n", g.Name, sections)
			}
		}
	}
}

// generate is the one place main() and TestGeneratedSkillIsCommitted both
// call, so the CI gate proves exactly what running the command produces —
// not a second description of it that could drift.
func generate(reg *command.Registry, dir string) error {
	return skill.Generate(registryAdapter{reg}, dir, skill.Options{SkillMD: skillMD})
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "genskill:", err)
	os.Exit(1)
}

// registryAdapter is the seam docs/09 - Skill/Especificação da Skill.md's
// own "pkg/skill sem dependência do projeto" decision calls for: pkg/skill
// knows nothing of command.Registry, so the translation happens here, in
// the one place that is allowed to know about both.
type registryAdapter struct{ reg *command.Registry }

func (a registryAdapter) Groups() []skill.Group {
	groups := a.reg.Groups()
	out := make([]skill.Group, 0, len(groups))
	for _, g := range groups {
		cmds := make([]skill.Command, 0, len(g.Commands))
		for _, d := range g.Commands {
			examples := make([]skill.Example, 0, len(d.Examples()))
			for _, ex := range d.Examples() {
				examples = append(examples, skill.Example{Description: ex.Description, Input: ex.Input})
			}
			cmds = append(cmds, skill.Command{
				Key: d.Key(), Summary: d.Summary(), Doc: d.Doc(), Examples: examples,
			})
		}
		out = append(out, skill.Group{
			Name: g.Name, Summary: g.Summary, Doc: g.Doc, Commands: cmds,
		})
	}
	return out
}

const skillMD = `---
name: aos
description: Use when you need persistent memory across sessions, lifecycle-managed
  tasks, specialized agents, structured data collections, views, scheduled
  routines, or installable skills. Triggers: "remember this", "what did we
  decide about X", "create a task", "delegate to", "schedule", "build a CRM".
---

# AOS

Infrastructure layer that gives agents persistent memory, lifecycle
execution and continuity across sessions.

## Session start — mandatory

Before any substantive work:

1. ` + "`agents_me`" + ` — find out who you are in this workspace
2. ` + "`memories_recall`" + ` (limit 20) — retrieve what you already know
3. ` + "`workspace_introspect`" + ` — see what exists

## Memory protocol

**Recall before you store.** If a trace already exists, link or supersede it
— duplicates dilute the graph and confuse future recall.

**Calibrate confidence honestly:** 0.9-1.0 verified · 0.6-0.8 strong inference
· below 0.6 guess. Inflated confidence is the main way you mislead your
future self.

**Memories are global across your parallel instances.** There is no draft:
what you write, every parallel self sees, and a deprecation affects all of
them immediately.

**Before delivering a final answer, before moving a task to in_review, and
before completing a routine:** reflect on what you learned and keep your
memories current.

## Composite tools — inspect before you call

Many tools group several actions under one ` + "`action`" + ` field. The tool's own
description is only the group's overview; each action has its own
description, examples and input schema.

On the **first** call to each action in a session, pass ` + "`schema: true`" + ` at the
same level as ` + "`action`" + ` to get the full specification. After that you know
the contract and can call directly.

If a call fails validation, do not retry blindly — read the error, inspect
the contract with ` + "`schema: true`" + `, fix the payload.

## Routing

| I need... | Read |
|---|---|
| Memory, learning, a decision | ` + "`references/memories.md`" + ` |
| Lifecycle-managed work | ` + "`references/tasks.md`" + ` |
| Delegate to a specialist | ` + "`references/agents.md`" + ` |
| Structured data | ` + "`references/collections.md`" + ` |
| An interface over data | ` + "`references/views.md`" + ` |
| Scheduled automation | ` + "`references/routines.md`" + ` |
| An external tool | ` + "`references/toolsets.md`" + ` |
| A publishable deliverable | ` + "`references/artifacts.md`" + ` |
| Workspace policy | ` + "`references/instructions.md`" + ` |

## Hard rules

- Every tool call requires a non-empty ` + "`_reasoning`" + `
- Use ` + "`set-status`" + ` to move a task; never ` + "`update`" + `
- In task mode, communicate through the task's own comments, not chat
- Only move to in_review with validation evidence — the system refuses the
  transition with everything still pending
- An instruction is workspace policy; a memory is yours. Personal correction
  → memory. Broad-scope correction → instruction, and it goes through human
  approval
- A command outside your agent's allowlist fails; the error says exactly
  what to ask the workspace owner for
- An action that asks for approval genuinely asks a human when a channel is
  available. In headless mode the denial is immediate and explicit
`
