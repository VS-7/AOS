package project

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
)

// Service is the project aggregate: a durable container, with Delete
// unlinking rather than cascading — see Delete's own doc.
type Service struct {
	repo      Repository
	unlinkers []Unlinker
	clock     Clock
	stat      PathStat
}

// Deps is what the service is built from.
type Deps struct {
	Repo Repository

	// Unlinkers is asked, in order, to drop every reference to a project
	// Delete removes. Task's adapter is wired today; Goal's is meant to join
	// it once that domain exists — see this package's own INTEGRATION.md.
	Unlinkers []Unlinker

	Clock Clock

	// Stat validates Source. Nil uses the real filesystem — see PathStat's
	// own doc.
	Stat PathStat
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	stat := d.Stat
	if stat == nil {
		stat = osStat{}
	}
	return &Service{repo: d.Repo, unlinkers: d.Unlinkers, clock: d.Clock, stat: stat}
}

// Query filters List.
type Query struct {
	Status Status `json:"status,omitempty" jsonschema:"Only projects in this status. Omit for every status."`
}

// ListInput carries the query and the reason for it.
type ListInput struct {
	Query
	command.Reasoning
}

// List returns every project matching q, independent of what the repository
// still holds.
func (s *Service) List(ctx context.Context, in ListInput) ([]Project, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("List", err)
	}
	out := make([]Project, 0, len(found))
	for _, p := range found {
		if in.Status != "" && p.Status != in.Status {
			continue
		}
		out = append(out, p.Clone())
	}
	return out, nil
}

// GetInput names one project.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the project." validate:"required,notblank"`

	command.Reasoning
}

// Get reads one project.
func (s *Service) Get(ctx context.Context, in GetInput) (*Project, error) {
	return s.get(ctx, strings.TrimSpace(in.ID))
}

// get is the shared lookup Get, Update and Delete all resolve a project
// through, so a not-found project reads the same everywhere.
func (s *Service) get(ctx context.Context, id string) (*Project, error) {
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

// CreateInput is what a new project needs.
type CreateInput struct {
	ID          string   `json:"id" jsonschema:"Identifier for the new project. A slug derived from name if omitted." `
	Name        string   `json:"name" jsonschema:"Human-readable name." validate:"required,notblank"`
	Description string   `json:"description,omitempty" jsonschema:"Short description."`
	Status      Status   `json:"status,omitempty" jsonschema:"Lifecycle status: active, paused, done or archived. Defaults to active."`
	Color       string   `json:"color,omitempty" jsonschema:"Display color."`
	Icon        string   `json:"icon,omitempty" jsonschema:"Lucide icon name, image URL, or data URI."`
	Source      string   `json:"source,omitempty" jsonschema:"Absolute host directory this project is bound to. Validated: must be absolute, exist, and be a directory."`
	Paths       []string `json:"paths,omitempty" jsonschema:"Globs this project owns, matched with doublestar."`
	Content     string   `json:"content,omitempty" jsonschema:"Markdown body."`

	command.Reasoning
}

// Create makes a new project.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Project, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errNameRequired()
	}
	if in.Source != "" {
		if err := s.validateSource(in.Source); err != nil {
			return nil, err
		}
	}
	status := in.Status
	if status == "" {
		status = Active
	}
	if !status.Valid() {
		status = Active
	}

	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = slugify(name)
	}

	now := s.clock.Now()
	p := Project{
		ID: id, Name: name, Description: in.Description,
		Status: status, Color: in.Color, Icon: in.Icon, Source: in.Source,
		Paths:     append([]string(nil), in.Paths...),
		CreatedAt: now, UpdatedAt: now,
		Content: in.Content,
	}
	if err := s.repo.Create(ctx, &p); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return &p, nil
}

// UpdateInput changes a project. A nil pointer leaves that field unchanged;
// Paths, given at all, replaces the field wholesale.
type UpdateInput struct {
	ID string `json:"id" jsonschema:"Identifier of the project to update." validate:"required,notblank"`

	Name        *string  `json:"name,omitempty" jsonschema:"New name. Omit to leave unchanged."`
	Description *string  `json:"description,omitempty" jsonschema:"New description. Omit to leave unchanged."`
	Status      *Status  `json:"status,omitempty" jsonschema:"New lifecycle status. Omit to leave unchanged."`
	Color       *string  `json:"color,omitempty" jsonschema:"New display color. Omit to leave unchanged."`
	Icon        *string  `json:"icon,omitempty" jsonschema:"New display icon. Omit to leave unchanged."`
	Source      *string  `json:"source,omitempty" jsonschema:"New source directory — an absolute, existing path. Omit to leave unchanged."`
	Paths       []string `json:"paths,omitempty" jsonschema:"New list of associated paths. Replaces the field wholesale when given."`
	Content     *string  `json:"content,omitempty" jsonschema:"New body content, in Markdown. Omit to leave unchanged."`

	command.Reasoning
}

// Update changes the describable parts of a project.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Project, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		current.Description = *in.Description
	}
	if in.Status != nil && in.Status.Valid() {
		current.Status = *in.Status
	}
	if in.Color != nil {
		current.Color = *in.Color
	}
	if in.Icon != nil {
		current.Icon = *in.Icon
	}
	if in.Source != nil {
		if *in.Source != "" {
			if err := s.validateSource(*in.Source); err != nil {
				return nil, err
			}
		}
		current.Source = *in.Source
	}
	if in.Paths != nil {
		current.Paths = append([]string(nil), in.Paths...)
	}
	if in.Content != nil {
		current.Content = *in.Content
	}
	current.UpdatedAt = s.clock.Now()

	toWrite := current.Clone()
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// DeleteInput names one project to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the project to delete." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the project that was deleted."`
}

// Delete removes a project's own record. It never cascades: every Unlinker
// is asked first to drop its references to id, so the tasks and goals that
// were organized under this project keep existing, just no longer grouped —
// cascading here would destroy legitimate history over an organizing
// operation, which is the one thing a project is not allowed to do.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	for _, u := range s.unlinkers {
		if err := u.UnlinkProject(ctx, id); err != nil {
			return DeleteOutput{}, errWriteFailed("Delete", err)
		}
	}
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}

// validateSource enforces the original's three checks: absolute, exists, is
// a directory — a source that fails silently would bind a project to a path
// an agent can never actually reach.
func (s *Service) validateSource(source string) error {
	if !filepath.IsAbs(source) {
		return errSourceInvalid(source, "not_absolute")
	}
	info, err := s.stat.Stat(source)
	if err != nil {
		return errSourceInvalid(source, "not_found")
	}
	if !info.IsDir() {
		return errSourceInvalid(source, "not_directory")
	}
	return nil
}

// slugify derives an id from a name the same coarse way the original's
// Slug.generate does: lowercase, non-alphanumerics collapsed to a hyphen.
func slugify(name string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
