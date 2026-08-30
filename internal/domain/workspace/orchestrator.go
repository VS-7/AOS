package workspace

import (
	"fmt"
	"strings"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/slug"
)

// DefaultOrchestratorName is the name the first agent gets when none is asked
// for. It is the original's, kept because a user who has seen one product
// recognises the other.
const DefaultOrchestratorName = "Atlas"

// DefaultOrchestratorRole is what the orchestrator answers when asked what it
// does.
const DefaultOrchestratorRole = build.DisplayName + " Architect & Workspace Orchestrator"

// DefaultOrchestratorDescription is the routing criterion the orchestrator
// publishes to whoever is deciding whom to delegate to. In this schema
// "description" is not documentation: it is how the choice gets made.
const DefaultOrchestratorDescription = "Default workspace orchestrator responsible for triage, delegation, lifecycle guidance and capability routing."

// DefaultOrchestratorSandbox is what the first agent of a workspace may do.
//
// An agent with no sandbox block gets the zero value, which is read-only with
// no execution at all — deliberately stricter than the original, and the right
// default for an agent somebody adds later whose job nobody has declared yet.
//
// It is the wrong default for this one. The orchestrator is created by the
// system, on the person's own machine, in the repository they just pointed it
// at, in answer to them asking for a workspace. Shipping it read-only made the
// product's first experience an assistant that reads, plans, delegates, and
// then reports that the sandbox refused it — which is what happened.
//
// Execution stays an allowlist (ADR-0006) and the shell stays off: a shell
// makes an allowlist a suggestion, so it keeps its own opt-in. What is on the
// list is the set somebody watching an agent work in a repository expects it
// to reach without being asked twice.
func DefaultOrchestratorSandbox() *SandboxSeed {
	return &SandboxSeed{
		Permissions: []string{"read", "write", "delete", "execute"},
		Exec: &ExecSeed{
			Policy: "allowlist",
			Allow: []string{
				"git", "go", "node", "npm", "npx", "pnpm", "yarn", "bun",
				"python3", "pip3", "make", "task", "cargo", "rustc",
				"ls", "cat", "grep", "find", "rg", "sed", "awk", "head", "tail", "wc", "diff",
			},
			AllowShell: false,
		},
	}
}

// buildOrchestrator turns the dials of the create input into the agent record
// that will be written.
func buildOrchestrator(w *Workspace, spec *OrchestratorSpec) OrchestratorSeed {
	name := DefaultOrchestratorName
	if spec != nil && strings.TrimSpace(spec.Name) != "" {
		name = strings.TrimSpace(spec.Name)
	}
	return OrchestratorSeed{
		Root:         w.Path,
		ID:           slug.Generate(name),
		Name:         name,
		Role:         DefaultOrchestratorRole,
		Description:  DefaultOrchestratorDescription,
		Instructions: orchestratorInstructions(w, name, spec),
		Sandbox:      orchestratorSandbox(spec),
	}
}

// orchestratorSandbox honours what the caller asked for, and falls back to the
// working default. A caller that names a narrower sandbox gets exactly it.
func orchestratorSandbox(spec *OrchestratorSpec) *SandboxSeed {
	if spec == nil || spec.Sandbox == nil {
		return DefaultOrchestratorSandbox()
	}
	seed := &SandboxSeed{Permissions: spec.Sandbox.Permissions}
	if spec.Sandbox.Exec != nil {
		seed.Exec = &ExecSeed{
			Policy:     spec.Sandbox.Exec.Policy,
			Allow:      spec.Sandbox.Exec.Allow,
			AllowShell: spec.Sandbox.Exec.AllowShell,
		}
	}
	return seed
}

// orchestratorInstructions renders the Markdown body of the orchestrator: the
// system prompt it carries into every turn.
//
// The tone, style and autonomy dials are rendered as explicit prose rather than
// as structured fields, because the consumer is a language model reading a
// document, not a program reading a record. That is the original's choice and
// it is the right one.
func orchestratorInstructions(w *Workspace, name string, spec *OrchestratorSpec) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Identity\nYou are %s, the orchestrator of the workspace %q.\n\n", name, w.Name)

	b.WriteString(`## Mission
- Be the person's primary counterpart inside this workspace.
- Triage what is asked, decide the next safe step, and delegate specialist work.
- Prefer the native capabilities for memories, tasks, instructions, skills, templates, toolsets, chats, collections and agents.

`)

	fmt.Fprintf(&b, "## Workspace Context\n- Workspace id: %s\n- Workspace path: %s\n"+
		"- Treat this workspace as the source of truth for its agents, memories, instructions, templates and collections.\n",
		w.ID, w.Path)

	if directives := behaviourDirectives(spec); directives != "" {
		b.WriteString("\n## Communication & Behaviour\n")
		b.WriteString(directives)
	}

	b.WriteString(`
## Operating Rules
- Start by understanding the goal, the current state of the code, and any existing records that bear on it.
- Keep answers concise, practical and oriented to the next action.
- Use tasks, todos, comments and memories to preserve continuity whenever the work benefits from being tracked.
- When a request is ambiguous or the change is hard to undo, clarify before acting.
- Create focused specialists for bounded work, and keep the orchestration decisions here.

## Capabilities
- Memories hold durable context and lessons. Recall before you store.
- Tasks and todos structure multi-step execution.
- Instructions and skills are the first source of behavioural and domain guidance.
- Templates and toolsets exist so that boilerplate and integrations are not improvised.
- Agents are how work is delegated — delegation does not transfer accountability for the outcome.
`)

	return b.String()
}

// behaviourDirectives renders the three dials. Each value maps to one sentence
// that says something a model can act on; a bare label like "tone: candid"
// would not change a single response.
func behaviourDirectives(spec *OrchestratorSpec) string {
	if spec == nil {
		return ""
	}
	var lines []string

	switch spec.Tone {
	case "efficient":
		lines = append(lines, "- **Tone**: efficient. Be direct, optimise for speed of execution, and skip conversational filler.")
	case "friendly":
		lines = append(lines, "- **Tone**: friendly. Be warm, encouraging and supportive in how you phrase things.")
	case "professional":
		lines = append(lines, "- **Tone**: professional. Be polite, structured and objective.")
	case "candid":
		lines = append(lines, "- **Tone**: candid. Say what you actually think, name problems directly, and do not soften a real risk.")
	}

	switch spec.Style {
	case "concise":
		lines = append(lines, "- **Style**: concise. Keep explanations short, summarise, and list steps without padding.")
	case "balanced":
		lines = append(lines, "- **Style**: balanced. Explain the parts that are genuinely subtle and keep the routine parts brief.")
	case "detailed":
		lines = append(lines, "- **Style**: detailed. Give the full breakdown, walk through the flow, and state the reasoning behind each decision.")
	}

	if spec.Autonomy > 0 {
		pct := int(spec.Autonomy*100 + 0.5)
		switch {
		case spec.Autonomy < 0.3:
			lines = append(lines, fmt.Sprintf("- **Autonomy**: %d%%. Ask before running tools or changing files.", pct))
		case spec.Autonomy <= 0.7:
			lines = append(lines, fmt.Sprintf("- **Autonomy**: %d%%. Read files and run harmless commands on your own; ask before structural changes.", pct))
		default:
			lines = append(lines, fmt.Sprintf("- **Autonomy**: %d%%. Act independently, and ask only before operations that are destructive or hard to undo.", pct))
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
