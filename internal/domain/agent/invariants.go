package agent

import (
	"context"

	"github.com/OWNER/aos/internal/core/collections"
)

// maxLeaderDepth bounds the walk up the org chart.
//
// The cycle check below would terminate on its own, because it stops when it
// revisits a node. This bound is for the other failure: a chain long enough
// that walking it means reading a hundred files to answer one write, which is a
// denial of service written as a data structure.
const maxLeaderDepth = 32

// orchestratorOf returns the workspace's orchestrator, if there is one.
func (s *Service) orchestratorOf(ctx context.Context) (*Agent, error) {
	found, err := s.repo.List(ctx, collections.Query{
		Filters: map[string]any{"orchestrator": true},
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	// Ordered by id so that a workspace which somehow has two — hand-edited
	// files, a merge — resolves to the same one on every call rather than
	// answering differently each time.
	best := found[0]
	for _, a := range found[1:] {
		if a.ID < best.ID {
			best = a
		}
	}
	return &best, nil
}

// demoteOtherOrchestrators enforces the invariant that a workspace has at most
// one orchestrator.
//
// The original documents "there should be at most one" and does not impose it,
// which means the routing of every chat without an explicit mention depends on
// which file the directory listing returns first. Promoting a second one here
// demotes the first, and the change is reported to the caller rather than
// happening silently.
func (s *Service) demoteOtherOrchestrators(ctx context.Context, keep string) (demoted []string, err error) {
	found, err := s.repo.List(ctx, collections.Query{
		Filters: map[string]any{"orchestrator": true},
	})
	if err != nil {
		return nil, err
	}
	for i := range found {
		other := found[i]
		if other.ID == keep {
			continue
		}
		other.Orchestrator = false
		other.UpdatedAt = s.clock.Now()
		if err := s.repo.Update(ctx, &other, collections.Version{}); err != nil {
			return demoted, err
		}
		demoted = append(demoted, other.ID)
	}
	return demoted, nil
}

// checkLeaderChain refuses a leader assignment that would close a loop.
//
// A cycle in the org chart is not a cosmetic problem: prompt assembly walks the
// chain to describe who an agent reports to, and delegation walks it to decide
// escalation. Either would run until it ran out of something.
func (s *Service) checkLeaderChain(ctx context.Context, id, leader string) error {
	if leader == "" {
		return nil
	}
	if leader == id {
		return errLeaderCycle(id, []string{id, leader})
	}

	seen := map[string]bool{id: true}
	chain := []string{id}
	current := leader

	for depth := 0; current != "" && depth < maxLeaderDepth; depth++ {
		chain = append(chain, current)
		if seen[current] {
			return errLeaderCycle(id, chain)
		}
		seen[current] = true

		next, err := s.repo.Get(ctx, collections.Key{"id": current})
		if err != nil {
			// A leader that does not exist yet is allowed: teams are assembled
			// in whatever order the person thinks of them, and a dangling
			// reference is visible in the roster. What is not allowed is a
			// reference that closes a loop, and a missing node closes nothing.
			return nil
		}
		current = normalizeID(next.Leader)
	}

	if current != "" {
		return errLeaderChainTooDeep(id, maxLeaderDepth)
	}
	return nil
}
