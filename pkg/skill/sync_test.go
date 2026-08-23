package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/aos/pkg/skill"
)

func generatedSkill(t *testing.T, skillMD string) string {
	t.Helper()
	dir := t.TempDir()
	if err := skill.Generate(sampleRegistry(), dir, skill.Options{SkillMD: skillMD}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSyncCopiesIntoAnExistingTarget(t *testing.T) {
	src := generatedSkill(t, sampleSkillMD)
	target := t.TempDir() // simulates ~/.claude/skills

	result, err := skill.Sync(src, []string{target}, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Synced) != 1 {
		t.Fatalf("expected one synced target, got %+v", result)
	}

	got, err := os.ReadFile(filepath.Join(target, "aos", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != sampleSkillMD {
		t.Fatalf("synced SKILL.md does not match the source:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(target, "aos", "references", "memories.md")); err != nil {
		t.Errorf("expected references/memories.md to be synced too: %v", err)
	}
}

func TestSyncSkipsATargetDirectoryThatDoesNotExist(t *testing.T) {
	src := generatedSkill(t, sampleSkillMD)
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	result, err := skill.Sync(src, []string{missing}, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Synced) != 0 {
		t.Fatalf("expected nothing synced, got %+v", result.Synced)
	}
	if _, ok := result.Skipped[missing]; !ok {
		t.Fatalf("expected %s to be reported skipped, got %+v", missing, result.Skipped)
	}
}

// The property this whole function exists to protect: a directory that
// already holds somebody else's skill under the same name is never
// clobbered.
func TestSyncSkipsAThirdPartySkillWithTheSameName(t *testing.T) {
	src := generatedSkill(t, sampleSkillMD)
	target := t.TempDir()

	thirdParty := filepath.Join(target, "aos")
	if err := os.MkdirAll(thirdParty, 0o755); err != nil {
		t.Fatal(err)
	}
	thirdPartyContent := "---\nname: someone-elses-aos\n---\n\nNot this project's skill.\n"
	if err := os.WriteFile(filepath.Join(thirdParty, "SKILL.md"), []byte(thirdPartyContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := skill.Sync(src, []string{target}, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Synced) != 0 {
		t.Fatalf("expected nothing synced, got %+v", result.Synced)
	}
	got, err := os.ReadFile(filepath.Join(thirdParty, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != thirdPartyContent {
		t.Fatal("the third-party skill's own SKILL.md was overwritten")
	}
}

// A previous sync of this same skill (same name, real content) is a stale
// copy, not a third party — Sync must overwrite it, or "regenerate and
// resync" would never actually update anything already synced once.
func TestSyncOverwritesAStaleCopyOfTheSameSkill(t *testing.T) {
	oldSkillMD := "---\nname: aos\ndescription: old\n---\n\nOld body.\n"
	newSkillMD := "---\nname: aos\ndescription: new\n---\n\nNew body.\n"
	target := t.TempDir()

	if _, err := skill.Sync(generatedSkill(t, oldSkillMD), []string{target}, "aos"); err != nil {
		t.Fatal(err)
	}
	result, err := skill.Sync(generatedSkill(t, newSkillMD), []string{target}, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Synced) != 1 {
		t.Fatalf("expected the resync to succeed, got %+v", result)
	}
	got, err := os.ReadFile(filepath.Join(target, "aos", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newSkillMD {
		t.Fatal("resync did not update the stale copy")
	}
}

func TestSyncWritesToEveryTargetIndependently(t *testing.T) {
	src := generatedSkill(t, sampleSkillMD)
	targetA, targetB := t.TempDir(), t.TempDir()
	missing := filepath.Join(t.TempDir(), "nope")

	result, err := skill.Sync(src, []string{targetA, missing, targetB}, "aos")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Synced) != 2 {
		t.Fatalf("expected both real targets synced, got %+v", result.Synced)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected exactly the missing target skipped, got %+v", result.Skipped)
	}
}
