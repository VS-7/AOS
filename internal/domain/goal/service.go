package goal

import (
	"context"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/slug"
)

// Service is the goal aggregate: CRUD over Repository, plus Active, which is
// what feeds the workspace inventory in Prompt Assembly — "check active goals
// before planning or executing significant work."
type Service struct {
	repo  Repository
	tasks Tasks
	clock Clock
}

// Deps is what the service is built from.
type Deps struct {
	Repo  Repository
	Tasks Tasks
	Clock Clock
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{repo: d.Repo, tasks: d.Tasks, clock: d.Clock}
}

// Query filters List and Active.
type Query struct {
	Status  []Status `json:"status,omitempty" jsonschema:"Filter by one or more statuses."`
	Project string   `json:"project,omitempty" jsonschema:"Filter goals by project id."`
	Text    string   `json:"query,omitempty" jsonschema:"Full-text search across a goal's id and title."`
}

// ListInput carries Query plus the reason every command needs.
type ListInput struct {
	Query

	command.Reasoning
}

// List returns every goal matching q, independent of what the repository
// still holds.
func (s *Service) List(ctx context.Context, in ListInput) ([]Goal, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("List", err)
	}
	out := make([]Goal, 0, len(found))
	for _, g := range found {
		if !matches(g, in.Query) {
			continue
		}
		out = append(out, g)
	}
	return out, nil
}

// Active returns goals the agent should align with right now — status
// active, nothing else. It is what feeds the workspace inventory in the
// prompt, per docs/03 - Peças Críticas/Prompt Assembly.md.
func (s *Service) Active(ctx context.Context) ([]Goal, error) {
	return s.List(ctx, ListInput{Query: Query{Status: []Status{StatusActive}}})
}

func matches(g Goal, q Query) bool {
	if len(q.Status) > 0 {
		found := false
		for _, st := range q.Status {
			if g.Status == st {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.Project != "" && g.Project != q.Project {
		return false
	}
	if q.Text != "" {
		text := strings.ToLower(q.Text)
		if !strings.Contains(strings.ToLower(g.ID), text) && !strings.Contains(strings.ToLower(g.Title), text) {
			return false
		}
	}
	return true
}

// GetInput names one goal.
type GetInput struct {
	ID string `json:"id" jsonschema:"Identifier of the goal." validate:"required,notblank"`

	command.Reasoning
}

// Get reads one goal.
func (s *Service) Get(ctx context.Context, in GetInput) (*Goal, error) {
	return s.get(ctx, strings.TrimSpace(in.ID))
}

func (s *Service) get(ctx context.Context, id string) (*Goal, error) {
	if id == "" {
		return nil, errNotFound(id)
	}
	found, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	return found, nil
}

// CreateInput is what a new goal needs. ID is derived from Title, the same
// way every slug-keyed native does — see internal/core/slug.
type CreateInput struct {
	Title       string     `json:"title" jsonschema:"What this goal is." validate:"required,notblank"`
	Description string     `json:"description,omitempty" jsonschema:"One line summarising the outcome."`
	Status      Status     `json:"status,omitempty" jsonschema:"One of: active, achieved, abandoned, paused. Defaults to active."`
	Priority    Priority   `json:"priority,omitempty" jsonschema:"How urgent this goal is. Defaults to no_priority."`
	Project     string     `json:"project,omitempty" jsonschema:"Project this goal belongs to, if any."`
	DueAt       *time.Time `json:"dueAt,omitempty" jsonschema:"When this goal is due, if it has a deadline."`
	Skill       string     `json:"skill,omitempty" jsonschema:"Skill installing this goal, if any."`
	Measure     string     `json:"measure,omitempty" jsonschema:"How to tell this goal was actually served."`
	Content     string     `json:"content,omitempty" jsonschema:"Markdown body."`

	command.Reasoning
}

// Create scaffolds a new goal, deriving its id from Title.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Goal, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, errTitleRequired()
	}
	id := slug.Generate(title)
	if id == "" {
		return nil, errTitleRequired()
	}

	status := in.Status
	if status == "" {
		status = StatusActive
	}
	if !status.Valid() {
		return nil, errStatusInvalid(string(status))
	}

	priority := in.Priority
	if priority == "" {
		priority = NoPriority
	}
	if !priority.Valid() {
		return nil, errPriorityInvalid(string(priority))
	}

	now := s.clock.Now()
	g := Goal{
		ID: id, Title: title, Description: in.Description, Status: status,
		Priority: priority,
		Project:  in.Project, DueAt: in.DueAt, Skill: in.Skill, Measure: in.Measure,
		Content:   in.Content,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, &g); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return &g, nil
}

// UpdateInput changes the describable parts of a goal. A nil pointer leaves
// the field unchanged.
type UpdateInput struct {
	ID string `json:"id" jsonschema:"Identifier of the goal to update." validate:"required,notblank"`

	Title       *string    `json:"title,omitempty" jsonschema:"New title. Omit to leave unchanged."`
	Description *string    `json:"description,omitempty" jsonschema:"New one-line summary of the outcome. Omit to leave unchanged."`
	Status      *Status    `json:"status,omitempty" jsonschema:"New lifecycle status: active, achieved, abandoned, or paused. Omit to leave unchanged."`
	Priority    *Priority  `json:"priority,omitempty" jsonschema:"New priority: no_priority, urgent, high, medium or low. Omit to leave unchanged."`
	Project     *string    `json:"project,omitempty" jsonschema:"New project this goal belongs to. Empty string clears it. Omit to leave unchanged."`
	DueAt       *time.Time `json:"dueAt,omitempty" jsonschema:"New due date. Omit to leave unchanged."`
	Measure     *string    `json:"measure,omitempty" jsonschema:"New measure that makes this goal checkable rather than aspirational. Omit to leave unchanged."`
	Content     *string    `json:"content,omitempty" jsonschema:"New body content, in Markdown. Omit to leave unchanged."`

	command.Reasoning
}

// Update changes a goal's describable fields.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Goal, error) {
	id := strings.TrimSpace(in.ID)
	current, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Title != nil {
		current.Title = strings.TrimSpace(*in.Title)
	}
	if in.Description != nil {
		current.Description = *in.Description
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return nil, errStatusInvalid(string(*in.Status))
		}
		current.Status = *in.Status
	}
	if in.Priority != nil {
		if !in.Priority.Valid() {
			return nil, errPriorityInvalid(string(*in.Priority))
		}
		current.Priority = *in.Priority
	}
	if in.Project != nil {
		current.Project = *in.Project
	}
	if in.DueAt != nil {
		current.DueAt = in.DueAt
	}
	if in.Measure != nil {
		current.Measure = *in.Measure
	}
	if in.Content != nil {
		current.Content = *in.Content
	}
	current.UpdatedAt = s.clock.Now()

	toWrite := *current
	if err := s.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// DeleteInput names one goal to remove.
type DeleteInput struct {
	ID string `json:"id" jsonschema:"Identifier of the goal to delete." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput confirms what was removed.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the goal that was deleted."`
}

// Delete removes a goal and disassociates every task that referenced it —
// the tasks themselves are not touched otherwise, exactly the rule
// Project (Go) applies to its own children.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.ID)
	if s.tasks != nil {
		if err := s.tasks.ClearGoal(ctx, id); err != nil {
			return DeleteOutput{}, errWriteFailed("Delete", err)
		}
	}
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: id}, nil
}
