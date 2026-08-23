package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/pkg/skill"
)

type fakeRegistry struct{ groups []skill.Group }

func (f fakeRegistry) Groups() []skill.Group { return f.groups }

func sampleRegistry() fakeRegistry {
	return fakeRegistry{groups: []skill.Group{
		{
			Name: "memories", Summary: "Persistent memory across sessions.",
			Doc: "## What It Does\nRemembers things.\n\n## Commands\n- **recall**\n\n" +
				"## When to Use This Group\n- Start of session\n\n## Key Concepts\n- **Trace:** one memory\n\n" +
				"## Rules\n- Recall before you store",
			Commands: []skill.Command{
				{
					Key: "memories_recall", Summary: "Retrieve memories.",
					Doc:      "Reads memories matching a query.",
					Examples: []skill.Example{{Description: "recall recent memories", Input: map[string]any{"limit": 20}}},
				},
				{Key: "memories_store", Summary: "Store a memory.", Doc: "Writes a new memory."},
			},
		},
		{Name: "tasks", Summary: "Work with a lifecycle.", Commands: []skill.Command{
			{Key: "tasks_list", Summary: "List tasks."},
		}},
	}}
}

const sampleSkillMD = "---\nname: aos\ndescription: test\n---\n\n# AOS\n\nTest body.\n"

func TestGenerateWritesSkillMDAndReferences(t *testing.T) {
	dir := t.TempDir()
	if err := skill.Generate(sampleRegistry(), dir, skill.Options{SkillMD: sampleSkillMD}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != sampleSkillMD {
		t.Fatalf("SKILL.md was not published verbatim:\n%s", got)
	}

	for _, name := range []string{"memories", "tasks"} {
		path := filepath.Join(dir, "references", name+".md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}

	memories, err := os.ReadFile(filepath.Join(dir, "references", "memories.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(memories)
	for _, want := range []string{"# Memories", "Persistent memory across sessions.", "## What It Does", "### `memories_recall`", "Retrieve memories.", "recall recent memories", "### `memories_store`"} {
		if !strings.Contains(body, want) {
			t.Errorf("memories.md missing %q:\n%s", want, body)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	reg := sampleRegistry()
	if err := skill.Generate(reg, dirA, skill.Options{SkillMD: sampleSkillMD}); err != nil {
		t.Fatal(err)
	}
	if err := skill.Generate(reg, dirB, skill.Options{SkillMD: sampleSkillMD}); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"SKILL.md", filepath.Join("references", "memories.md"), filepath.Join("references", "tasks.md")} {
		a, err := os.ReadFile(filepath.Join(dirA, rel))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dirB, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs between two runs", rel)
		}
	}
}

func TestGenerateClearsStaleReferenceFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "references", "a-domain-that-no-longer-exists.md")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := skill.Generate(sampleRegistry(), dir, skill.Options{SkillMD: sampleSkillMD}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected the stale reference file to be gone, stat err = %v", err)
	}
}

func TestGenerateRefusesAnEmptySkillMD(t *testing.T) {
	if err := skill.Generate(sampleRegistry(), t.TempDir(), skill.Options{}); err == nil {
		t.Fatal("expected an error for an empty SkillMD")
	}
}

func TestMissingSectionsReportsAnIncompleteDoc(t *testing.T) {
	reg := fakeRegistry{groups: []skill.Group{
		{Name: "complete", Doc: "## What It Does\nx\n## Commands\nx\n## When to Use This Group\nx\n## Key Concepts\nx\n## Rules\nx"},
		{Name: "bare", Doc: "Just a paragraph, no sections at all."},
	}}

	missing := skill.MissingSections(reg)
	if _, ok := missing["complete"]; ok {
		t.Error("a Doc with all five sections should not be reported")
	}
	got := missing["bare"]
	if len(got) != len(skill.RequiredSections) {
		t.Fatalf("expected all %d sections missing for a bare Doc, got %v", len(skill.RequiredSections), got)
	}
}
