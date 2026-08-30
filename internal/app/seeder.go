package app

import (
	"context"

	"github.com/OWNER/aos/internal/adapters/fscollections"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// The workspace aggregate decides when an orchestrator must exist and what it
// should say; it does not know how an agent is stored. These two types close
// that gap, and the composition root is where they belong: they are the only
// place two aggregates are joined, and joining them anywhere else would make
// one depend on the other's persistence.

// seeder creates the first agent of a workspace.
type seeder struct {
	lock  *collections.PathLock
	index *fscollections.Index
	clock clockx.Clock
}

func newSeeder(lock *collections.PathLock, index *fscollections.Index, clock clockx.Clock) *seeder {
	return &seeder{lock: lock, index: index, clock: clock}
}

// serviceFor binds an agent service to a repository root that is not the one
// the process was started in — creating a workspace writes into whichever
// directory the caller named.
func (s *seeder) serviceFor(root string) (*agent.Service, error) {
	repos, err := newRepoSet(root, s.lock, s.index, collections.NopPublisher{})
	if err != nil {
		return nil, err
	}
	return agent.NewService(repos.agents, s.clock), nil
}

func (s *seeder) FindOrchestrator(ctx context.Context, root string) (string, bool, error) {
	svc, err := s.serviceFor(root)
	if err != nil {
		return "", false, err
	}
	found, err := svc.List(ctx, agent.ListInput{})
	if err != nil {
		return "", false, err
	}
	for _, a := range found.Agents {
		if a.Orchestrator {
			return a.ID, true, nil
		}
	}
	return "", false, nil
}

func (s *seeder) SeedOrchestrator(ctx context.Context, in workspace.OrchestratorSeed) (string, error) {
	svc, err := s.serviceFor(in.Root)
	if err != nil {
		return "", err
	}
	created, err := svc.Create(ctx, agent.CreateInput{
		ID:           in.ID,
		Name:         in.Name,
		Role:         in.Role,
		Description:  in.Description,
		Content:      in.Instructions,
		Orchestrator: true,
		Sandbox:      sandboxFrom(in.Sandbox),
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

// sandboxFrom translates the workspace domain's own sandbox shape into the
// agent record's. The two are declared separately because the dependency runs
// one way: workspace knows the Seeder port, not the agent domain.
func sandboxFrom(in *workspace.SandboxSeed) *agent.Sandbox {
	if in == nil {
		return nil
	}
	out := &agent.Sandbox{Permissions: in.Permissions}
	if in.Exec != nil {
		out.Exec = &agent.Exec{
			Policy:     in.Exec.Policy,
			Allow:      in.Exec.Allow,
			AllowShell: in.Exec.AllowShell,
		}
	}
	return out
}

// surveyor counts what a workspace holds, collection by collection.
type surveyor struct {
	lock  *collections.PathLock
	index *fscollections.Index
}

func newSurveyor(lock *collections.PathLock, index *fscollections.Index) *surveyor {
	return &surveyor{lock: lock, index: index}
}

// maxListedKeys bounds how many identifiers a summary carries. Past this the
// list stops being an orientation and becomes the thing you needed orienting
// through, so only the count is reported.
const maxListedKeys = 50

// Survey walks the native collections and counts their records.
//
// It reads keys, never bodies. This is the call an agent makes at the start of
// a session, and the cheapest question in the system should not be answered by
// loading every file in the repository.
func (s *surveyor) Survey(ctx context.Context, root string) ([]workspace.CollectionSummary, error) {
	repos, err := newRepoSet(root, s.lock, s.index, collections.NopPublisher{})
	if err != nil {
		return nil, err
	}

	agents, err := repos.agents.List(ctx, collections.Query{})
	if err != nil {
		return nil, err
	}
	memories, err := repos.memories.List(ctx, collections.Query{})
	if err != nil {
		return nil, err
	}

	out := []workspace.CollectionSummary{
		{Name: "agents", Count: len(agents)},
		{Name: "memories", Count: len(memories)},
	}
	if len(agents) <= maxListedKeys {
		keys := make([]string, len(agents))
		for i, a := range agents {
			keys[i] = a.ID
		}
		out[0].Keys = keys
	}
	return out, nil
}

// directory answers the two questions chat routing asks, over the agent roster.
//
// It is a type here rather than a method on the agent service because the chat
// aggregate declares the port and the agent aggregate should not know that
// conversations exist.
type directory struct{ agents *agent.Service }

func newDirectory(agents *agent.Service) directory { return directory{agents: agents} }

func (d directory) IsAgent(ctx context.Context, id string) bool {
	found, err := d.agents.Get(ctx, agent.GetInput{ID: id})
	return err == nil && found != nil
}

func (d directory) Orchestrator(ctx context.Context) (string, error) {
	found, err := d.agents.List(ctx, agent.ListInput{})
	if err != nil {
		return "", err
	}
	for _, a := range found.Agents {
		if a.Orchestrator {
			return a.ID, nil
		}
	}
	return "", nil
}
