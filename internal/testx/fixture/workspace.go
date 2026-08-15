// Package fixture materialises deterministic workspaces on disk.
//
// Every timestamp comes from a fixed clock and every id from a sequential
// generator, so anything built from a fixture is byte-stable and a golden file
// derived from one does not drift between runs or between machines.
package fixture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// RefTime is the fixed clock of every fixture.
var RefTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// Size selects how much a fixture contains.
type Size int

const (
	// Minimal is one agent, three memories and one task: enough to exercise a
	// code path, small enough to run on every save.
	Minimal Size = iota
	// Typical is a workspace that looks like real use.
	Typical
	// Large exists to measure: ten thousand memories is where Refresh and the
	// index have to prove they scale. It is generated, never committed.
	Large
)

// Counts describes the content of each size.
type Counts struct {
	Agents       int
	MemoriesEach int
	Tasks        int
	TodosEach    int
	Skills       int
}

func countsFor(size Size) Counts {
	switch size {
	case Typical:
		return Counts{Agents: 3, MemoriesEach: 14, Tasks: 12, TodosEach: 3, Skills: 2}
	case Large:
		return Counts{Agents: 10, MemoriesEach: 1000, Tasks: 500, TodosEach: 2, Skills: 5}
	default:
		return Counts{Agents: 1, MemoriesEach: 3, Tasks: 1, TodosEach: 2, Skills: 0}
	}
}

// Fixture is a materialised workspace.
type Fixture struct {
	Root   string
	Counts Counts
	Agents []string
}

// StateDir returns the workspace state directory.
func (f *Fixture) StateDir() string { return filepath.Join(f.Root, collections.Root) }

// Workspace materialises a workspace of the given size in a temporary
// directory and returns it.
func Workspace(t *testing.T, size Size) *Fixture {
	t.Helper()
	return WorkspaceAt(t, t.TempDir(), size)
}

// WorkspaceAt materialises a workspace at an explicit root, for a test that
// needs the path to survive the helper.
func WorkspaceAt(t *testing.T, root string, size Size) *Fixture {
	t.Helper()
	counts := countsFor(size)
	f := &Fixture{Root: root, Counts: counts}

	for a := 0; a < counts.Agents; a++ {
		agent := fmt.Sprintf("agent-%02d", a)
		f.Agents = append(f.Agents, agent)
		write(t, root, fmt.Sprintf(".aos/agents/%s/AGENT.md", agent), agentDoc(agent, a))
		for m := 0; m < counts.MemoriesEach; m++ {
			write(t, root,
				fmt.Sprintf(".aos/agents/%s/memories/m-%05d.memory.md", agent, m),
				memoryDoc(a, m))
		}
	}
	for k := 0; k < counts.Tasks; k++ {
		write(t, root, fmt.Sprintf(".aos/tasks/task-%04d/TASK.md", k), taskDoc(k))
		for td := 0; td < counts.TodosEach; td++ {
			write(t, root,
				fmt.Sprintf(".aos/tasks/task-%04d/todos/todo-%02d.json", k, td),
				todoDoc(k, td))
		}
	}
	for s := 0; s < counts.Skills; s++ {
		skill := fmt.Sprintf("skill-%02d", s)
		write(t, root, fmt.Sprintf(".aos/skills/%s/SKILL.md", skill), skillDoc(skill, s))
		// A skill ships its own agent and one memory for it, which is the
		// second pattern of the agents and memories collections.
		write(t, root, fmt.Sprintf(".aos/skills/%s/agents/packed-%02d/AGENT.md", skill, s), agentDoc("packed", s))
		write(t, root,
			fmt.Sprintf(".aos/skills/%s/agents/packed-%02d/memories/packed.memory.md", skill, s),
			memoryDoc(s, 0))
	}
	return f
}

// Total returns how many records the fixture wrote, for a scale assertion.
func (f *Fixture) Total() int {
	c := f.Counts
	return c.Agents + c.Agents*c.MemoriesEach + c.Tasks + c.Tasks*c.TodosEach + c.Skills*3
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stamp returns a deterministic timestamp derived from an index, so two runs of
// the same fixture produce identical files.
func stamp(i int) string {
	return RefTime.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
}

func agentDoc(id string, i int) string {
	return strings.Join([]string{
		"---",
		"name: " + id,
		"role: " + []string{"orchestrator", "member"}[i%2],
		"createdAt: " + stamp(i),
		"updatedAt: " + stamp(i),
		"---",
		"",
		"# " + id,
		"",
		"Deterministic fixture agent.",
		"",
	}, "\n")
}

func memoryDoc(agent, i int) string {
	categories := []string{"decision", "intent", "commitment", "observation", "learning", "fact", "preference"}
	return strings.Join([]string{
		"---",
		fmt.Sprintf("title: memory %05d of agent %02d", i, agent),
		"category: " + categories[i%len(categories)],
		fmt.Sprintf("confidence: %.1f", float64(i%10)/10),
		"status: active",
		"createdAt: " + stamp(i),
		"updatedAt: " + stamp(i),
		"---",
		"",
		fmt.Sprintf("Body of memory %05d. Deterministic content for a stable fixture.", i),
		"",
	}, "\n")
}

func taskDoc(i int) string {
	states := []string{"backlog", "todo", "in_progress", "in_review", "done"}
	return strings.Join([]string{
		"---",
		fmt.Sprintf("title: task %04d", i),
		"status: " + states[i%len(states)],
		"createdAt: " + stamp(i),
		"updatedAt: " + stamp(i),
		"---",
		"",
		fmt.Sprintf("Description of task %04d.", i),
		"",
	}, "\n")
}

func todoDoc(task, i int) string {
	return fmt.Sprintf("{\n  \"title\": \"todo %02d of task %04d\",\n  \"status\": \"%s\",\n  \"createdAt\": %q\n}\n",
		i, task, []string{"pending", "done"}[i%2], stamp(i))
}

func skillDoc(id string, i int) string {
	return strings.Join([]string{
		"---",
		"name: " + id,
		"version: 1.0.0",
		"createdAt: " + stamp(i),
		"---",
		"",
		"# " + id,
		"",
		"Deterministic fixture skill.",
		"",
	}, "\n")
}
