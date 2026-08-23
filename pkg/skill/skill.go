// Package skill generates SKILL.md and per-group reference files from a
// command registry — see docs/09 - Skill/Especificação da Skill.md.
//
// The one thing this package cannot depend on is this project: an agent
// harness reading a generated skill has never heard of AOS's internal
// packages, and a generator coupled to them would be untestable outside a
// build of the whole daemon. Registry below is the minimal shape Generate
// needs; internal/app is where the real command.Registry gets adapted to
// it (tools/genskill/main.go), not here.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Registry is everything Generate reads.
type Registry interface {
	Groups() []Group
}

// Group is one command group — "memories", "tasks" — with the Markdown that
// becomes its reference file's body.
type Group struct {
	Name    string
	Summary string
	// Doc is the group's own documentation, already in the five-section
	// shape docs/09 - Skill/Especificação da Skill.md requires ("## What It
	// Does", "## Commands", "## When to Use This Group", "## Key Concepts",
	// "## Rules") — Generate does not impose that structure, it publishes
	// whatever Doc holds; MissingSections is the separate, non-blocking
	// check for whether it actually has all five (see that function's own
	// doc comment for why it does not fail Generate itself).
	Doc      string
	Commands []Command
}

// Command is one action inside a Group, with the worked examples that go
// straight into the reference file and into the MCP tool description alike
// — one source, not two hand-kept lists.
type Command struct {
	Key      string
	Summary  string
	Doc      string
	Examples []Example
}

// Example is one worked call.
type Example struct {
	Description string
	Input       any
}

// Options carries the one thing Generate cannot derive from the registry:
// SKILL.md's own body. docs/09 - Skill/SKILL (gerada).md's own "Decisões"
// section is explicit about why — SKILL.md is curated (session protocol,
// routing, hard rules), references/ is mechanical. Mixing the two would
// mean curation gets silently overwritten on every regeneration.
type Options struct {
	// SkillMD is SKILL.md's full contents, frontmatter included.
	SkillMD string
}

// Generate writes SKILL.md and references/<group>.md under dir, replacing
// whatever was there — this is the only writer of these files (see the
// package doc comment); a hand edit under dir is overwritten on the next
// generation, which is the property task gen-skill's CI gate
// (git diff --exit-code) depends on: the committed output either matches
// what Generate produces from the current registry, or the build fails.
func Generate(reg Registry, dir string, opts Options) error {
	if strings.TrimSpace(opts.SkillMD) == "" {
		return fmt.Errorf("skill: Options.SkillMD is empty — SKILL.md is curated, not derived, and Generate refuses to publish a blank one")
	}

	refsDir := filepath.Join(dir, "references")
	if err := os.RemoveAll(refsDir); err != nil {
		return fmt.Errorf("skill: clearing %s: %w", refsDir, err)
	}
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		return fmt.Errorf("skill: creating %s: %w", refsDir, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(opts.SkillMD), 0o644); err != nil { //nolint:gosec // generated source, not a secret
		return fmt.Errorf("skill: writing SKILL.md: %w", err)
	}

	groups := reg.Groups()
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	for _, g := range groups {
		path := filepath.Join(refsDir, g.Name+".md")
		if err := os.WriteFile(path, []byte(renderReference(g)), 0o644); err != nil { //nolint:gosec // generated source, not a secret
			return fmt.Errorf("skill: writing %s: %w", path, err)
		}
	}
	return nil
}

// renderReference is the mechanical part: Group.Doc as the body, one
// section per command with its own Doc and worked examples. Deterministic —
// commands are already ordered by the registry (alphabetical, see
// command.Registry.Groups), and nothing here reads a clock or a random
// source — which is what makes "two runs produce identical bytes" true.
func renderReference(g Group) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", title(g.Name))
	if g.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", g.Summary)
	}
	if doc := strings.TrimSpace(g.Doc); doc != "" {
		fmt.Fprintf(&b, "%s\n\n", doc)
	}

	if len(g.Commands) == 0 {
		return b.String()
	}

	b.WriteString("## Commands\n\n")
	for _, c := range g.Commands {
		fmt.Fprintf(&b, "### `%s`\n\n", c.Key)
		if c.Summary != "" {
			fmt.Fprintf(&b, "%s\n\n", c.Summary)
		}
		if doc := strings.TrimSpace(c.Doc); doc != "" && doc != strings.TrimSpace(c.Summary) {
			fmt.Fprintf(&b, "%s\n\n", doc)
		}
		for _, ex := range c.Examples {
			fmt.Fprintf(&b, "- %s\n", ex.Description)
		}
		if len(c.Examples) > 0 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// title turns a group's registry name ("update") into its display form
// ("Update") — the same casing docs/09 - Skill/Especificação da Skill.md's
// own reference filenames are titled by, one word, no dictionary needed
// since every group name here already is one.
func title(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// RequiredSections are the five headings every group's Doc is meant to have
// — docs/09 - Skill/Especificação da Skill.md's own reasoning: a Doc
// missing "When to Use This Group" is a group an agent will not know when
// to reach for.
var RequiredSections = []string{
	"## What It Does",
	"## Commands",
	"## When to Use This Group",
	"## Key Concepts",
	"## Rules",
}

// MissingSections reports, per group name, which of RequiredSections its
// Doc lacks. A group absent from the result has all five.
//
// This does not fail Generate, and tools/genskill does not fail on it
// either — see that command's own doc comment on why forcing it today would
// mean rewriting every domain's hand-tuned Doc field in one pass rather
// than reviewing each one for what it actually needs to say. It exists so
// the gap is visible on every generation instead of invisible until an
// agent hits it.
func MissingSections(reg Registry) map[string][]string {
	out := map[string][]string{}
	for _, g := range reg.Groups() {
		var missing []string
		for _, section := range RequiredSections {
			if !strings.Contains(g.Doc, section) {
				missing = append(missing, section)
			}
		}
		if len(missing) > 0 {
			out[g.Name] = missing
		}
	}
	return out
}
