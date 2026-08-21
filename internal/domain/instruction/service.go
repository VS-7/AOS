package instruction

import (
	"context"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/slug"
)

// Service is the instruction aggregate: CRUD over Repository, plus Applicable
// — the query the prompt assembler runs to decide which instructions belong
// in the trusted block of a given turn.
type Service struct {
	repo  Repository
	clock Clock
}

// Deps is what the service is built from.
type Deps struct {
	Repo  Repository
	Clock Clock
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{repo: d.Repo, clock: d.Clock}
}

// ListInput selects the instructions List returns. Skill, given, narrows to
// one skill's own instructions — the same filter the original's list command
// takes. Query is a naive substring match over name, description and
// content, not the weighted fuzzy search the original's schema describes:
// this package has no full-text index of its own, and a plain match is
// honest about that rather than promising ranking it cannot do.
type ListInput struct {
	Skill string `json:"skill,omitempty" jsonschema:"Filter to one skill's own instructions."`
	Query string `json:"query,omitempty" jsonschema:"Substring match over name, description and content."`

	command.Reasoning
}

// ListOutput is every instruction List found, content included — unlike the
// original, which excludes it from a list response as an optimization. This
// package accepts the cost: instructions are few and read rarely enough that
// a second round trip to fetch content a caller almost always wants next is
// the worse trade.
type ListOutput struct {
	Instructions []Instruction `json:"instructions" jsonschema:"Every instruction matching the filter."`
	Total        int           `json:"total" jsonschema:"How many there are."`
}

// List returns every instruction matching the filter.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}

	skill := strings.TrimSpace(in.Skill)
	query := strings.ToLower(strings.TrimSpace(in.Query))
	out := make([]Instruction, 0, len(found))
	for _, i := range found {
		if skill != "" && i.Skill != skill {
			continue
		}
		if query != "" && !matchesQuery(i, query) {
			continue
		}
		out = append(out, i.Clone())
	}
	return ListOutput{Instructions: out, Total: len(out)}, nil
}

func matchesQuery(i Instruction, query string) bool {
	return strings.Contains(strings.ToLower(i.Name), query) ||
		strings.Contains(strings.ToLower(i.Description), query) ||
		strings.Contains(strings.ToLower(i.Content), query)
}

// GetInput names one instruction.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction." validate:"required,notblank"`

	command.Reasoning
}

// Get reads one instruction in full.
func (s *Service) Get(ctx context.Context, in GetInput) (*Instruction, error) {
	return s.get(ctx, strings.TrimSpace(in.ID))
}

// get is the shared lookup every method resolves an instruction through, so
// a not-found instruction reads the same everywhere.
func (s *Service) get(ctx context.Context, id string) (*Instruction, error) {
	if id == "" {
		return nil, errNotFound(id)
	}
	found, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	clone := found.Clone()
	return &clone, nil
}

// CreateInput composes a new instruction. ID is optional: left empty, it is
// derived from Name the way the original derives its own slug — see Create.
//
// Creating an instruction is a workspace-wide policy change, not a private
// one, so instructions_create carries no ReadOnlyHint — the generic
// PreToolUse approval path (ADR-0007) treats an unannotated mutation as
// RiskMedium and routes it through a human when the caller is an agent,
// exactly the "consultive" classification the design doc calls for. Nothing
// in this package requests approval itself; that would duplicate a
// cross-cutting mechanism this system already has one of.
type CreateInput struct {
	ID          string   `json:"id,omitempty" jsonschema:"Identifier for the instruction. Derived from Name when omitted."`
	Name        string   `json:"name" jsonschema:"Human name of the instruction. Example: \"Feature Protocol\"." validate:"required,notblank"`
	Type        string   `json:"type,omitempty" jsonschema:"Categorization for organizational purposes. Example: standards, patterns, workflows."`
	Description string   `json:"description,omitempty" jsonschema:"What this instruction is for."`
	Skill       string   `json:"skill,omitempty" jsonschema:"The skill this instruction ships with, when it is skill-scoped."`
	Paths       []string `json:"paths,omitempty" jsonschema:"Glob patterns matching files this instruction applies to. Empty applies to the whole workspace."`
	Content     string   `json:"content,omitempty" jsonschema:"Markdown body of the instruction."`

	command.Reasoning
}

// Create declares a new instruction. The id is the slugified name when one is
// not given explicitly, matching the original's own derivation
// (`name.toLowerCase().replace(/\s+/g, "-")`) closely enough that a workspace
// migrating from the original keeps the same ids — slug.Generate is stricter
// about punctuation, which only ever produces a cleaner id, never a
// different one for the names the original actually accepted.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Instruction, error) {
	name := strings.TrimSpace(in.Name)
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = slug.Generate(name)
	}
	if id == "" {
		return nil, errNameRequired()
	}

	if _, err := s.get(ctx, id); err == nil {
		return nil, errAlreadyExists(id)
	}

	now := s.clock.Now()
	instr := Instruction{
		ID:          id,
		Name:        name,
		Type:        strings.TrimSpace(in.Type),
		Description: in.Description,
		Skill:       in.Skill,
		Paths:       clonePaths(in.Paths),
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Content:     in.Content,
	}
	if err := s.repo.Create(ctx, &instr); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return &instr, nil
}

// UpdateInput changes an existing instruction. A nil pointer leaves that
// field unchanged; Paths, given at all, replaces the field wholesale — the
// same "no per-key merge" rule toolset.UpdateConfigInput already documents.
type UpdateInput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction to update." validate:"required,notblank"`

	Name        *string  `json:"name,omitempty" jsonschema:"New name. Omit to leave unchanged."`
	Type        *string  `json:"type,omitempty" jsonschema:"New categorization. Omit to leave unchanged."`
	Description *string  `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
	Paths       []string `json:"paths,omitempty" jsonschema:"New glob patterns, replacing the old ones wholesale."`
	Content     *string  `json:"content,omitempty" jsonschema:"New body. Omit to leave unchanged."`
	Active      *bool    `json:"active,omitempty" jsonschema:"New lifecycle state. Omit to leave unchanged."`

	command.Reasoning
}

// Update changes the describable parts of an instruction. See CreateInput's
// own doc for why this, like Create, requests no approval itself.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Instruction, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Type != nil {
		current.Type = strings.TrimSpace(*in.Type)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Paths != nil {
		current.Paths = clonePaths(in.Paths)
	}
	if in.Content != nil {
		current.Content = *in.Content
	}
	if in.Active != nil {
		current.Active = *in.Active
	}
	current.UpdatedAt = s.clock.Now()

	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// DeleteInput names one instruction to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction to delete." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the instruction that was deleted."`
}

// Delete removes an instruction. Idempotent, like every native record's
// delete in this codebase: deleting what is already gone succeeds.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}

// Applicable returns every active instruction that applies to at least one of
// paths — or that has no Paths at all, which means workspace-wide, the
// default and the common case per the design doc's own decision. It is what
// internal/runtime/prompt calls to build the trusted instruction block for a
// turn; see internal/domain/instruction/INTEGRATION.md for the exact wiring.
//
// Unlike memory.ScopesMode, there is no strict variant: an instruction with
// no Paths is *always* workspace-wide, never excluded, because that is what
// "no paths given" means for a policy rule — there is no reading of the
// original or the design doc where an unscoped instruction should sometimes
// not apply.
func (s *Service) Applicable(ctx context.Context, paths []string) ([]Instruction, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("Applicable", err)
	}
	out := make([]Instruction, 0, len(found))
	for _, i := range found {
		if !i.Active {
			continue
		}
		if len(i.Paths) == 0 || matchesAnyPath(i.Paths, paths) {
			out = append(out, i.Clone())
		}
	}
	return out, nil
}

// matchesAnyPath reports whether any of candidates matches any of patterns.
// An invalid pattern matches nothing rather than failing the whole call — the
// same defensive rule memory.matchScopes already applies to its own globs.
func matchesAnyPath(patterns, candidates []string) bool {
	for _, pattern := range patterns {
		for _, candidate := range candidates {
			if ok, err := doublestar.Match(pattern, candidate); err == nil && ok {
				return true
			}
		}
	}
	return false
}

func clonePaths(p []string) []string {
	if p == nil {
		return nil
	}
	out := make([]string, len(p))
	copy(out, p)
	return out
}
