package task

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/slug"
)

// Service is the task aggregate.
type Service struct {
	repo      Repository
	plan      Plan
	directory Directory
	worktrees Worktrees
	setup     Setup
	policy    Policy
	notifier  Notifier
	clock     Clock
	ids       IDs
	log       *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo  Repository
	Clock Clock
	IDs   IDs

	// Plan is the review guard's source of truth. Without it a task can reach
	// in_review with an unfinished plan, so the composition root always
	// supplies it; nil is for the tests that are about something else.
	Plan Plan

	// Directory resolves an assignee. Without it every assignee is unknown, and
	// nothing is dispatched — which is the safe direction.
	Directory Directory

	Worktrees Worktrees
	Setup     Setup
	Policy    Policy
	Notifier  Notifier

	Log *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		repo: d.Repo, plan: d.Plan, directory: d.Directory,
		worktrees: d.Worktrees, setup: d.Setup, policy: d.Policy,
		notifier: d.Notifier, clock: d.Clock, ids: d.IDs, log: log,
	}
}

// Exists reports whether a task is there. It is the Parent port the todo and
// comment aggregates hold, narrowed to the one question they ask.
func (s *Service) Exists(ctx context.Context, taskID string) (bool, error) {
	if _, err := s.repo.Get(ctx, collections.Key{"id": strings.TrimSpace(taskID)}); err != nil {
		return false, nil //nolint:nilerr // a missing task is an answer, not a failure
	}
	return true, nil
}

// List returns the tasks matching a query, with their projections.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	if in.Status != "" && !in.Status.Valid() {
		return ListOutput{}, errInvalidStatus(string(in.Status))
	}
	found, err := s.repo.List(ctx, collections.Query{IncludeContent: false})
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}

	matched := make([]Task, 0, len(found))
	for _, t := range found {
		if in.Status != "" && t.Status != in.Status {
			continue
		}
		if in.Type != "" && t.Type != in.Type {
			continue
		}
		if in.Assigned != "" && !strings.EqualFold(t.Assigned, in.Assigned) {
			continue
		}
		if in.Project != "" && t.Project != in.Project {
			continue
		}
		if in.Goal != "" && t.Goal != in.Goal {
			continue
		}
		matched = append(matched, t)
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID < matched[j].ID
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := len(matched)
	if in.Offset > 0 {
		if in.Offset >= len(matched) {
			matched = nil
		} else {
			matched = matched[in.Offset:]
		}
	}
	if in.Limit > 0 && in.Limit < len(matched) {
		matched = matched[:in.Limit]
	}

	views := make([]View, 0, len(matched))
	for i := range matched {
		view, err := s.view(ctx, &matched[i])
		if err != nil {
			return ListOutput{}, err
		}
		views = append(views, view)
	}
	return ListOutput{Tasks: views, Total: total}, nil
}

// Get reads one task with its projections.
func (s *Service) Get(ctx context.Context, in GetInput) (*View, error) {
	current, err := s.load(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	view, err := s.view(ctx, current)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// Create records a new task.
//
// It starts in the status the caller asks for, defaulting to backlog. A task
// created straight into in_progress would skip every guard on the way there,
// so the only openings are the entry points of the graph.
func (s *Service) Create(ctx context.Context, in CreateInput) (*View, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errInvalidName(in.Name)
	}
	generated := slug.Generate(name)
	if generated == "" {
		return nil, errInvalidName(in.Name)
	}

	status := in.Status
	if status == "" {
		status = Backlog
	}
	if !status.Valid() {
		return nil, errInvalidStatus(string(status))
	}
	if !isEntryPoint(status) {
		return nil, errNotAnEntryPoint(status)
	}

	priority := in.Priority
	if priority == "" {
		priority = NoPriority
	}
	if !priority.Valid() {
		return nil, errInvalidPriority(string(priority))
	}
	if err := s.checkType(ctx, in.Type); err != nil {
		return nil, err
	}

	var due *time.Time
	if in.DueAt != "" {
		parsed, err := time.Parse(time.RFC3339, in.DueAt)
		if err != nil {
			return nil, errInvalidTime("dueAt", in.DueAt, err)
		}
		due = &parsed
	}

	now := s.clock.Now()
	t := &Task{
		ID:        s.ids.New(),
		Name:      name,
		Slug:      generated,
		Type:      in.Type,
		Assigned:  strings.TrimSpace(in.Assigned),
		DueAt:     due,
		Priority:  priority,
		Summary:   in.Summary,
		Status:    status,
		Project:   in.Project,
		Goal:      in.Goal,
		DependsOn: in.DependsOn,
		Worktree:  Worktree{Enabled: in.Worktree, Base: in.Base},
		CreatedAt: now,
		UpdatedAt: now,
		Content:   strings.TrimLeft(in.Content, " \t\n\r"),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	s.notify(ctx, "created", t, nil)

	view, err := s.view(ctx, t)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// Update changes the describable parts of a task.
//
// Status is not one of them, and the attempt is refused rather than ignored.
// This is the rule the original states as prose — "use set_status for lifecycle
// moves; never change status via update" — made mechanical.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*View, error) {
	current, err := s.load(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if in.Status != "" {
		return nil, errStatusIsNotAField(in.ID)
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errInvalidName(*in.Name)
		}
		current.Name = name
		current.Slug = slug.Generate(name)
	}
	if in.Type != nil {
		if err := s.checkType(ctx, *in.Type); err != nil {
			return nil, err
		}
		current.Type = *in.Type
	}
	if in.Assigned != nil {
		current.Assigned = strings.TrimSpace(*in.Assigned)
	}
	if in.Priority != nil {
		if !in.Priority.Valid() {
			return nil, errInvalidPriority(string(*in.Priority))
		}
		current.Priority = *in.Priority
	}
	if in.Summary != nil {
		current.Summary = *in.Summary
	}
	if in.Project != nil {
		current.Project = *in.Project
	}
	if in.Goal != nil {
		current.Goal = *in.Goal
	}
	if in.DependsOn != nil {
		if err := s.checkDependencies(ctx, current.ID, *in.DependsOn); err != nil {
			return nil, err
		}
		current.DependsOn = *in.DependsOn
	}
	if in.DueAt != nil {
		if *in.DueAt == "" {
			current.DueAt = nil
		} else {
			parsed, err := time.Parse(time.RFC3339, *in.DueAt)
			if err != nil {
				return nil, errInvalidTime("dueAt", *in.DueAt, err)
			}
			current.DueAt = &parsed
		}
	}
	if in.Content != nil {
		current.Content = strings.TrimLeft(*in.Content, " \t\n\r")
	}
	if in.Chat != nil {
		current.Chat = *in.Chat
	}
	current.UpdatedAt = s.clock.Now()

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	s.notify(ctx, "updated", current, nil)

	view, err := s.view(ctx, current)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// SetStatus is the only path to a lifecycle move.
func (s *Service) SetStatus(ctx context.Context, in SetStatusInput) (SetStatusOutput, error) {
	current, err := s.load(ctx, in.ID)
	if err != nil {
		return SetStatusOutput{}, err
	}
	next := Status(strings.TrimSpace(string(in.Status)))
	if !next.Valid() {
		return SetStatusOutput{}, errInvalidStatus(string(next))
	}
	if current.Status == next {
		view, err := s.view(ctx, current)
		if err != nil {
			return SetStatusOutput{}, err
		}
		return SetStatusOutput{Task: &view, From: next, To: next}, nil
	}
	if !current.Status.CanMoveTo(next) {
		return SetStatusOutput{}, errInvalidTransition(in.ID, current.Status, next)
	}

	switch next {
	case InReview:
		if err := s.guardReview(ctx, current); err != nil {
			return SetStatusOutput{}, err
		}
	case InProgress:
		if err := s.guardDependencies(ctx, current); err != nil {
			return SetStatusOutput{}, err
		}
	}

	from := current.Status
	current.Status = next
	current.UpdatedAt = s.clock.Now()

	switch next {
	case Stopped:
		checkpoint, err := s.checkpoint(ctx, current, in.Reason)
		if err != nil {
			return SetStatusOutput{}, err
		}
		current.Checkpoint = checkpoint
	case InProgress:
		// Resuming consumes the checkpoint: leaving it would make the next stop
		// look like it happened at the point of the previous one.
		current.Checkpoint = nil
	}

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return SetStatusOutput{}, errWriteFailed("SetStatus", err)
	}
	s.notify(ctx, "status_changed", current, map[string]any{
		"from": string(from), "to": string(next), "type": current.Type,
	})

	view, err := s.view(ctx, current)
	if err != nil {
		return SetStatusOutput{}, err
	}
	return SetStatusOutput{Task: &view, From: from, To: next}, nil
}

// guardReview is the master prompt's hardest task rule, enforced: a task with
// open steps does not reach review. The original teaches it and hopes; here a
// completion claim without the plan behind it fails.
func (s *Service) guardReview(ctx context.Context, t *Task) error {
	if s.plan == nil {
		return nil
	}
	pending, err := s.plan.CountPending(ctx, t.ID)
	if err != nil {
		return errReadFailed("guardReview", err)
	}
	if pending > 0 {
		ids, _ := s.plan.PendingIDs(ctx, t.ID)
		return errReviewBlocked(t.ID, pending, ids)
	}
	return nil
}

// guardDependencies refuses to start work whose prerequisites are not done.
func (s *Service) guardDependencies(ctx context.Context, t *Task) error {
	blocked, err := s.blockedBy(ctx, t)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return errDependenciesPending(t.ID, blocked)
	}
	return nil
}

// blockedBy lists the dependencies that are not finished.
func (s *Service) blockedBy(ctx context.Context, t *Task) ([]string, error) {
	var blocked []string
	for _, id := range t.DependsOn {
		dep, err := s.repo.Get(ctx, collections.Key{"id": strings.TrimSpace(id)})
		if err != nil {
			// A dependency that no longer exists blocks nothing. It is worth a
			// line in the log: the reference is dangling and somebody should
			// clean it up, but refusing to start the work is the wrong penalty.
			s.log.Warn("a task depends on one that no longer exists",
				"task", t.ID, "dependency", id)
			continue
		}
		if !dep.Status.Terminal() {
			blocked = append(blocked, dep.ID)
		}
	}
	return blocked, nil
}

// checkpoint captures where a stopped run was, so resuming starts there.
func (s *Service) checkpoint(ctx context.Context, t *Task, reason string) (*Checkpoint, error) {
	c := &Checkpoint{ChatID: t.Chat, StoppedAt: s.clock.Now(), Reason: reason}
	if t.Checkpoint != nil && t.Checkpoint.JobID != "" {
		c.JobID = t.Checkpoint.JobID
	}
	if s.plan == nil {
		return c, nil
	}
	pending, err := s.plan.PendingIDs(ctx, t.ID)
	if err != nil {
		return nil, errReadFailed("checkpoint", err)
	}
	progress, err := s.plan.Progress(ctx, t.ID)
	if err != nil {
		return nil, errReadFailed("checkpoint", err)
	}
	c.PendingTodoIDs = pending
	c.Progress = progress
	return c, nil
}

// Delete removes the task and everything under it.
//
// The whole directory goes: todos, comments and runs. That is the collection
// engine's cascade, declared once in the registry, rather than a hook that
// swallows its errors like the original's.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	current, err := s.load(ctx, in.ID)
	if err != nil {
		return DeleteOutput{}, err
	}
	if current.Worktree.Path != "" && s.worktrees != nil {
		if err := s.worktrees.Remove(ctx, current.Worktree.Path); err != nil {
			// The checkout is outside the task directory, so removing the task
			// cannot take it with it. Reported rather than hidden: a leftover
			// worktree holds a branch and a copy of the repository.
			s.log.Error("the task was deleted but its worktree could not be removed",
				"task", current.ID, "path", current.Worktree.Path, "err", err)
		}
	}
	if err := s.repo.Delete(ctx, collections.Key{"id": current.ID}); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	s.notify(ctx, "deleted", current, nil)
	return DeleteOutput{ID: current.ID}, nil
}

// view builds the projections a reader needs and the file does not hold.
func (s *Service) view(ctx context.Context, t *Task) (View, error) {
	out := View{Task: *t, Assignee: ResolvedAssignee{ID: t.Assigned, Type: AssigneeUnknown}}
	if t.Assigned != "" && s.directory != nil {
		resolved, err := s.directory.Resolve(ctx, t.Assigned)
		if err == nil {
			out.Assignee = resolved
		}
	}
	if s.plan != nil {
		progress, err := s.plan.Progress(ctx, t.ID)
		if err != nil {
			return View{}, errReadFailed("view", err)
		}
		out.Progress = progress
	}
	blocked, err := s.blockedBy(ctx, t)
	if err != nil {
		return View{}, err
	}
	out.Blocked = blocked
	return out, nil
}

func (s *Service) load(ctx context.Context, id string) (*Task, error) {
	trimmed := strings.TrimSpace(id)
	current, err := s.repo.Get(ctx, collections.Key{"id": trimmed})
	if err != nil {
		return nil, errNotFound(trimmed)
	}
	return current, nil
}

func (s *Service) checkType(ctx context.Context, kind string) error {
	if kind == "" || s.policy == nil {
		return nil
	}
	known, err := s.policy.TaskTypes(ctx)
	if err != nil || len(known) == 0 {
		return nil
	}
	for _, k := range known {
		if k == kind {
			return nil
		}
	}
	return errUnknownType(kind, known)
}

// checkDependencies refuses a cycle and a self-reference. Without this a task
// can be made permanently unstartable by two calls that each look reasonable.
func (s *Service) checkDependencies(ctx context.Context, id string, deps []string) error {
	for _, dep := range deps {
		if strings.TrimSpace(dep) == id {
			return errSelfDependency(id)
		}
	}
	seen := map[string]bool{id: true}
	queue := append([]string(nil), deps...)
	for len(queue) > 0 {
		next := strings.TrimSpace(queue[0])
		queue = queue[1:]
		if next == "" || seen[next] {
			continue
		}
		seen[next] = true
		dep, err := s.repo.Get(ctx, collections.Key{"id": next})
		if err != nil {
			continue
		}
		for _, further := range dep.DependsOn {
			if strings.TrimSpace(further) == id {
				return errDependencyCycle(id, dep.ID)
			}
			queue = append(queue, further)
		}
	}
	return nil
}

func (s *Service) notify(ctx context.Context, event string, t *Task, data map[string]any) {
	if s.notifier == nil {
		return
	}
	s.notifier.TaskChanged(ctx, event, t, data)
}

// isEntryPoint reports whether a task may be created directly in this status.
func isEntryPoint(s Status) bool {
	switch s {
	case Suggestion, Backlog, Planning, Todo:
		return true
	default:
		return false
	}
}
