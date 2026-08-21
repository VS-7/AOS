package template

import (
	"context"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
)

// Service is the template aggregate: persistence (List, Get, Create, Update,
// Delete) and the one place this system runs user-authored Liquid (Render).
type Service struct {
	repo   Repository
	engine Engine
	clock  Clock
}

// Deps is what the service is built from.
type Deps struct {
	Repo   Repository
	Engine Engine
	Clock  Clock
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{repo: d.Repo, engine: d.Engine, clock: d.Clock}
}

// ListInput selects the templates List returns. Query and Skill mirror the
// original's list filters (docs/04 - Domínio/Template (Go).md); an empty
// ListInput lists everything.
type ListInput struct {
	Skill string `json:"skill,omitempty" jsonschema:"Filter to templates that ship with this skill."`
	Query string `json:"query,omitempty" jsonschema:"Filter by substring match against name and description."`

	command.Reasoning
}

// ListOutput is every template List found, filtered.
type ListOutput struct {
	Templates []Template `json:"templates" jsonschema:"Every template matching the filter."`
	Total     int        `json:"total" jsonschema:"How many there are."`
}

// List returns every template matching in's filter.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}

	skill := strings.TrimSpace(in.Skill)
	query := strings.ToLower(strings.TrimSpace(in.Query))

	out := make([]Template, 0, len(found))
	for _, t := range found {
		if skill != "" && t.Skill != skill {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(t.Name), query) &&
			!strings.Contains(strings.ToLower(t.Description), query) {
			continue
		}
		out = append(out, t.Clone())
	}
	return ListOutput{Templates: out, Total: len(out)}, nil
}

// GetInput names one template.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the template." validate:"required,notblank"`

	command.Reasoning
}

// Get reads one template in full, including its Liquid content and declared
// Variables — what templates_get exists for: inspect before render.
func (s *Service) Get(ctx context.Context, in GetInput) (*Template, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, errNotFound(id)
	}
	found, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	out := found.Clone()
	return &out, nil
}

// CreateInput composes a new template. Content is validated as Liquid before
// anything is written — see validateContent.
type CreateInput struct {
	ID          string     `json:"id" jsonschema:"Identifier for the template. Also its file name: lowercase, digits, hyphen and underscore only." validate:"required,notblank"`
	Name        string     `json:"name" jsonschema:"Human name of the template." validate:"required,notblank"`
	Description string     `json:"description,omitempty" jsonschema:"What this template produces and when to use it."`
	Skill       string     `json:"skill,omitempty" jsonschema:"The skill this template ships with, when Scope is skill."`
	Variables   []Variable `json:"variables,omitempty" jsonschema:"The variables this template's body expects."`
	Output      string     `json:"output,omitempty" jsonschema:"Suggested relative output path for a render of this template."`
	Content     string     `json:"content" jsonschema:"The Liquid template body." validate:"required"`

	command.Reasoning
}

// Create validates Content as Liquid before writing anything — a broken
// template is refused here, never the thing an agent discovers by rendering
// it later.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Template, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, errIDRequired()
	}
	if err := s.validateContent(id, in.Content); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	t := Template{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		Skill:       in.Skill,
		Variables:   append([]Variable(nil), in.Variables...),
		Output:      in.Output,
		Content:     in.Content,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, &t); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	out := t.Clone()
	return &out, nil
}

// UpdateInput changes an existing template. A nil pointer leaves that field
// unchanged; Variables, when non-nil, replaces the field wholesale — there is
// no per-entry merge, the same rule toolset.UpdateConfigInput applies to its
// own slice and map fields.
type UpdateInput struct {
	ID string `json:"id" jsonschema:"Identifier of the template to update." validate:"required,notblank"`

	Name        *string    `json:"name,omitempty" jsonschema:"New name. Omit to leave unchanged."`
	Description *string    `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
	Variables   []Variable `json:"variables,omitempty" jsonschema:"New variable contract, replacing the old one wholesale."`
	Output      *string    `json:"output,omitempty" jsonschema:"New suggested output path. Omit to leave unchanged."`
	Content     *string    `json:"content,omitempty" jsonschema:"New Liquid body. Omit to leave unchanged. Re-validated as Liquid when given."`

	command.Reasoning
}

// Update changes the describable parts of a template. New Content, if given,
// is re-validated as Liquid before anything is written, the same as Create.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Template, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.Get(ctx, GetInput{ID: id})
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Variables != nil {
		current.Variables = append([]Variable(nil), in.Variables...)
	}
	if in.Output != nil {
		current.Output = strings.TrimSpace(*in.Output)
	}
	if in.Content != nil {
		if err := s.validateContent(id, *in.Content); err != nil {
			return nil, err
		}
		current.Content = *in.Content
	}
	current.UpdatedAt = s.clock.Now()

	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// DeleteInput names one template to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the template to delete." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the template that was deleted."`
}

// Delete removes a template. Idempotent, like every native record's
// repository: deleting what is already gone succeeds rather than erroring.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}

// RenderInput names the template to render and the variables to render it
// with.
type RenderInput struct {
	ID        string         `json:"id" jsonschema:"Identifier of the template to render." validate:"required,notblank"`
	Variables map[string]any `json:"variables,omitempty" jsonschema:"Values for the template's declared Variables. A missing Required variable with no Default refuses the render."`

	command.Reasoning
}

// RenderResult is what a render produced.
type RenderResult struct {
	Output string `json:"output" jsonschema:"The rendered text."`
}

// Render validates in.Variables against the template's declared contract,
// then runs Content through the engine under a timeout and an output size
// cap — see render() in render.go for what that boundary actually enforces
// and why.
//
// It writes nothing to disk: the entity's own Output field is a suggested
// path for a caller to use, not a target this method resolves and writes to
// itself — the original does write render output to disk as its primary
// scaffolding path, and this build does not yet. See this package's
// INTEGRATION.md for the gap and what wiring it would need.
func (s *Service) Render(ctx context.Context, in RenderInput) (RenderResult, error) {
	t, err := s.Get(ctx, GetInput{ID: in.ID})
	if err != nil {
		return RenderResult{}, err
	}

	vars, err := validateVariables(t.ID, t.Variables, in.Variables)
	if err != nil {
		return RenderResult{}, err
	}

	out, err := render(ctx, s.engine, t.ID, t.Content, vars)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{Output: out}, nil
}

// validateContent refuses Content that does not parse as Liquid, unless it
// carries no delimiter at all — a plain-text template has nothing to
// validate, the same "no placeholders, no parse" optimization ADR-0014
// records from the original.
func (s *Service) validateContent(id, content string) error {
	if !hasDelimiters(content) {
		return nil
	}
	if err := s.engine.Validate(content); err != nil {
		return errContentInvalid(id, err)
	}
	return nil
}
