package memory

import (
	"context"
	"sort"

	"github.com/OWNER/aos/internal/core/collections"
)

// graphLimit bounds the map. A graph larger than this is not read, it is
// skimmed, and the caller wants a filter rather than more nodes.
const graphLimit = 1000

// graphDescriptionMax truncates a node's summary. The map is meant to be
// scanned; the full text is one Reflect away.
const graphDescriptionMax = 320

// hubDegree is the number of edges at which a memory counts as a hub — the
// point where other knowledge is hanging off it rather than merely referring
// to it.
const hubDegree = 3

// Graph maps what an agent knows and how it connects.
//
// Beyond nodes and edges it reports the shape: which memories everything hangs
// off, which are isolated, how confident the whole is, and how much of it has
// been superseded. An agent that can only see its knowledge cannot tell that
// half of it is unlinked.
func (s *Service) Graph(ctx context.Context, in GraphInput) (Graph, error) {
	owner := s.resolveAgent(ctx, in.Agent)
	if in.Category != "" && !in.Category.Valid() {
		return Graph{}, errInvalidCategory(string(in.Category))
	}

	q := collections.Query{}
	if owner != "" {
		q.Key = collections.Key{"agent": owner}
	}
	found, err := s.repo.List(ctx, q)
	if err != nil {
		return Graph{}, errReadFailed("Graph", err)
	}

	mode := in.ScopesMode
	if mode == "" {
		mode = ScopesLax
	}
	limit := in.Limit
	if limit <= 0 || limit > graphLimit {
		limit = graphLimit
	}

	// Ordering before truncation makes the map deterministic: the same graph
	// twice is the same graph, which a caller comparing two runs depends on.
	sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })

	nodes := make(map[string]*Node, len(found))
	order := make([]string, 0, len(found))
	counts := map[Category]int{}
	var confidenceSum float64
	var deprecated int

	for _, m := range found {
		if in.Category != "" && m.Category != in.Category {
			continue
		}
		if in.MinConfidence > 0 && m.Confidence < in.MinConfidence {
			continue
		}
		if !matchScopes(m, in.Scopes, mode) {
			continue
		}
		if len(order) >= limit {
			break
		}
		nodes[m.ID] = &Node{
			ID:          m.ID,
			Title:       m.Title,
			Description: truncate(m.Description, graphDescriptionMax),
			Category:    m.Category,
			Status:      normalizeStatus(m.Status),
			Confidence:  m.Confidence,
		}
		order = append(order, m.ID)
		counts[m.Category]++
		confidenceSum += m.Confidence
		if normalizeStatus(m.Status) == StatusDeprecated {
			deprecated++
		}
	}

	edges := buildEdges(found, nodes)

	out := Graph{Edges: edges, Counts: countsOf(counts)}
	for _, id := range order {
		if in.Isolated && nodes[id].Degree > 0 {
			continue
		}
		out.Nodes = append(out.Nodes, *nodes[id])
	}
	// Asking for the isolated nodes means asking which memories are about to be
	// lost. Returning the edges of the rest with them would bury the answer.
	if in.Isolated {
		out.Edges = nil
	}

	out.Health = healthOf(order, nodes, confidenceSum, deprecated)
	return out, nil
}

// buildEdges materialises the two kinds of link, skipping the ones that point
// outside the filtered graph. A dangling edge would draw a line to nothing.
func buildEdges(found []Memory, nodes map[string]*Node) []Edge {
	var edges []Edge
	seen := map[string]bool{}

	add := func(from, to, kind string) {
		if from == to || nodes[from] == nil || nodes[to] == nil {
			return
		}
		key := kind + ":" + from + "->" + to
		if seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, Edge{From: from, To: to, Type: kind})
		nodes[from].Degree++
		nodes[to].Degree++
	}

	for _, m := range found {
		for _, linked := range m.Links {
			add(m.ID, linked, "reference")
		}
		for _, sup := range m.Supersedes {
			add(m.ID, sup.ID, "supersedes")
		}
	}
	return edges
}

func healthOf(order []string, nodes map[string]*Node, confidenceSum float64, deprecated int) Health {
	var h Health
	if len(order) == 0 {
		return h
	}
	for _, id := range order {
		switch n := nodes[id]; {
		case n.Degree == 0:
			h.Silos = append(h.Silos, id)
		case n.Degree >= hubDegree:
			h.Hubs = append(h.Hubs, id)
		}
	}
	// Most connected first: the hub list is read as a ranking.
	sort.SliceStable(h.Hubs, func(i, j int) bool {
		a, b := nodes[h.Hubs[i]].Degree, nodes[h.Hubs[j]].Degree
		if a == b {
			return h.Hubs[i] < h.Hubs[j]
		}
		return a > b
	})
	h.AvgConfidence = confidenceSum / float64(len(order))
	h.DeprecatedPct = float64(deprecated) / float64(len(order))
	return h
}

// countsOf returns the per-category tally in the canonical category order, so
// that two graphs are comparable line by line.
func countsOf(counts map[Category]int) []Count {
	out := make([]Count, 0, len(counts))
	for _, c := range Categories {
		if n := counts[c]; n > 0 {
			out = append(out, Count{Category: c, Count: n})
		}
	}
	return out
}

func truncate(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-1]) + "…"
}
