package activity

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/safe"
)

// DefaultRetention is how long the log keeps an entry.
//
// The original has no retention at all — it has an error code for an invalid
// purge age and no policy that would ever raise it. A log that never expires
// eventually dominates the repository it lives in.
const DefaultRetention = 90 * 24 * time.Hour

// DefaultLimit is one page of the inbox.
const DefaultLimit = 50

// Service is the activity aggregate.
type Service struct {
	log       Log
	read      ReadStore
	sinks     []Sink
	clock     Clock
	ids       IDs
	logger    *slog.Logger
	retention time.Duration
}

// Deps is what the service is built from.
type Deps struct {
	Log   Log
	Read  ReadStore
	Clock Clock
	IDs   IDs

	// Sinks receive every published activity. Empty is normal: the log is the
	// product, the fan-out is the convenience.
	Sinks []Sink

	// Retention overrides DefaultRetention.
	Retention time.Duration

	Logger *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	retention := d.Retention
	if retention <= 0 {
		retention = DefaultRetention
	}
	return &Service{
		log: d.Log, read: d.Read, sinks: d.Sinks,
		clock: d.Clock, ids: d.IDs, logger: logger, retention: retention,
	}
}

// AddSink registers a consumer after construction.
//
// The routine evaluator needs it: routines react to activities, and publishing
// an activity is how a routine's own run gets recorded, so one of the two has
// to be attached to the other once both exist.
func (s *Service) AddSink(sink Sink) {
	if sink != nil {
		s.sinks = append(s.sinks, sink)
	}
}

// Publish records the activity and fans it out.
//
// The order is the contract: the log is written first and its failure is the
// only one the caller hears about. Realtime and the trigger evaluator run after
// and cannot fail the mutation that produced the event — a routine that panics
// must not roll back the task whose status changed.
func (s *Service) Publish(ctx context.Context, in PublishInput) (*Activity, error) {
	namespace := strings.TrimSpace(in.Namespace)
	event := strings.TrimSpace(in.Event)
	if namespace == "" || event == "" {
		return nil, errIncomplete(namespace, event)
	}

	actor, kind := identity.Actor(ctx)
	if actor == "" {
		actor, kind = ActorSystem, ActorSystem
	}

	a := Activity{
		ID:        s.ids.New(),
		Namespace: namespace,
		Event:     event,
		Title:     in.Title,
		Body:      in.Body,
		Icon:      in.Icon,
		Data:      in.Data,
		Actor:     actor,
		ActorType: string(kind),
		CreatedAt: s.clock.Now(),
	}
	if err := s.log.Append(ctx, a); err != nil {
		return nil, errWriteFailed("Publish", err)
	}

	for _, sink := range s.sinks {
		s.deliver(ctx, sink, a)
	}
	return &a, nil
}

// deliver runs one sink under recovery, so a consumer that panics costs its own
// notification and nothing else.
func (s *Service) deliver(ctx context.Context, sink Sink, a Activity) {
	err := safe.Do(ctx, "activity.sink", func(ctx context.Context) error {
		sink.OnActivity(ctx, a)
		return nil
	})
	if err != nil {
		s.logger.Error("an activity consumer failed",
			"namespace", a.Namespace, "event", a.Event, "err", err)
	}
}

// List reads the inbox, newest first.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	since := time.Time{}
	if in.Since != "" {
		parsed, err := time.Parse(time.RFC3339, in.Since)
		if err != nil {
			return ListOutput{}, errInvalidTime("since", in.Since, err)
		}
		since = parsed
	}

	entries, err := s.log.Load(ctx, since)
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}
	state, err := s.readState(ctx)
	if err != nil {
		return ListOutput{}, err
	}
	viewer := s.viewer(ctx, in.Actor)

	matched := make([]Activity, 0, len(entries))
	unread := 0
	for _, a := range entries {
		if in.Namespace != "" && !strings.EqualFold(in.Namespace, a.Namespace) {
			continue
		}
		if in.Event != "" && !strings.EqualFold(in.Event, a.Event) {
			continue
		}
		seen := state.IsRead(viewer, a)
		if !seen {
			unread++
		}
		if in.Unread && seen {
			continue
		}
		matched = append(matched, a)
	}

	// Newest first: an inbox is read from the top.
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID > matched[j].ID
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
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit < len(matched) {
		matched = matched[:limit]
	}

	return ListOutput{Activities: matched, Total: total, Unread: unread, Actor: viewer}, nil
}

// Get reads one entry.
func (s *Service) Get(ctx context.Context, in GetInput) (*Activity, error) {
	entries, err := s.log.Load(ctx, time.Time{})
	if err != nil {
		return nil, errReadFailed("Get", err)
	}
	for i := range entries {
		if entries[i].ID == in.ID {
			return &entries[i], nil
		}
	}
	return nil, errNotFound(in.ID)
}

// MarkAsRead records that the current actor has seen one entry.
func (s *Service) MarkAsRead(ctx context.Context, in MarkInput) (MarkOutput, error) {
	found, err := s.Get(ctx, GetInput{ID: in.ID})
	if err != nil {
		return MarkOutput{}, err
	}
	state, err := s.readState(ctx)
	if err != nil {
		return MarkOutput{}, err
	}
	viewer := s.viewer(ctx, in.Actor)
	if !state.Mark(viewer, *found) {
		return MarkOutput{Actor: viewer, Changed: false}, nil
	}
	if err := s.read.Save(ctx, state); err != nil {
		return MarkOutput{}, errWriteFailed("MarkAsRead", err)
	}
	return MarkOutput{Actor: viewer, Changed: true}, nil
}

// MarkAllAsRead moves the actor's watermark to now.
func (s *Service) MarkAllAsRead(ctx context.Context, in MarkAllInput) (MarkOutput, error) {
	state, err := s.readState(ctx)
	if err != nil {
		return MarkOutput{}, err
	}
	viewer := s.viewer(ctx, in.Actor)
	state.MarkAll(viewer, s.clock.Now())
	if err := s.read.Save(ctx, state); err != nil {
		return MarkOutput{}, errWriteFailed("MarkAllAsRead", err)
	}
	return MarkOutput{Actor: viewer, Changed: true}, nil
}

// Purge drops entries older than the retention window.
//
// Whole partitions go first, which is the cheap path and the usual one. A month
// that is only partly expired is rewritten, and that is the one case where this
// package edits history — reported in the output so it is never silent.
func (s *Service) Purge(ctx context.Context, in PurgeInput) (PurgeOutput, error) {
	window := s.retention
	if in.OlderThanDays > 0 {
		window = time.Duration(in.OlderThanDays) * 24 * time.Hour
	}
	cutoff := s.clock.Now().Add(-window)

	months, err := s.log.Months(ctx)
	if err != nil {
		return PurgeOutput{}, errReadFailed("Purge", err)
	}
	entries, err := s.log.Load(ctx, time.Time{})
	if err != nil {
		return PurgeOutput{}, errReadFailed("Purge", err)
	}

	keep := map[string][]Activity{}
	removed := map[string]int{}
	for _, a := range entries {
		month := a.Month()
		if a.CreatedAt.Before(cutoff) {
			removed[month]++
			continue
		}
		keep[month] = append(keep[month], a)
	}

	out := PurgeOutput{Cutoff: cutoff}
	for _, month := range months {
		if removed[month] == 0 {
			continue
		}
		if err := s.log.Rewrite(ctx, month, keep[month]); err != nil {
			return out, errWriteFailed("Purge", err)
		}
		out.Removed += removed[month]
		if len(keep[month]) == 0 {
			out.Dropped = append(out.Dropped, month)
		} else {
			out.Rewritten = append(out.Rewritten, month)
		}
	}
	return out, nil
}

// Delete removes one entry, rewriting the month it lived in.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	entries, err := s.log.Load(ctx, time.Time{})
	if err != nil {
		return DeleteOutput{}, errReadFailed("Delete", err)
	}
	var target *Activity
	for i := range entries {
		if entries[i].ID == in.ID {
			target = &entries[i]
			break
		}
	}
	if target == nil {
		return DeleteOutput{}, errNotFound(in.ID)
	}

	month := target.Month()
	keep := make([]Activity, 0, len(entries))
	for _, a := range entries {
		if a.Month() == month && a.ID != in.ID {
			keep = append(keep, a)
		}
	}
	if err := s.log.Rewrite(ctx, month, keep); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: in.ID, Month: month}, nil
}

func (s *Service) readState(ctx context.Context) (ReadState, error) {
	if s.read == nil {
		return ReadState{}, nil
	}
	state, err := s.read.Load(ctx)
	if err != nil {
		return ReadState{}, errReadFailed("readState", err)
	}
	return state, nil
}

// viewer decides whose inbox is being read: the actor named, otherwise the
// ambient identity, otherwise the one shared inbox a bare terminal sees.
func (s *Service) viewer(ctx context.Context, explicit string) string {
	if named := strings.TrimSpace(explicit); named != "" {
		return named
	}
	if actor, _ := identity.Actor(ctx); actor != "" {
		return actor
	}
	return ActorSystem
}

// Events answers the catalogue of event kinds a routine can trigger on.
//
// It reads no storage: the catalogue is a declaration, and a workspace where
// nothing has happened yet must still be able to say what could.
func (s *Service) Events(_ context.Context, in EventsInput) (EventsOutput, error) {
	if in.Namespace == "" {
		return EventsOutput{Events: Kinds, Namespaces: Namespaces()}, nil
	}

	out := EventsOutput{Events: make([]EventKind, 0, 8)}
	for _, kind := range Kinds {
		if !strings.EqualFold(kind.Namespace, in.Namespace) {
			continue
		}
		out.Events = append(out.Events, kind)
	}
	if len(out.Events) > 0 {
		out.Namespaces = []string{out.Events[0].Namespace}
	}
	return out, nil
}
