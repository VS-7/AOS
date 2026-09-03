package routine

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/safe"
)

// Service is the routine aggregate.
type Service struct {
	repo      Repository
	runs      Runs
	executor  Executor
	tokens    Tokens
	directory Directory
	notifier  Notifier
	clock     Clock
	ids       IDs
	tick      time.Duration
	log       *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo  Repository
	Runs  Runs
	Clock Clock
	IDs   IDs

	Executor  Executor
	Tokens    Tokens
	Directory Directory
	Notifier  Notifier

	// Tick is the scheduler's window. It is carried here because the effective
	// resolution is reported to the user with the cron they declared, and the
	// two have to come from the same number.
	Tick time.Duration

	Log *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	tick := d.Tick
	if tick <= 0 {
		tick = DefaultTick
	}
	return &Service{
		repo: d.Repo, runs: d.Runs, executor: d.Executor, tokens: d.Tokens,
		directory: d.Directory, notifier: d.Notifier,
		clock: d.Clock, ids: d.IDs, tick: tick, log: log,
	}
}

// SetExecutor binds the runtime after both exist.
//
// A routine's run is a real turn, and a turn writes back to the conversation
// the runtime owns — so the runtime cannot be built before this aggregate and
// this aggregate cannot be built before it. The pointer is set once at boot,
// before any request can arrive.
func (s *Service) SetExecutor(e Executor) { s.executor = e }

// List returns the routines, with the effective resolution of each schedule.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	q := collections.Query{IncludeContent: false}
	if agent := s.resolveAgent(ctx, in.Agent); agent != "" {
		q.Key = collections.Key{"agent": agent}
	}
	found, err := s.repo.List(ctx, q)
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}

	needle := strings.ToLower(strings.TrimSpace(in.Query))
	views := make([]View, 0, len(found))
	for i := range found {
		if in.Status != "" && found[i].Status != in.Status {
			continue
		}
		// Name and id, which is what the list shows. A routine's prompt is
		// its body and is not loaded here (IncludeContent is false), so
		// searching it would mean reading every file to answer a keystroke.
		if needle != "" &&
			!strings.Contains(strings.ToLower(found[i].Name), needle) &&
			!strings.Contains(strings.ToLower(found[i].ID), needle) {
			continue
		}
		views = append(views, s.view(&found[i]))
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].Agent == views[j].Agent {
			return views[i].ID < views[j].ID
		}
		return views[i].Agent < views[j].Agent
	})
	// Total counts what matched, before the page was cut — the same contract
	// task.ListOutput's own Total documents.
	total := len(views)
	if in.Limit > 0 && len(views) > in.Limit {
		views = views[:in.Limit]
	}
	return ListOutput{Routines: views, Total: total, Tick: s.tick.String()}, nil
}

// Get reads one routine.
func (s *Service) Get(ctx context.Context, in GetInput) (*View, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return nil, err
	}
	view := s.view(current)
	return &view, nil
}

// Create records a routine and, for a webhook trigger, mints its token.
//
// The token is returned once and never again: the file holds only a hash. The
// original writes the token in clear into front matter that is committed to
// Git, which puts a live credential in the repository's history.
func (s *Service) Create(ctx context.Context, in CreateInput) (CreateOutput, error) {
	agent := s.resolveAgent(ctx, in.Agent)
	if agent == "" {
		return CreateOutput{}, errAgentRequired("Create")
	}
	if s.directory != nil && !s.directory.IsAgent(ctx, agent) {
		return CreateOutput{}, errNoSuchAgent(agent)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CreateOutput{}, errInvalidName(in.Name)
	}

	status := in.Status
	if status == "" {
		status = Enabled
	}
	if !status.Valid() {
		return CreateOutput{}, errInvalidStatus(string(status))
	}

	triggers, token, err := s.buildTriggers(in.Triggers)
	if err != nil {
		return CreateOutput{}, err
	}

	now := s.clock.Now()
	r := &Routine{
		Agent:     agent,
		ID:        s.ids.New(),
		Name:      name,
		Triggers:  triggers,
		Status:    status,
		Scope:     in.Scope,
		CreatedAt: now,
		UpdatedAt: now,
		Content:   strings.TrimLeft(in.Content, " \t\n\r"),
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return CreateOutput{}, errWriteFailed("Create", err)
	}
	return CreateOutput{Routine: s.view(r), Token: token}, nil
}

// Update changes a routine.
func (s *Service) Update(ctx context.Context, in UpdateInput) (CreateOutput, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return CreateOutput{}, err
	}

	var token string
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return CreateOutput{}, errInvalidName(*in.Name)
		}
		current.Name = name
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return CreateOutput{}, errInvalidStatus(string(*in.Status))
		}
		current.Status = *in.Status
	}
	if in.Scope != nil {
		current.Scope = *in.Scope
	}
	if in.Content != nil {
		current.Content = strings.TrimLeft(*in.Content, " \t\n\r")
	}
	if in.Triggers != nil {
		built, minted, err := s.buildTriggers(*in.Triggers)
		if err != nil {
			return CreateOutput{}, err
		}
		current.Triggers, token = built, minted
	}
	current.UpdatedAt = s.clock.Now()

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return CreateOutput{}, errWriteFailed("Update", err)
	}
	return CreateOutput{Routine: s.view(current), Token: token}, nil
}

// Rotate mints a new webhook token, invalidating the old one.
func (s *Service) Rotate(ctx context.Context, in RotateInput) (RotateOutput, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return RotateOutput{}, err
	}
	if s.tokens == nil {
		return RotateOutput{}, errTokensUnavailable()
	}

	for i := range current.Triggers {
		if current.Triggers[i].Type != Webhook {
			continue
		}
		token, hash, err := s.tokens.New()
		if err != nil {
			return RotateOutput{}, errTokenMint(err)
		}
		current.Triggers[i].Config.TokenHash = hash
		current.UpdatedAt = s.clock.Now()
		if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
			return RotateOutput{}, errWriteFailed("Rotate", err)
		}
		return RotateOutput{Token: token}, nil
	}
	return RotateOutput{}, errNoWebhookTrigger(current.ID)
}

// Delete removes a routine with its run history.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return DeleteOutput{}, err
	}
	key := collections.Key{"agent": current.Agent, "id": current.ID}
	if err := s.repo.Delete(ctx, key); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: current.ID, Agent: current.Agent}, nil
}

// Fire runs a routine now, recording a run whichever way it ends.
func (s *Service) Fire(ctx context.Context, in FireInput) (*Run, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return nil, err
	}
	trigger := in.Trigger
	if trigger == "" {
		trigger = TriggerType("manual")
	}
	return s.fire(ctx, current, trigger, in.Payload, in.Force)
}

// FireWebhook authenticates a token and fires the routine it belongs to.
func (s *Service) FireWebhook(ctx context.Context, in WebhookInput) (*Run, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return nil, err
	}
	if s.tokens == nil {
		return nil, errTokensUnavailable()
	}
	for _, t := range current.Triggers {
		if t.Type == Webhook && s.tokens.Verify(in.Token, t.Config.TokenHash) {
			return s.fire(ctx, current, Webhook, in.Payload, false)
		}
	}
	return nil, errInvalidToken(current.ID)
}

// OnActivity is the reactive half: an activity fires every enabled routine
// whose activity trigger matches it.
//
// It never returns an error to its caller. It is wired as a sink of the
// activity aggregate, and a routine that fails must not roll back the task
// whose status changed.
func (s *Service) OnActivity(ctx context.Context, namespace, event string, data map[string]any) {
	found, err := s.repo.List(ctx, collections.Query{IncludeContent: true})
	if err != nil {
		s.log.Error("could not read the routines to react to an activity",
			"namespace", namespace, "event", event, "err", err)
		return
	}
	for i := range found {
		r := &found[i]
		if r.Status != Enabled {
			continue
		}
		for _, t := range r.Triggers {
			if !t.Matches(namespace, event, data) {
				continue
			}
			payload := map[string]any{"namespace": namespace, "event": event, "data": data}
			if _, err := s.fire(ctx, r, Activity, payload, false); err != nil {
				s.log.Error("a routine that reacted to an activity failed",
					"routine", r.ID, "agent", r.Agent, "err", err)
			}
			break // one firing per activity, however many triggers match
		}
	}
}

// ProcessScheduled is the queue job: it evaluates cron triggers against the
// current tick window and fires the ones that are due.
func (s *Service) ProcessScheduled(ctx context.Context, now time.Time) (ScheduleOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{IncludeContent: true})
	if err != nil {
		return ScheduleOutput{}, errReadFailed("ProcessScheduled", err)
	}

	out := ScheduleOutput{Window: s.tick.String(), At: now}
	for i := range found {
		r := &found[i]
		if r.Status != Enabled {
			continue
		}
		for _, t := range r.Triggers {
			if t.Type != Scheduled {
				continue
			}
			schedule, err := Parse(t.Config.Cron)
			if err != nil {
				// A cron that does not parse is a routine that will never fire.
				// It is worth saying so on every tick rather than once, because
				// the alternative is silence for as long as it stays broken.
				s.log.Warn("a routine has a cron expression that does not parse",
					"routine", r.ID, "agent", r.Agent, "cron", t.Config.Cron, "err", err)
				out.Broken = append(out.Broken, r.ID)
				continue
			}
			var last time.Time
			if r.LastFiredAt != nil {
				last = *r.LastFiredAt
			}
			if !DueInWindow(schedule, last, now, s.tick) {
				continue
			}
			if _, err := s.fire(ctx, r, Scheduled, map[string]any{"cron": schedule.Expr()}, false); err != nil {
				s.log.Error("a scheduled routine failed", "routine", r.ID, "err", err)
				out.Failed = append(out.Failed, r.ID)
				continue
			}
			out.Fired = append(out.Fired, r.ID)
			break // one firing per tick, however many schedules are due
		}
	}
	return out, nil
}

// fire is the single place a run is recorded, so no trigger can fire without
// leaving one.
func (s *Service) fire(ctx context.Context, r *Routine, trigger TriggerType, payload map[string]any, force bool) (*Run, error) {
	if r.Status != Enabled && !force {
		run := s.newRun(r, trigger, payload)
		run.Status = RunSkipped
		run.Error = "the routine is disabled"
		s.finish(ctx, r, run)
		return run, errDisabled(r.ID)
	}

	run := s.newRun(r, trigger, payload)
	if s.runs != nil {
		if err := s.runs.Create(ctx, run); err != nil {
			return nil, errWriteFailed("fire", err)
		}
	}

	if s.executor == nil {
		run.Status = RunFailed
		run.Error = "this installation has no runtime to execute a routine"
		s.finish(ctx, r, run)
		return run, errNoExecutor(r.ID)
	}

	// The routine runs as the agent that owns it, not as whoever triggered it.
	// A webhook from the internet must not inherit the identity of the process
	// that received it.
	runCtx := identity.With(ctx, identity.Identity{
		AgentID:     r.Agent,
		WorkspaceID: identity.From(ctx).WorkspaceID,
		RequestID:   run.ID,
	})

	var outcome Outcome
	err := safe.Do(runCtx, "routine.execute", func(ctx context.Context) error {
		var execErr error
		outcome, execErr = s.executor.Execute(ctx, Execution{
			Agent: r.Agent, Routine: r.ID, RunID: run.ID,
			Trigger: trigger, Payload: payload,
			Prompt: r.Content, Scope: r.Scope,
		})
		return execErr
	})

	run.ChatID = outcome.ChatID
	run.Usage = outcome.Usage
	switch {
	case err == nil:
		run.Status = RunSucceeded
	case errors.Is(err, context.DeadlineExceeded):
		run.Status = RunTimedOut
		run.Error = err.Error()
	default:
		run.Status = RunFailed
		run.Error = err.Error()
	}
	s.finish(ctx, r, run)

	if err != nil {
		return run, errRunFailed(r.ID, err)
	}
	return run, nil
}

func (s *Service) newRun(r *Routine, trigger TriggerType, payload map[string]any) *Run {
	return &Run{
		Agent: r.Agent, Routine: r.ID, ID: s.ids.New(),
		Trigger: trigger, Payload: payload,
		Status: RunRunning, StartedAt: s.clock.Now(),
	}
}

// finish closes the audit record and marks the routine as fired.
//
// Both writes are best-effort and logged: the work already happened, and losing
// the record of it is bad but refusing to return the result is worse.
func (s *Service) finish(ctx context.Context, r *Routine, run *Run) {
	ended := s.clock.Now()
	run.EndedAt = &ended

	if s.runs != nil {
		if err := s.runs.Update(ctx, run, collections.Version{}); err != nil {
			// Create may not have happened either, on the skipped path.
			if createErr := s.runs.Create(ctx, run); createErr != nil {
				s.log.Error("a routine ran and its audit record could not be written",
					"routine", r.ID, "run", run.ID, "err", err)
			}
		}
	}
	if run.Status != RunSkipped {
		r.LastFiredAt = &run.StartedAt
		r.UpdatedAt = ended
		if err := s.repo.Update(ctx, r, collections.Version{}); err != nil {
			s.log.Error("a routine fired and the firing was not recorded on it",
				"routine", r.ID, "err", err)
		}
	}
	if s.notifier != nil {
		s.notifier.RoutineFired(ctx, r, run)
	}
}

// Runs lists the audit history of one routine, newest first.
func (s *Service) Runs(ctx context.Context, in RunsInput) (RunsOutput, error) {
	current, err := s.load(ctx, in.Agent, in.ID)
	if err != nil {
		return RunsOutput{}, err
	}
	if s.runs == nil {
		return RunsOutput{Routine: current.ID}, nil
	}
	found, err := s.runs.List(ctx, collections.Query{
		Key: collections.Key{"agent": current.Agent, "routine": current.ID},
	})
	if err != nil {
		return RunsOutput{}, errReadFailed("Runs", err)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].StartedAt.Equal(found[j].StartedAt) {
			return found[i].ID > found[j].ID
		}
		return found[i].StartedAt.After(found[j].StartedAt)
	})
	total := len(found)
	if in.Limit > 0 && in.Limit < len(found) {
		found = found[:in.Limit]
	}
	return RunsOutput{Routine: current.ID, Runs: found, Total: total}, nil
}

// buildTriggers validates the union and mints a webhook token when one is
// declared. It returns the token exactly once, to be shown and then forgotten.
func (s *Service) buildTriggers(in []TriggerInput) ([]Trigger, string, error) {
	out := make([]Trigger, 0, len(in))
	var token string

	for _, t := range in {
		if !t.Type.Valid() {
			return nil, "", errUnknownTriggerType(string(t.Type))
		}
		built := Trigger{Type: t.Type}

		switch t.Type {
		case Scheduled:
			schedule, err := Parse(t.Cron)
			if err != nil {
				return nil, "", errInvalidCron(t.Cron, err)
			}
			built.Config.Cron = schedule.Expr()
			if len(t.Filters) > 0 {
				return nil, "", errFiltersNotApplicable(string(t.Type))
			}
		case Webhook:
			if s.tokens == nil {
				return nil, "", errTokensUnavailable()
			}
			minted, hash, err := s.tokens.New()
			if err != nil {
				return nil, "", errTokenMint(err)
			}
			built.Config.TokenHash = hash
			token = minted
			if len(t.Filters) > 0 {
				return nil, "", errFiltersNotApplicable(string(t.Type))
			}
		case Activity:
			if strings.TrimSpace(t.Namespace) == "" {
				return nil, "", errActivityNamespaceRequired()
			}
			built.Config.Namespace = strings.TrimSpace(t.Namespace)
			built.Config.Event = strings.TrimSpace(t.Event)
			for _, f := range t.Filters {
				if f.Operator != OpEq && f.Operator != OpNeq && f.Operator != OpContains {
					return nil, "", errUnknownOperator(f.Operator)
				}
				if strings.TrimSpace(f.Field) == "" {
					return nil, "", errFilterFieldRequired()
				}
			}
			built.Filters = t.Filters
		}
		out = append(out, built)
	}
	return out, token, nil
}

// view adds what the file does not hold: the effective resolution of each
// schedule, and when it will next fire.
//
// The original leaves a user to discover on their own that `* * * * *` does not
// run every minute. Saying it here is the whole of the divergence.
func (s *Service) view(r *Routine) View {
	out := View{Routine: *r, EffectiveInterval: s.tick.String()}
	for _, t := range r.Triggers {
		if t.Type != Scheduled {
			continue
		}
		schedule, err := Parse(t.Config.Cron)
		if err != nil {
			out.Warnings = append(out.Warnings,
				"the cron expression "+t.Config.Cron+" does not parse, so this routine will never fire on a schedule")
			continue
		}
		if next, ok := schedule.Next(s.clock.Now()); ok {
			out.NextRun = &next
		}
		if firesOftenerThan(schedule, s.tick) {
			out.Warnings = append(out.Warnings,
				"this cron would fire more often than the "+s.tick.String()+
					" scheduler tick, so it fires once per tick — that is the real resolution of the system")
		}
	}
	return out
}

// firesOftenerThan reports whether a schedule would fire more than once inside
// one tick window.
func firesOftenerThan(s *Schedule, tick time.Duration) bool {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first, ok := s.Next(from)
	if !ok {
		return false
	}
	second, ok := s.Next(first.Add(time.Minute))
	if !ok {
		return false
	}
	return second.Sub(first) < tick
}

func (s *Service) load(ctx context.Context, agent, id string) (*Routine, error) {
	owner := s.resolveAgent(ctx, agent)
	trimmed := strings.TrimSpace(id)

	if owner != "" {
		found, err := s.repo.Get(ctx, collections.Key{"agent": owner, "id": trimmed})
		if err == nil {
			return found, nil
		}
	}
	// Without an owner there is no key to build, so the routine is found by
	// scanning. This is the human-terminal path: a person listing routines does
	// not know which agent owns which.
	all, err := s.repo.List(ctx, collections.Query{IncludeContent: true})
	if err != nil {
		return nil, errReadFailed("load", err)
	}
	for i := range all {
		if all[i].ID == trimmed && (owner == "" || all[i].Agent == owner) {
			return &all[i], nil
		}
	}
	return nil, errNotFound(owner, trimmed)
}

func (s *Service) resolveAgent(ctx context.Context, explicit string) string {
	if named := strings.ToLower(strings.TrimSpace(explicit)); named != "" {
		return named
	}
	if actor, kind := identity.Actor(ctx); kind == identity.ActorAgent {
		return strings.ToLower(actor)
	}
	return ""
}
