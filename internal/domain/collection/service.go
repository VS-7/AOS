package collection

import (
	"context"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
)

// Service is the collection aggregate: the agent-facing surface that turns a
// declaration into a registered collection, and back out again.
type Service struct {
	repo        Repository
	registry    *collections.Registry
	clock       Clock
	recordRepos RecordRepositories
	ids         IDs
}

// Deps is what the service is built from.
//
// RecordRepos and IDs are only used by Records() — declaring a collection
// needs neither a repository factory for its rows nor an id generator, the
// caller supplies the collection's own id. They are still here, on the same
// Deps, rather than on a second constructor: one aggregate, one place to wire
// it, and Task 10's wiring passes all five at once.
type Deps struct {
	Repo        Repository
	Registry    *collections.Registry
	Clock       Clock
	RecordRepos RecordRepositories
	IDs         IDs
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{
		repo: d.Repo, registry: d.Registry, clock: d.Clock,
		recordRepos: d.RecordRepos, ids: d.IDs,
	}
}

// Records returns the aggregate for this collection's rows. It is a method
// rather than a second top-level constructor because a record is meaningless
// without the declaration it is validated against, and the two aggregates
// share the repository that reads that declaration.
func (s *Service) Records() RecordService {
	return &recordService{decls: s.repo, repos: s.recordRepos, clock: s.clock, ids: s.ids}
}

// ListInput selects the declarations List returns. It has no filter today: a
// workspace declares few enough collections that listing all of them is never
// a cost worth guarding.
type ListInput struct{}

// ListOutput is every collection declared in the workspace.
type ListOutput struct {
	Collections []Collection `json:"collections"`
	Total       int          `json:"total"`
}

// GetInput names one declaration.
//
// Skill is optional and only needed for a skill-scoped collection: the
// engine stores that declaration under a second, skill-qualified pattern
// (.aos/skills/{skill}/collections/{id}/schema.json), and {"id": id} alone
// does not resolve it — a Get with the id only, for a collection that turns
// out to be skill-scoped, is refused as not found rather than silently
// returning the wrong thing or scanning every skill for a match. A record
// List returns already carries its own Skill (Collection.Skill), so a caller
// that got an id from List has what it needs to hand back here.
type GetInput struct {
	ID    string `json:"id"`
	Skill string `json:"skill,omitempty"`
}

// CreateInput declares a new collection.
type CreateInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Scope       Scope  `json:"scope,omitempty"`
	Skill       string `json:"skill,omitempty"`
	Format      Format `json:"format"`

	Fields []Field `json:"fields"`
	Hooks  []Hook  `json:"hooks,omitempty"`
}

// DeleteInput removes a declaration. Skill is the same optional qualifier
// GetInput takes, for the same reason.
type DeleteInput struct {
	ID    string `json:"id"`
	Skill string `json:"skill,omitempty"`
}

// List returns every declared collection.
func (s *Service) List(ctx context.Context, _ ListInput) (ListOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, err
	}
	return ListOutput{Collections: found, Total: len(found)}, nil
}

// Get reads one declaration.
func (s *Service) Get(ctx context.Context, in GetInput) (*Collection, error) {
	id := strings.TrimSpace(in.ID)
	found, err := s.repo.Get(ctx, keyOf(id, in.Skill))
	if err != nil {
		return nil, errNotFound(id, s.registry.Names())
	}
	return found, nil
}

// keyOf builds the lookup key Get and Delete share: the id alone for a
// workspace-scoped declaration, or the id plus skill for one that is not —
// an empty skill is left out of the key entirely rather than sent as "",
// because collections.Key.String() folds every present field into the
// identity a repository indexes by, and a key carrying an empty "skill" is
// not the same key as one that never mentioned skill at all.
func keyOf(id, skill string) collections.Key {
	key := collections.Key{"id": id}
	if skill = strings.TrimSpace(skill); skill != "" {
		key["skill"] = skill
	}
	return key
}

// Create declares a collection.
//
// The order matters and is not arbitrary: the id is checked as a path segment
// before anything else runs, the schema is checked for internal coherence
// before it is turned into a descriptor, and the collection is registered only
// once the write to disk has actually succeeded. Registering first would let a
// caller address a collection that a failed write never created; registering
// before validating would let an incoherent schema become visible to every
// reader of the registry for the time it takes the write to fail.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Collection, error) {
	id := strings.TrimSpace(in.ID)
	if err := validName(id); err != nil {
		return nil, err
	}

	scope := in.Scope
	if scope == "" {
		scope = ScopeWorkspace
	}

	now := s.clock.Now()
	c := Collection{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		Scope:       scope,
		Skill:       in.Skill,
		Format:      in.Format,
		Fields:      in.Fields,
		Hooks:       in.Hooks,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// exists closes over the registry rather than the repository: a ref must
	// resolve to something that can serve records right now, native or
	// dynamic, and the registry is what knows both.
	exists := func(name string) bool {
		_, ok := s.registry.Lookup(name)
		return ok
	}
	if err := ValidateSchema(c, exists); err != nil {
		return nil, err
	}

	desc, err := DescriptorFor(c)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, &c); err != nil {
		return nil, err
	}

	// Registered last, and deliberately: a name that resolved to nothing on
	// disk would be a collection an agent could write to and never read back.
	if err := s.registry.Register(desc); err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete removes a declaration.
//
// Unregistering happens before the record is removed, the reverse of Create's
// order and for the reverse reason: nothing may resolve a write against a
// declaration that is on its way out, and unregistering first closes that
// window even if the removal itself fails partway through.
func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	id := strings.TrimSpace(in.ID)
	if _, err := s.Get(ctx, GetInput{ID: id, Skill: in.Skill}); err != nil {
		return err
	}
	if err := s.registry.Unregister(id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, keyOf(id, in.Skill))
}
