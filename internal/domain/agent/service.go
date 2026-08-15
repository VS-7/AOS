package agent

import (
	"context"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
)

// Service is the agent aggregate. It governs one aggregate and nothing else:
// it does not write memories, and it does not start chats.
type Service struct {
	repo  Repository
	clock Clock
}

// NewService wires the service over its ports.
func NewService(repo Repository, clock Clock) *Service {
	return &Service{repo: repo, clock: clock}
}

// List returns the agents of the workspace, ordered by id.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	q := collections.Query{}
	if in.Skill != "" {
		q.Key = collections.Key{"skill": in.Skill}
	}
	if in.Role != "" {
		q.Filters = map[string]any{"role": in.Role}
	}
	found, err := s.repo.List(ctx, q)
	if err != nil {
		return ListOutput{}, err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })
	total := len(found)
	if in.Limit > 0 && in.Limit < len(found) {
		found = found[:in.Limit]
	}
	return ListOutput{Agents: found, Total: total}, nil
}

// Get reads one agent, instructions included.
func (s *Service) Get(ctx context.Context, in GetInput) (*Agent, error) {
	id := normalizeID(in.ID)
	got, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, err
	}
	return got, nil
}

// Me resolves the caller's own identity.
//
// Inside an agent execution it returns that agent; from a human terminal it
// returns the orchestrator. This is how an external agent — one running in
// another tool entirely — finds out who it is inside this workspace, without
// having been told its own slug.
func (s *Service) Me(ctx context.Context, _ MeInput) (*Agent, error) {
	if actor, kind := identity.Actor(ctx); kind == identity.ActorAgent {
		return s.Get(ctx, GetInput{ID: actor})
	}
	found, err := s.orchestratorOf(ctx)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errNoOrchestrator()
	}
	return found, nil
}

// Create writes a new agent.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Agent, error) {
	id := normalizeID(in.ID)
	if id == "" {
		return nil, errInvalidID(in.ID)
	}
	leader := normalizeID(in.Leader)
	if err := s.checkLeaderChain(ctx, id, leader); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	a := &Agent{
		ID:           id,
		Name:         in.Name,
		Description:  in.Description,
		Role:         in.Role,
		Leader:       leader,
		Provider:     in.Provider,
		Model:        in.Model,
		Voice:        in.Voice,
		Channels:     in.Channels,
		Orchestrator: in.Orchestrator,
		Content:      in.Content,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if a.Name == "" {
		// The original defaults the name to the id on create.
		a.Name = a.ID
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	// Demotion happens after the write, so a failure to create does not leave
	// the workspace with no orchestrator at all.
	if a.Orchestrator {
		if _, err := s.demoteOtherOrchestrators(ctx, a.ID); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// Update changes the fields the caller sent and leaves the rest alone.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Agent, error) {
	id := normalizeID(in.ID)
	current, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, err
	}
	applyString(&current.Name, in.Name)
	applyString(&current.Description, in.Description)
	applyString(&current.Role, in.Role)
	applyString(&current.Provider, in.Provider)
	applyString(&current.Model, in.Model)
	applyString(&current.Voice, in.Voice)
	applyString(&current.Content, in.Content)
	if in.Leader != nil {
		leader := normalizeID(*in.Leader)
		if err := s.checkLeaderChain(ctx, id, leader); err != nil {
			return nil, err
		}
		current.Leader = leader
	}
	promoted := in.Orchestrator != nil && *in.Orchestrator && !current.Orchestrator
	if in.Orchestrator != nil {
		current.Orchestrator = *in.Orchestrator
	}
	current.UpdatedAt = s.clock.Now()

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, err
	}
	if promoted {
		if _, err := s.demoteOtherOrchestrators(ctx, current.ID); err != nil {
			return nil, err
		}
	}
	return current, nil
}

// Delete removes an agent and, with it, the whole directory: its memories, its
// routines and its event log.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := normalizeID(in.ID)
	if _, err := s.repo.Get(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{ID: id, Deleted: false}, err
	}
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{ID: id}, err
	}
	return DeleteOutput{ID: id, Deleted: true}, nil
}

func applyString(dst *string, v *string) {
	if v != nil {
		*dst = *v
	}
}

// normalizeID lowercases the slug, as the original's onCreated hook does. The
// file name carries the identity, so "Atlas" and "atlas" must not be two agents.
func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
