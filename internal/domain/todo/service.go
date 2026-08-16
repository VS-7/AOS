package todo

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
)

// Service is the todo aggregate.
type Service struct {
	repo   Repository
	parent Parent
	clock  Clock
	ids    IDs
	log    *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo   Repository
	Parent Parent
	Clock  Clock
	IDs    IDs
	Log    *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: d.Repo, parent: d.Parent, clock: d.Clock, ids: d.IDs, log: log}
}

// List returns the plan of one task, in order.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	taskID := strings.TrimSpace(in.Task)
	if taskID == "" {
		return ListOutput{}, errTaskRequired("List")
	}
	steps, err := s.all(ctx, taskID)
	if err != nil {
		return ListOutput{}, err
	}
	return ListOutput{Todos: steps, Total: len(steps), Progress: progressOf(steps)}, nil
}

// Get reads one step.
func (s *Service) Get(ctx context.Context, in GetInput) (*Todo, error) {
	taskID := strings.TrimSpace(in.Task)
	if taskID == "" {
		return nil, errTaskRequired("Get")
	}
	found, err := s.repo.Get(ctx, collections.Key{"taskId": taskID, "id": strings.TrimSpace(in.ID)})
	if err != nil {
		return nil, errNotFound(taskID, in.ID)
	}
	return found, nil
}

// Create adds a step to the plan.
//
// Order is assigned when it is not given, so a plan written one call at a time
// keeps the order it was written in rather than collapsing to a pile of zeroes.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Todo, error) {
	taskID := strings.TrimSpace(in.Task)
	if taskID == "" {
		return nil, errTaskRequired("Create")
	}
	if err := s.requireParent(ctx, taskID); err != nil {
		return nil, err
	}

	order := in.Order
	if order == 0 {
		existing, err := s.all(ctx, taskID)
		if err != nil {
			return nil, err
		}
		order = len(existing) + 1
	}

	now := s.clock.Now()
	t := &Todo{
		TaskID:    taskID,
		ID:        s.ids.New(),
		Title:     strings.TrimSpace(in.Title),
		Status:    Pending,
		Order:     order,
		CreatedAt: now,
		UpdatedAt: now,
		Content:   strings.TrimLeft(in.Content, " \t\n\r"),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return t, nil
}

// Update changes the describable parts of a step.
//
// Status is not one of them. That is the same rule the task aggregate enforces
// one level up: a lifecycle move is an operation with guards, and letting it be
// a field write would route around every one of them.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Todo, error) {
	current, err := s.Get(ctx, GetInput{Task: in.Task, ID: in.ID})
	if err != nil {
		return nil, err
	}
	if in.Status != "" {
		return nil, errStatusIsNotAField(in.ID)
	}

	if in.Title != nil {
		current.Title = strings.TrimSpace(*in.Title)
	}
	if in.Order != nil {
		current.Order = *in.Order
	}
	if in.Evidence != nil {
		current.Evidence = *in.Evidence
	}
	if in.Content != nil {
		current.Content = strings.TrimLeft(*in.Content, " \t\n\r")
	}
	current.UpdatedAt = s.clock.Now()

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// SetStatus is the only path to a lifecycle move.
//
// Finishing without evidence is a warning rather than a refusal. Not every step
// is verifiable — "decide which approach to take" has an outcome and no test —
// and a system that refuses to record an honest step teaches people to write
// dishonest evidence.
func (s *Service) SetStatus(ctx context.Context, in SetStatusInput) (SetStatusOutput, error) {
	current, err := s.Get(ctx, GetInput{Task: in.Task, ID: in.ID})
	if err != nil {
		return SetStatusOutput{}, err
	}
	next := Status(strings.TrimSpace(string(in.Status)))
	if !next.Valid() {
		return SetStatusOutput{}, errInvalidStatus(string(next))
	}
	if current.Status == next {
		return SetStatusOutput{Todo: current, From: next, To: next}, nil
	}
	if !current.Status.CanMoveTo(next) {
		return SetStatusOutput{}, errInvalidTransition(in.ID, current.Status, next)
	}

	from := current.Status
	current.Status = next
	if in.Evidence != "" {
		current.Evidence = in.Evidence
	}
	current.UpdatedAt = s.clock.Now()

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return SetStatusOutput{}, errWriteFailed("SetStatus", err)
	}

	out := SetStatusOutput{Todo: current, From: from, To: next}
	if next == Finished && strings.TrimSpace(current.Evidence) == "" {
		out.Warning = "this step is finished with no evidence recorded; the task's review guard counts it as done, and nobody reading the plan later will know what was checked"
	}
	return out, nil
}

// Delete removes one step.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	if _, err := s.Get(ctx, GetInput{Task: in.Task, ID: in.ID}); err != nil {
		return DeleteOutput{}, err
	}
	key := collections.Key{"taskId": strings.TrimSpace(in.Task), "id": strings.TrimSpace(in.ID)}
	if err := s.repo.Delete(ctx, key); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: in.ID, Task: in.Task}, nil
}

// CountPending reports how many steps are still open. It is what the task's
// review guard calls, and the reason this aggregate exists as more than storage.
func (s *Service) CountPending(ctx context.Context, taskID string) (int, error) {
	steps, err := s.all(ctx, taskID)
	if err != nil {
		return 0, err
	}
	return progressOf(steps).Pending(), nil
}

// PendingIDs lists the open steps, which is what a task checkpoint records so a
// resumed run knows exactly where it stopped.
func (s *Service) PendingIDs(ctx context.Context, taskID string) ([]string, error) {
	steps, err := s.all(ctx, taskID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, t := range steps {
		if !t.Status.Settled() {
			out = append(out, t.ID)
		}
	}
	return out, nil
}

// Progress reports the plan's completion.
func (s *Service) Progress(ctx context.Context, taskID string) (Progress, error) {
	steps, err := s.all(ctx, taskID)
	if err != nil {
		return Progress{}, err
	}
	return progressOf(steps), nil
}

// all reads one task's steps in plan order. Ties break by identifier, so the
// order is total even when a caller assigns the same number twice.
func (s *Service) all(ctx context.Context, taskID string) ([]Todo, error) {
	found, err := s.repo.List(ctx, collections.Query{
		Key:            collections.Key{"taskId": strings.TrimSpace(taskID)},
		IncludeContent: true,
	})
	if err != nil {
		return nil, errReadFailed("List", err)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Order == found[j].Order {
			return found[i].ID < found[j].ID
		}
		return found[i].Order < found[j].Order
	})
	return found, nil
}

func (s *Service) requireParent(ctx context.Context, taskID string) error {
	if s.parent == nil {
		return nil
	}
	exists, err := s.parent.Exists(ctx, taskID)
	if err != nil {
		return errReadFailed("Create", err)
	}
	if !exists {
		return errParentMissing(taskID)
	}
	return nil
}

func progressOf(steps []Todo) Progress {
	p := Progress{Total: len(steps)}
	for _, t := range steps {
		if t.Status.Settled() {
			p.Completed++
		}
	}
	return p
}
