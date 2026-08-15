package memory_test

import (
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/domain/memory"
)

// link stores a memory that references the given ids.
func link(t *testing.T, svc *memory.Service, title string, links ...string) memory.Memory {
	t.Helper()
	return store(t, svc, memory.StoreInput{Title: title, Links: links}).Memory
}

func TestGraphMapsNodesAndReferenceEdges(t *testing.T) {
	svc, _ := newService(t)
	a := link(t, svc, "A")
	b := link(t, svc, "B", a.ID)
	c := link(t, svc, "C", a.ID, b.ID)

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("nodes = %d", len(g.Nodes))
	}
	if len(g.Edges) != 3 {
		t.Fatalf("edges = %+v", g.Edges)
	}
	for _, e := range g.Edges {
		if e.Type != "reference" {
			t.Errorf("edge type = %q", e.Type)
		}
	}
	if degreeOf(g, c.ID) != 2 {
		t.Errorf("C has degree %d, want 2", degreeOf(g, c.ID))
	}
	if degreeOf(g, a.ID) != 2 {
		t.Errorf("A has degree %d, want 2 — both B and C point at it", degreeOf(g, a.ID))
	}
}

func TestGraphMapsSupersedeEdges(t *testing.T) {
	svc, _ := newService(t)
	old := store(t, svc, memory.StoreInput{Title: "Old"}).Memory
	replacement := store(t, svc, memory.StoreInput{
		Title:      "New",
		Supersedes: []memory.Super{{ID: old.ID, Reason: "The premise changed entirely."}},
	}).Memory

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range g.Edges {
		if e.Type == "supersedes" && e.From == replacement.ID && e.To == old.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("no supersedes edge in %+v", g.Edges)
	}
}

// TestDanglingEdgesAreNotDrawn: a link to a memory outside the filtered graph
// would be a line to nothing.
func TestDanglingEdgesAreNotDrawn(t *testing.T) {
	svc, _ := newService(t)
	link(t, svc, "A", "never-existed")

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 0 {
		t.Fatalf("edges = %+v", g.Edges)
	}
}

func TestASelfLinkIsNotAnEdge(t *testing.T) {
	svc, repo := newService(t)
	m := link(t, svc, "A")
	stored, err := repo.Get(t.Context(), keyOf(m))
	if err != nil {
		t.Fatal(err)
	}
	stored.Links = []string{stored.ID}
	if err := repo.Update(t.Context(), stored, versionZero()); err != nil {
		t.Fatal(err)
	}

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 0 {
		t.Fatalf("edges = %+v", g.Edges)
	}
}

// TestHealthNamesTheHubsAndTheSilos: an agent that can only see its knowledge
// cannot tell that half of it is unreachable.
func TestHealthNamesTheHubsAndTheSilos(t *testing.T) {
	svc, _ := newService(t)
	hub := link(t, svc, "Hub")
	for i := 0; i < 3; i++ {
		link(t, svc, "spoke", hub.ID)
	}
	silo := link(t, svc, "Silo")

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Health.Hubs) == 0 || g.Health.Hubs[0] != hub.ID {
		t.Fatalf("hubs = %v, want %q first", g.Health.Hubs, hub.ID)
	}
	if !contains(g.Health.Silos, silo.ID) {
		t.Fatalf("silos = %v, want it to include %q", g.Health.Silos, silo.ID)
	}
	if contains(g.Health.Silos, hub.ID) {
		t.Error("a hub was reported as a silo")
	}
}

func TestHealthAveragesConfidenceAndCountsTheDeprecated(t *testing.T) {
	svc, _ := newService(t)
	half := 0.5
	one := 1.0
	store(t, svc, memory.StoreInput{Title: "a", Confidence: &half})
	b := store(t, svc, memory.StoreInput{Title: "b", Confidence: &one}).Memory
	if _, err := svc.Forget(ctx(), memory.ForgetInput{
		Memory: b.ID, Reason: "It stopped being true after the migration.",
	}); err != nil {
		t.Fatal(err)
	}

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	if g.Health.AvgConfidence != 0.75 {
		t.Errorf("avgConfidence = %v, want 0.75", g.Health.AvgConfidence)
	}
	if g.Health.DeprecatedPct != 0.5 {
		t.Errorf("deprecatedPct = %v, want 0.5", g.Health.DeprecatedPct)
	}
}

// TestCountsPerCategoryMatchTheGraph is what feeds the prompt: the agent is
// told how much it knows of each kind, not what it knows.
func TestCountsPerCategoryMatchTheGraph(t *testing.T) {
	svc, _ := newService(t)
	store(t, svc, memory.StoreInput{Title: "a", Category: memory.CatDecision})
	store(t, svc, memory.StoreInput{Title: "b", Category: memory.CatDecision})
	store(t, svc, memory.StoreInput{Title: "c", Category: memory.CatLearning})

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, c := range g.Counts {
		total += c.Count
	}
	if total != len(g.Nodes) {
		t.Fatalf("counts total %d, graph has %d nodes", total, len(g.Nodes))
	}
	// Canonical category order, so two graphs are comparable line by line.
	if g.Counts[0].Category != memory.CatDecision || g.Counts[0].Count != 2 {
		t.Fatalf("counts = %+v", g.Counts)
	}
	if g.Counts[1].Category != memory.CatLearning {
		t.Fatalf("counts are not in canonical order: %+v", g.Counts)
	}
}

func TestGraphFiltersByCategoryAndConfidence(t *testing.T) {
	svc, _ := newService(t)
	low := 0.3
	store(t, svc, memory.StoreInput{Title: "a", Category: memory.CatDecision})
	store(t, svc, memory.StoreInput{Title: "b", Category: memory.CatDecision, Confidence: &low})
	store(t, svc, memory.StoreInput{Title: "c", Category: memory.CatFact})

	byCategory, err := svc.Graph(ctx(), memory.GraphInput{Category: memory.CatDecision})
	if err != nil {
		t.Fatal(err)
	}
	if len(byCategory.Nodes) != 2 {
		t.Fatalf("nodes = %d", len(byCategory.Nodes))
	}

	confident, err := svc.Graph(ctx(), memory.GraphInput{MinConfidence: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if len(confident.Nodes) != 2 {
		t.Fatalf("nodes = %d, want the low-confidence one excluded", len(confident.Nodes))
	}
}

// TestIsolatedReturnsOnlyTheOrphansAndNoEdges: the question is "what am I about
// to lose", and edges between the rest would bury the answer.
func TestIsolatedReturnsOnlyTheOrphansAndNoEdges(t *testing.T) {
	svc, _ := newService(t)
	a := link(t, svc, "A")
	link(t, svc, "B", a.ID)
	silo := link(t, svc, "Silo")

	g, err := svc.Graph(ctx(), memory.GraphInput{Isolated: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 1 || g.Nodes[0].ID != silo.ID {
		t.Fatalf("nodes = %v", nodeIDs(g))
	}
	if len(g.Edges) != 0 {
		t.Fatalf("edges = %+v", g.Edges)
	}
}

func TestGraphTruncatesLongDescriptions(t *testing.T) {
	svc, _ := newService(t)
	store(t, svc, memory.StoreInput{Title: "a", Description: strings.Repeat("x", 500)})

	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	got := []rune(g.Nodes[0].Description)
	if len(got) != 320 {
		t.Fatalf("description is %d runes, want it capped at 320", len(got))
	}
	if got[len(got)-1] != '…' {
		t.Error("a truncated description should say it was truncated")
	}
}

func TestGraphIsDeterministic(t *testing.T) {
	svc, _ := newService(t)
	a := link(t, svc, "A")
	link(t, svc, "B", a.ID)
	link(t, svc, "C", a.ID)

	first, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := svc.Graph(ctx(), memory.GraphInput{})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(nodeIDs(again), ",") != strings.Join(nodeIDs(first), ",") {
			t.Fatalf("run %d ordered nodes differently: %v vs %v", i, nodeIDs(again), nodeIDs(first))
		}
		if len(again.Edges) != len(first.Edges) {
			t.Fatalf("run %d has %d edges, first had %d", i, len(again.Edges), len(first.Edges))
		}
		for j := range again.Edges {
			if again.Edges[j] != first.Edges[j] {
				t.Fatalf("run %d ordered edges differently", i)
			}
		}
	}
}

func TestAnEmptyGraphIsNotAnError(t *testing.T) {
	svc, _ := newService(t)
	g, err := svc.Graph(ctx(), memory.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 0 || g.Health.AvgConfidence != 0 {
		t.Fatalf("graph = %+v", g)
	}
}

func degreeOf(g memory.Graph, id string) int {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n.Degree
		}
	}
	return -1
}

func nodeIDs(g memory.Graph) []string {
	out := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		out[i] = n.ID
	}
	return out
}

func contains(v []string, want string) bool {
	for _, s := range v {
		if s == want {
			return true
		}
	}
	return false
}
