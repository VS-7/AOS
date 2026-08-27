package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/pkg/skill"
)

// The embedded skill is the committed one, byte for byte: what `aos self
// skill install` puts on a machine is exactly what pkg/skill/ holds in the
// repository, not a second copy that could lag behind a regeneration.
func TestEmbeddedFilesMatchTheCommittedSkill(t *testing.T) {
	embedded, err := skill.Files.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(embedded) != string(committed) {
		t.Fatal("the embedded SKILL.md differs from the committed one")
	}
	if got := frontmatterNameOf(string(embedded)); got != skill.Name {
		t.Fatalf("SKILL.md frontmatter name = %q, want %q", got, skill.Name)
	}

	refs, err := skill.Files.ReadDir("references")
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadDir("references")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) == 0 || len(refs) != len(onDisk) {
		t.Fatalf("embedded %d references, committed %d", len(refs), len(onDisk))
	}
}

func frontmatterNameOf(md string) string {
	for _, line := range strings.Split(md, "\n")[1:] {
		if strings.TrimSpace(line) == "---" {
			return ""
		}
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "name:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func TestInstallCreatesTheTargetAndCopiesEveryFile(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "skills") // does not exist yet

	result, err := skill.Install(skill.Files, []string{dir}, skill.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 1 || result.Installed[0] != filepath.Join(dir, skill.Name) {
		t.Fatalf("installed = %v", result.Installed)
	}
	if _, err := os.Stat(filepath.Join(dir, skill.Name, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, skill.Name, "references", "memories.md")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallReplacesAStaleCopyOfItself(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, skill.Name)
	if err := os.MkdirAll(filepath.Join(stale, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "SKILL.md"), []byte("---\nname: aos\n---\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "references", "gone.md"), []byte("removed group"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := skill.Install(skill.Files, []string{dir}, skill.Name); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(stale, "SKILL.md"))
	if strings.TrimSpace(string(got)) == "old" {
		t.Fatal("the stale SKILL.md survived")
	}
	if _, err := os.Stat(filepath.Join(stale, "references", "gone.md")); !os.IsNotExist(err) {
		t.Fatal("a reference the skill no longer has survived the reinstall")
	}
}

func TestInstallRefusesToClobberSomebodyElsesSkill(t *testing.T) {
	dir := t.TempDir()
	theirs := filepath.Join(dir, skill.Name)
	if err := os.MkdirAll(theirs, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: not-this-one\n---\n"
	if err := os.WriteFile(filepath.Join(theirs, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := skill.Install(skill.Files, []string{dir}, skill.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 0 {
		t.Fatalf("installed = %v, want nothing", result.Installed)
	}
	if _, ok := result.Skipped[theirs]; !ok {
		t.Fatalf("skipped = %v, want %s", result.Skipped, theirs)
	}
	got, _ := os.ReadFile(filepath.Join(theirs, "SKILL.md"))
	if string(got) != content {
		t.Fatal("the third-party skill was overwritten")
	}
}

func TestTargetsReportPresenceAndInstallation(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := skill.Install(skill.Files, []string{filepath.Join(home, ".codex", "skills")}, skill.Name); err != nil {
		t.Fatal(err)
	}

	byID := map[string]skill.Target{}
	for _, target := range skill.Targets(home, skill.Name) {
		byID[target.ID] = target
	}
	if !byID["claude-code"].Present || byID["claude-code"].Installed {
		t.Fatalf("claude-code = %+v, want present and not installed", byID["claude-code"])
	}
	if !byID["codex"].Present || !byID["codex"].Installed {
		t.Fatalf("codex = %+v, want present and installed", byID["codex"])
	}
	if byID["cursor"].Present {
		t.Fatalf("cursor = %+v, want absent", byID["cursor"])
	}
	if _, ok := skill.LookupTarget(home, skill.Name, "no-such-agent"); ok {
		t.Fatal("an unknown id resolved to a target")
	}
	for _, id := range skill.TargetIDs() {
		if _, ok := byID[id]; !ok {
			t.Fatalf("TargetIDs lists %q but Targets does not", id)
		}
	}
}
