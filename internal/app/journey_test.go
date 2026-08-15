package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/workspace"

	"github.com/OWNER/aos/internal/app"
)

// TestTheDeliveryOfThePhase is the claim the phase is judged on, run end to end
// against a real filesystem rather than against fakes: register a workspace,
// find the orchestrator that was born with it, store memories, recall them, and
// map the graph.
//
// Every assertion below is about a file that exists or a record that comes
// back, because the phase promises a system that works and not a set of
// packages that compile.
func TestTheDeliveryOfThePhase(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()

	// Step one: register the repository. Nothing is configured beforehand —
	// this is what a person runs first.
	first := newAppAt(t, home, repo, "")
	created, err := first.Workspaces.Introspect(parityCtx(), workspace.IntrospectInput{Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	id := created.Workspace.ID
	if id == "" {
		t.Fatal("the workspace was registered without an identifier")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("the layout is inside the repository", func(t *testing.T) {
		root := filepath.Join(repo, collections.Root)
		for _, dir := range workspace.ManagedDirs {
			if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
				t.Errorf("%s/%s is missing", collections.Root, dir)
			}
		}
		env, err := os.ReadFile(filepath.Join(repo, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(env), "AOS_WORKSPACE_ID="+id) {
			t.Errorf(".env does not carry the workspace id:\n%s", env)
		}
	})

	t.Run("the record is outside it", func(t *testing.T) {
		// The metadata lives under the state directory; the data lives in the
		// repository. That split is what lets agent state be committed next to
		// the code without the registry following it around.
		if _, err := os.Stat(filepath.Join(home, "workspaces", id, "config.json")); err != nil {
			t.Errorf("no workspace record under the state directory: %v", err)
		}
		if _, err := os.Stat(filepath.Join(repo, "workspaces")); !os.IsNotExist(err) {
			t.Error("the registry leaked into the user's repository")
		}
	})

	t.Run("the orchestrator was born with the workspace", func(t *testing.T) {
		agentFile := filepath.Join(repo, collections.Root, "agents", created.Orchestrator, "AGENT.md")
		raw, err := os.ReadFile(agentFile)
		if err != nil {
			t.Fatalf("the orchestrator has no file on disk: %v", err)
		}
		body := string(raw)
		if !strings.Contains(body, "orchestrator: true") {
			t.Errorf("the front matter does not mark it as the orchestrator:\n%s", body)
		}
		if !strings.Contains(body, created.Workspace.Name) {
			t.Error("the instructions do not name the workspace they belong to")
		}
		// It is a Markdown file a person can read and Git can diff. That is the
		// property the whole storage decision exists for.
		if !strings.HasPrefix(body, "---\n") {
			t.Error("the record is not front matter followed by a body")
		}
	})

	// Step two: a second process, now with the workspace active — which is what
	// the managed block in .env arranges for every later command.
	second := newAppAt(t, home, repo, id)
	defer func() {
		if err := second.Close(); err != nil {
			t.Error(err)
		}
	}()

	t.Run("the orchestrator answers as itself", func(t *testing.T) {
		me, err := second.Agents.Me(parityCtx(), agent.MeInput{})
		if err != nil {
			t.Fatal(err)
		}
		if me.ID != "atlas" {
			t.Fatalf("me = %q", me.ID)
		}
	})

	var stored []string
	t.Run("memories are written as files", func(t *testing.T) {
		for _, in := range []memory.StoreInput{
			{
				Title: "The registry lives outside the repository",
				Description: "Workspace metadata is under the state directory and workspace data is under .aos/ " +
					"inside the repository, so agent state is committed with the code.",
				Category: memory.CatFact,
				Tags:     []string{"workspace", "storage"},
				Scopes:   []string{"internal/domain/workspace/**"},
			},
			{
				Title:       "The orchestrator is created with the workspace",
				Description: "Registering a repository creates the agent that answers when nobody else is addressed.",
				Category:    memory.CatDecision,
				Tags:        []string{"workspace", "agents"},
			},
		} {
			out, err := second.Memories.Store(parityCtx(), in)
			if err != nil {
				t.Fatal(err)
			}
			stored = append(stored, out.Memory.ID)
		}

		for _, memoryID := range stored {
			path := filepath.Join(repo, collections.Root, "agents", "atlas", "memories", memoryID+".memory.md")
			if _, err := os.Stat(path); err != nil {
				t.Errorf("memory %s has no file: %v", memoryID, err)
			}
		}
	})

	t.Run("memories come back by search", func(t *testing.T) {
		out, err := second.Memories.Recall(parityCtx(), memory.RecallInput{Query: "orchestrator workspace"})
		if err != nil {
			t.Fatal(err)
		}
		if out.Total != 1 || out.Memories[0].ID != stored[1] {
			t.Fatalf("recall = %d results", out.Total)
		}
		if !out.Indexed {
			t.Error("an active workspace has an index, and this query should have used it")
		}
	})

	t.Run("memories come back by scope", func(t *testing.T) {
		out, err := second.Memories.Recall(parityCtx(), memory.RecallInput{
			Scopes: []string{"internal/domain/**"}, ScopesMode: memory.ScopesStrict,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.Total != 1 || out.Memories[0].ID != stored[0] {
			t.Fatalf("recall = %d results", out.Total)
		}
	})

	t.Run("superseding preserves the lineage", func(t *testing.T) {
		out, err := second.Memories.Store(parityCtx(), memory.StoreInput{
			Title:       "Workspace metadata moved under the state directory",
			Description: "Superseding the earlier note now that the split is settled.",
			Category:    memory.CatDecision,
			Links:       []string{stored[1]},
			Supersedes: []memory.Super{{
				ID:     stored[0],
				Reason: "The earlier note described the split before the registry path was decided.",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Incomplete) != 0 {
			t.Fatalf("the supersede did not complete: %v", out.Incomplete)
		}

		old, err := second.Memories.Reflect(parityCtx(), memory.ReflectInput{Memory: stored[0]})
		if err != nil {
			t.Fatal(err)
		}
		if old.Status != memory.StatusDeprecated || old.DeprecatedBy != out.Memory.ID {
			t.Fatalf("lineage = %q / %q", old.Status, old.DeprecatedBy)
		}
		stored = append(stored, out.Memory.ID)
	})

	t.Run("the graph shows the shape of what is known", func(t *testing.T) {
		g, err := second.Memories.Graph(parityCtx(), memory.GraphInput{})
		if err != nil {
			t.Fatal(err)
		}
		if len(g.Nodes) != 3 {
			t.Fatalf("nodes = %d, want the three memories", len(g.Nodes))
		}

		var kinds []string
		for _, e := range g.Edges {
			kinds = append(kinds, e.Type)
		}
		if !contains(kinds, "reference") || !contains(kinds, "supersedes") {
			t.Fatalf("edges = %v, want both kinds", kinds)
		}
		if g.Health.DeprecatedPct == 0 {
			t.Error("one of three memories is deprecated and the health says none is")
		}
		total := 0
		for _, c := range g.Counts {
			total += c.Count
		}
		if total != len(g.Nodes) {
			t.Errorf("the per-category counts add up to %d, the graph has %d nodes", total, len(g.Nodes))
		}
	})

	t.Run("the inventory reports without reading bodies", func(t *testing.T) {
		got, err := second.Workspaces.Inventory(parityCtx(), workspace.InventoryInput{})
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]int{}
		for _, c := range got.Collections {
			byName[c.Name] = c.Count
		}
		if byName["agents"] != 1 || byName["memories"] != 3 {
			t.Fatalf("inventory = %+v", got.Collections)
		}
		if got.Total != 4 {
			t.Errorf("total = %d", got.Total)
		}
	})

	t.Run("registering the same repository twice adopts it", func(t *testing.T) {
		again, err := second.Workspaces.Introspect(parityCtx(), workspace.IntrospectInput{Path: repo})
		if err != nil {
			t.Fatal(err)
		}
		if !again.Adopted || again.Workspace.ID != id {
			t.Fatalf("second registration = %+v", again)
		}
	})
}

// newAppAt builds an installation over a specific state directory and
// repository, which is what lets one test run two processes against the same
// files.
func newAppAt(t *testing.T, home, repo, activeID string) *app.App {
	t.Helper()
	settings := map[string]string{env.KeyHome: home}
	if activeID != "" {
		settings[env.KeyWorkspaceID] = activeID
	}
	a, err := app.New(app.Options{
		Env:           env.New(env.Map(settings)),
		WorkspaceRoot: repo,
		Clock:         clockx.Fixed{At: refTime},
		IDs:           &ids.Sequence{Prefix: "m"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func contains(v []string, want string) bool {
	for _, s := range v {
		if s == want {
			return true
		}
	}
	return false
}
