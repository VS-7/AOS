package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/app"
)

// TestSkillMDFrontmatterIsValid — docs/09 - Skill/SKILL (gerada).md's own
// "Testes" list: a skill with no name or description is a skill a host
// agent has no way to find or load.
func TestSkillMDFrontmatterIsValid(t *testing.T) {
	if !strings.HasPrefix(skillMD, "---\n") {
		t.Fatal("SKILL.md must start with a YAML frontmatter block")
	}
	end := strings.Index(skillMD[4:], "\n---")
	if end < 0 {
		t.Fatal("SKILL.md frontmatter is never closed")
	}
	frontmatter := skillMD[:end+4]

	if !regexp.MustCompile(`(?m)^name:\s*\S+`).MatchString(frontmatter) {
		t.Error("frontmatter is missing a non-empty `name`")
	}
	if !regexp.MustCompile(`(?m)^description:\s*\S+`).MatchString(frontmatter) {
		t.Error("frontmatter is missing a non-empty `description`")
	}
}

// TestSkillMDRoutingTargetsExist — every references/*.md the routing table
// points at is a group Generate will actually produce. A stale route (a
// domain renamed or removed) sends an agent to a file that is not there.
func TestSkillMDRoutingTargetsExist(t *testing.T) {
	built, err := app.New(app.Options{WorkspaceRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = built.Close() }()

	known := map[string]bool{}
	for _, g := range built.Registry.Groups() {
		known[g.Name+".md"] = true
	}

	for _, ref := range regexp.MustCompile("`references/([a-z-]+\\.md)`").FindAllStringSubmatch(skillMD, -1) {
		if !known[ref[1]] {
			t.Errorf("SKILL.md routes to references/%s, which no registered group produces", ref[1])
		}
	}
}

// TestSkillMDSessionCommandsExist — docs/09 - Skill/SKILL (gerada).md's own
// "Testes" list: every command cited in the session-start protocol must be
// a real, registered command key.
func TestSkillMDSessionCommandsExist(t *testing.T) {
	built, err := app.New(app.Options{WorkspaceRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = built.Close() }()

	known := map[string]bool{}
	for _, d := range built.Registry.Sorted() {
		known[d.Key()] = true
	}

	sessionStart := skillMD[strings.Index(skillMD, "## Session start"):strings.Index(skillMD, "## Memory protocol")]
	for _, m := range regexp.MustCompile("`([a-z]+_[a-z-]+)`").FindAllStringSubmatch(sessionStart, -1) {
		if !known[m[1]] {
			t.Errorf("session-start protocol cites %q, which is not a registered command", m[1])
		}
	}
}

// TestGeneratedSkillIsCommitted is the CI gate docs/09 - Skill/Especificação
// da Skill.md asks for — "task gen-skill && git diff --exit-code
// pkg/skill/" — as a Go test rather than a shell script, so it also runs
// under `go test ./...` and `task test`, not only `task check`.
func TestGeneratedSkillIsCommitted(t *testing.T) {
	built, err := app.New(app.Options{WorkspaceRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = built.Close() }()

	fresh := t.TempDir()
	if err := generate(built.Registry, fresh); err != nil {
		t.Fatal(err)
	}

	committed := moduleRoot(t) + "/pkg/skill"
	assertTreesEqual(t, committed, fresh)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir || parent == "" {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// assertTreesEqual compares only the generated surface — SKILL.md and
// references/*.md — never the whole want directory: pkg/skill also holds
// this generator's own .go source, which a fresh generation into an empty
// temp dir never produces and was never meant to.
func assertTreesEqual(t *testing.T, want, got string) {
	t.Helper()
	wantFiles := generatedFiles(t, want)
	gotFiles := generatedFiles(t, got)

	if len(wantFiles) != len(gotFiles) {
		t.Fatalf("pkg/skill is stale: committed has %d generated files, a fresh generation has %d — run `task gen-skill`",
			len(wantFiles), len(gotFiles))
	}
	for _, rel := range wantFiles {
		wantData, err := os.ReadFile(want + "/" + rel)
		if err != nil {
			t.Fatal(err)
		}
		gotData, err := os.ReadFile(got + "/" + rel)
		if err != nil {
			t.Fatalf("pkg/skill is stale: %s no longer exists in a fresh generation — run `task gen-skill`", rel)
		}
		if string(wantData) != string(gotData) {
			t.Errorf("pkg/skill is stale: %s differs from a fresh generation — run `task gen-skill`", rel)
		}
	}
}

// generatedFiles lists SKILL.md and every references/*.md under dir — the
// exact surface skill.Generate owns (see its own doc comment) — sorted, so
// two listings compare in the same order.
func generatedFiles(t *testing.T, dir string) []string {
	t.Helper()
	out := []string{"SKILL.md"}
	entries, err := os.ReadDir(dir + "/references")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, "references/"+e.Name())
		}
	}
	sort.Strings(out)
	return out
}
