package job

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// Service is the read and repair surface over the queue.
//
// It deliberately cannot enqueue. Work is scheduled by the aggregate that owns
// it — a turn, a task, a routine — and a surface that let anything put anything
// on the queue would make the queue's contents unattributable.
type Service struct {
	queue Queue
	clock Clock
	log   *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Queue Queue
	Clock Clock
	Log   *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{queue: d.Queue, clock: d.Clock, log: log}
}

// List reads the queue.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	if s.queue == nil {
		return ListOutput{}, errNoQueue("List")
	}
	if in.Status != "" && !in.Status.Valid() {
		return ListOutput{}, errInvalidStatus(string(in.Status))
	}
	found, err := s.queue.List(ctx, Filter{
		Queue: in.Queue, Status: in.Status, Workspace: in.Workspace,
		Kind: in.Kind, Limit: in.Limit,
	})
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}
	return ListOutput{Jobs: found, Total: len(found)}, nil
}

// Get reads one job.
func (s *Service) Get(ctx context.Context, in GetInput) (*Job, error) {
	if s.queue == nil {
		return nil, errNoQueue("Get")
	}
	found, err := s.queue.Get(ctx, strings.TrimSpace(in.ID))
	if err != nil || found == nil {
		return nil, errNotFound(in.ID)
	}
	return found, nil
}

// Stats counts the queue by state, which is the one question an operator asks
// first: is anything stuck, and how much.
func (s *Service) Stats(ctx context.Context, in StatsInput) (StatsOutput, error) {
	if s.queue == nil {
		return StatsOutput{}, errNoQueue("Stats")
	}
	found, err := s.queue.List(ctx, Filter{Workspace: in.Workspace})
	if err != nil {
		return StatsOutput{}, errReadFailed("Stats", err)
	}

	out := StatsOutput{
		ByStatus: map[string]int{},
		ByQueue:  map[string]int{},
		Total:    len(found),
		At:       s.clock.Now(),
	}
	for _, j := range found {
		out.ByStatus[string(j.Status)]++
		out.ByQueue[j.Queue]++
		if j.Status == Dead {
			out.Dead = append(out.Dead, j.ID)
		}
		if j.Status == Claimed && j.LeaseUntil != nil && j.LeaseUntil.Before(out.At) {
			// A claimed job whose lease has lapsed is one whose worker died.
			// It is separated from the healthy claims because it is the shape
			// of a real incident, not of a busy queue.
			out.Stale = append(out.Stale, j.ID)
		}
	}
	sort.Strings(out.Dead)
	sort.Strings(out.Stale)
	return out, nil
}

// Recover hands back every job whose lease lapsed.
func (s *Service) Recover(ctx context.Context, _ RecoverInput) (RecoverOutput, error) {
	if s.queue == nil {
		return RecoverOutput{}, errNoQueue("Recover")
	}
	n, err := s.queue.RecoverStale(ctx)
	if err != nil {
		return RecoverOutput{}, errWriteFailed("Recover", err)
	}
	if n > 0 {
		s.log.Warn("jobs were handed back after their worker stopped reporting", "count", n)
	}
	return RecoverOutput{Recovered: n}, nil
}

// Purge removes terminal jobs older than the window.
func (s *Service) Purge(ctx context.Context, in PurgeInput) (PurgeOutput, error) {
	if s.queue == nil {
		return PurgeOutput{}, errNoQueue("Purge")
	}
	window := 7 * 24 * time.Hour
	if in.OlderThanDays > 0 {
		window = time.Duration(in.OlderThanDays) * 24 * time.Hour
	}
	n, err := s.queue.Purge(ctx, window)
	if err != nil {
		return PurgeOutput{}, errWriteFailed("Purge", err)
	}
	return PurgeOutput{Removed: n, OlderThan: window.String()}, nil
}
