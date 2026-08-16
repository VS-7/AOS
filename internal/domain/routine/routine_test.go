package routine

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/fakes"
)

type countingIDs struct {
	mu sync.Mutex
	n  int
}

func (g *countingIDs) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return "r" + strconv.Itoa(g.n)
}

// tokens is the credential port. The domain's rule is only that the file holds
// a hash and the token is shown once; the hashing itself lives in an adapter.
type tokens struct{ n int }

func (t *tokens) New() (string, string, error) {
	t.n++
	token := "tok-" + strconv.Itoa(t.n)
	return token, "hash:" + token, nil
}

func (t *tokens) Verify(token, hash string) bool { return hash == "hash:"+token }

type directory map[string]bool

func (d directory) IsAgent(_ context.Context, id string) bool { return d[id] }

// executor records what it was asked to run, including the identity it ran as.
type executor struct {
	mu       sync.Mutex
	calls    []Execution
	actors   []string
	failWith error
	panics   bool
	usage    Usage
}

func (e *executor) Execute(ctx context.Context, req Execution) (Outcome, error) {
	e.mu.Lock()
	e.calls = append(e.calls, req)
	actor, _ := identity.Actor(ctx)
	e.actors = append(e.actors, actor)
	e.mu.Unlock()

	if e.panics {
		panic("the routine's prompt blew up")
	}
	if e.failWith != nil {
		return Outcome{}, e.failWith
	}
	return Outcome{ChatID: "chat-" + req.RunID, Usage: e.usage}, nil
}

func (e *executor) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

var start = time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC) // a Monday

type harness struct {
	svc      *Service
	repo     *fakes.Repo[Routine]
	runs     *fakes.Repo[Run]
	executor *executor
	clock    *clockx.Stepping
}

func newHarness(t *testing.T, opts ...func(*Deps)) *harness {
	t.Helper()
	repo := fakes.NewRepo[Routine]("routines").WithKeyFunc(func(v *Routine) collections.Key {
		return collections.Key{"agent": v.Agent, "id": v.ID}
	})
	runs := fakes.NewRepo[Run]("runs").WithKeyFunc(func(v *Run) collections.Key {
		return collections.Key{"agent": v.Agent, "routine": v.Routine, "id": v.ID}
	})
	exec := &executor{}
	clock := &clockx.Stepping{At: start, Step: time.Second}

	deps := Deps{
		Repo: repo, Runs: runs, Executor: exec,
		Tokens: &tokens{}, Directory: directory{"atlas": true},
		Clock: clock, IDs: &countingIDs{},
	}
	for _, opt := range opts {
		opt(&deps)
	}
	return &harness{svc: NewService(deps), repo: repo, runs: runs, executor: exec, clock: clock}
}

func asAgent(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: id})
}

func (h *harness) create(t *testing.T, in CreateInput) CreateOutput {
	t.Helper()
	got, err := h.svc.Create(asAgent("atlas"), in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestTheThreeTriggersAreBuiltAndTheFourthIsRefused.
func TestTheThreeTriggersAreBuiltAndTheFourthIsRefused(t *testing.T) {
	h := newHarness(t)

	out := h.create(t, CreateInput{
		Name: "Everything at once",
		Triggers: []TriggerInput{
			{Type: Scheduled, Cron: "0 9 * * 1-5"},
			{Type: Webhook},
			{Type: Activity, Namespace: "task", Event: "status_changed"},
		},
	})
	if len(out.Routine.Triggers) != 3 {
		t.Fatalf("triggers = %+v", out.Routine.Triggers)
	}
	if out.Token == "" {
		t.Fatal("a webhook trigger minted no token")
	}
	for _, tr := range out.Routine.Triggers {
		if tr.Type == Webhook && !strings.Contains(tr.Config.TokenHash, out.Token) {
			t.Fatalf("the stored hash does not derive from the token: %q", tr.Config.TokenHash)
		}
	}

	if _, err := h.svc.Create(asAgent("atlas"), CreateInput{
		Name: "Unknown", Triggers: []TriggerInput{{Type: "polling"}},
	}); err == nil {
		t.Fatal("a fourth kind of trigger was accepted")
	}
}

// TestTheTokenIsShownOnceAndStoredAsAHash. The original writes it in clear into
// front matter that is committed to Git.
func TestTheTokenIsShownOnceAndStoredAsAHash(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Hooked", Triggers: []TriggerInput{{Type: Webhook}}})

	stored, err := h.svc.Get(asAgent("atlas"), GetInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range stored.Triggers {
		if tr.Config.TokenHash == out.Token {
			t.Fatal("the token itself is stored on the routine")
		}
	}

	// Reading it again never yields the token: only a rotation does.
	if got, _ := h.svc.Get(asAgent("atlas"), GetInput{ID: out.Routine.ID}); got == nil {
		t.Fatal("the routine disappeared")
	}
	rotated, err := h.svc.Rotate(asAgent("atlas"), RotateInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Token == "" || rotated.Token == out.Token {
		t.Fatalf("rotation produced %q", rotated.Token)
	}

	// The old token stops working immediately.
	if _, err := h.svc.FireWebhook(context.Background(), WebhookInput{
		Agent: "atlas", ID: out.Routine.ID, Token: out.Token,
	}); err == nil {
		t.Fatal("a rotated-away token still fires the routine")
	}
	if _, err := h.svc.FireWebhook(context.Background(), WebhookInput{
		Agent: "atlas", ID: out.Routine.ID, Token: rotated.Token,
	}); err != nil {
		t.Fatalf("the new token was refused: %v", err)
	}
}

// TestAWrongTokenIsRefusedWithoutSayingWhichPartWasWrong. An endpoint that
// distinguishes "no such routine" from "wrong token" is an oracle.
func TestAWrongTokenIsRefusedWithoutSayingWhichPartWasWrong(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Hooked", Triggers: []TriggerInput{{Type: Webhook}}})

	_, err := h.svc.FireWebhook(context.Background(), WebhookInput{
		Agent: "atlas", ID: out.Routine.ID, Token: "guessed",
	})
	if err == nil {
		t.Fatal("a wrong token fired the routine")
	}
	got, ok := apperr.As(err)
	if !ok || !strings.HasSuffix(got.Code, "ROUTINE_FIRE_INVALID_TOKEN") {
		t.Fatalf("error = %v", err)
	}
	if h.executor.count() != 0 {
		t.Fatal("the routine ran despite the refusal")
	}
}

// TestADisabledRoutineDoesNotFireByAnyTrigger, and the attempt is recorded.
func TestADisabledRoutineDoesNotFireByAnyTrigger(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{
		Name: "Off", Status: Disabled,
		Triggers: []TriggerInput{
			{Type: Scheduled, Cron: "* * * * *"},
			{Type: Activity, Namespace: "task"},
		},
	})

	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err == nil {
		t.Fatal("a disabled routine fired by hand")
	}
	if _, err := h.svc.ProcessScheduled(asAgent("atlas"), start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	h.svc.OnActivity(asAgent("atlas"), "task", "created", nil)

	if h.executor.count() != 0 {
		t.Fatalf("a disabled routine ran %d times", h.executor.count())
	}

	// Firing by hand still recorded that it was asked and refused.
	history, err := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Runs) != 1 || history.Runs[0].Status != RunSkipped {
		t.Fatalf("runs = %+v", history.Runs)
	}
}

// TestForceRunsADisabledRoutineOnceWithoutEnablingIt.
func TestForceRunsADisabledRoutineOnceWithoutEnablingIt(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Off", Status: Disabled})

	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID, Force: true}); err != nil {
		t.Fatal(err)
	}
	if h.executor.count() != 1 {
		t.Fatal("force did not run it")
	}
	after, err := h.svc.Get(asAgent("atlas"), GetInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != Disabled {
		t.Fatal("forcing a run enabled the routine")
	}
}

// TestEveryFiringRecordsARunIncludingTheOnesThatFail. Failing without a record
// is the worst outcome for an audit trail: afterwards nobody can tell "it ran
// and did nothing" from "it never ran".
func TestEveryFiringRecordsARunIncludingTheOnesThatFail(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Executor = &executor{failWith: errors.New("the model refused")}
	})
	out := h.create(t, CreateInput{Name: "Fails"})

	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err == nil {
		t.Fatal("a failing routine reported success")
	}
	history, err := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Runs) != 1 {
		t.Fatalf("runs = %+v", history.Runs)
	}
	run := history.Runs[0]
	if run.Status != RunFailed || run.Error == "" || run.EndedAt == nil {
		t.Fatalf("run = %+v", run)
	}
}

// TestARoutineThatPanicsIsARunThatFailed, not a daemon that stops.
func TestARoutineThatPanicsIsARunThatFailed(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Executor = &executor{panics: true} })
	out := h.create(t, CreateInput{Name: "Explodes"})

	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err == nil {
		t.Fatal("a panicking routine reported success")
	}
	history, err := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Runs) != 1 || history.Runs[0].Status != RunFailed {
		t.Fatalf("runs = %+v", history.Runs)
	}
}

// TestATimeoutIsRecordedAsOne, distinct from a failure — the two send whoever
// reads the history to different places.
func TestATimeoutIsRecordedAsOne(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Executor = &executor{failWith: context.DeadlineExceeded}
	})
	out := h.create(t, CreateInput{Name: "Slow"})

	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err == nil {
		t.Fatal("a timed-out routine reported success")
	}
	history, _ := h.svc.Runs(asAgent("atlas"), RunsInput{ID: out.Routine.ID})
	if len(history.Runs) != 1 || history.Runs[0].Status != RunTimedOut {
		t.Fatalf("runs = %+v", history.Runs)
	}
}

// TestARoutineRunsAsItsOwnerNotAsWhoeverTriggeredIt. A webhook from the
// internet must not inherit the identity of the process that received it.
func TestARoutineRunsAsItsOwnerNotAsWhoeverTriggeredIt(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Hooked", Triggers: []TriggerInput{{Type: Webhook}}})

	if _, err := h.svc.FireWebhook(context.Background(), WebhookInput{
		Agent: "atlas", ID: out.Routine.ID, Token: out.Token,
	}); err != nil {
		t.Fatal(err)
	}
	if len(h.executor.actors) != 1 || h.executor.actors[0] != "atlas" {
		t.Fatalf("the routine ran as %v", h.executor.actors)
	}
}

// TestAnActivityFiresTheRoutinesThatMatchIt, and the filters decide which.
func TestAnActivityFiresTheRoutinesThatMatchIt(t *testing.T) {
	h := newHarness(t)
	h.create(t, CreateInput{
		Name: "Only reviewed bugs",
		Triggers: []TriggerInput{{
			Type: Activity, Namespace: "task", Event: "status_changed",
			Filters: []Filter{
				{Field: "to", Operator: OpEq, Value: "in_review"},
				{Field: "type", Operator: OpEq, Value: "bug"},
			},
		}},
	})

	h.svc.OnActivity(asAgent("atlas"), "task", "status_changed",
		map[string]any{"to": "in_review", "type": "feature"})
	if h.executor.count() != 0 {
		t.Fatal("a feature entering review fired the bug routine")
	}

	h.svc.OnActivity(asAgent("atlas"), "task", "status_changed",
		map[string]any{"to": "in_review", "type": "bug"})
	if h.executor.count() != 1 {
		t.Fatalf("a bug entering review fired %d times", h.executor.count())
	}

	h.svc.OnActivity(asAgent("atlas"), "memory", "stored", map[string]any{"to": "in_review", "type": "bug"})
	if h.executor.count() != 1 {
		t.Fatal("an activity in another namespace fired the routine")
	}
}

// TestTheThreeFilterOperators, including the rule that a missing field never
// matches — even under neq, where the naive reading would fire for every
// unrelated activity in the namespace.
func TestTheThreeFilterOperators(t *testing.T) {
	data := map[string]any{"type": "bug", "count": 3, "title": "Denial patterns"}

	cases := []struct {
		filter Filter
		want   bool
	}{
		{Filter{Field: "type", Operator: OpEq, Value: "bug"}, true},
		{Filter{Field: "type", Operator: OpEq, Value: "feature"}, false},
		{Filter{Field: "type", Operator: OpNeq, Value: "feature"}, true},
		{Filter{Field: "type", Operator: OpNeq, Value: "bug"}, false},
		{Filter{Field: "title", Operator: OpContains, Value: "denial"}, true},
		{Filter{Field: "title", Operator: OpContains, Value: "sandbox"}, false},
		{Filter{Field: "count", Operator: OpEq, Value: 3}, true},
		{Filter{Field: "count", Operator: OpEq, Value: 3.0}, true},
		{Filter{Field: "absent", Operator: OpEq, Value: "x"}, false},
		{Filter{Field: "absent", Operator: OpNeq, Value: "x"}, false},
		{Filter{Field: "type", Operator: "startsWith", Value: "b"}, false},
	}
	for _, tc := range cases {
		if got := tc.filter.Matches(data); got != tc.want {
			t.Errorf("%+v matched %v, want %v", tc.filter, got, tc.want)
		}
	}
}

// TestACronFinerThanTheTickFiresOncePerWindow. That is the effective resolution
// of the system, and the whole reason DueInWindow returns a boolean.
func TestACronFinerThanTheTickFiresOncePerWindow(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{
		Name: "Every minute, allegedly",
		Triggers: []TriggerInput{{Type: Scheduled, Cron: "* * * * *"}},
	})

	fired, err := h.svc.ProcessScheduled(asAgent("atlas"), start.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(fired.Fired) != 1 {
		t.Fatalf("fired = %v", fired.Fired)
	}
	if h.executor.count() != 1 {
		t.Fatalf("a per-minute cron ran %d times in one window", h.executor.count())
	}

	// A second tick one minute later is inside the same window as far as the
	// routine is concerned: it fired at the start of it.
	if _, err := h.svc.ProcessScheduled(asAgent("atlas"), start.Add(16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if h.executor.count() != 2 {
		t.Fatalf("the routine ran %d times over two ticks", h.executor.count())
	}

	view, err := h.svc.Get(asAgent("atlas"), GetInput{ID: out.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Warnings) == 0 {
		t.Fatal("a per-minute cron produced no warning about the real resolution")
	}
	if view.EffectiveInterval != "15m0s" {
		t.Fatalf("effective interval = %q", view.EffectiveInterval)
	}
}

// TestACronThatIsNotDueDoesNotFire.
func TestACronThatIsNotDueDoesNotFire(t *testing.T) {
	h := newHarness(t)
	h.create(t, CreateInput{
		Name: "Weekday mornings",
		Triggers: []TriggerInput{{Type: Scheduled, Cron: "0 9 * * 1-5"}},
	})

	// A Monday at 14:00, window 13:45–14:00. The nine-o'clock slot is behind us.
	if _, err := h.svc.ProcessScheduled(asAgent("atlas"),
		time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if h.executor.count() != 0 {
		t.Fatal("a nine-o'clock routine fired at two in the afternoon")
	}

	// The next morning's window contains it.
	if _, err := h.svc.ProcessScheduled(asAgent("atlas"),
		time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if h.executor.count() != 1 {
		t.Fatalf("the routine ran %d times", h.executor.count())
	}
}

// TestACronThatDoesNotParseIsReportedOnEveryTick. The alternative is silence
// for as long as it stays broken.
func TestACronThatDoesNotParseIsReportedOnEveryTick(t *testing.T) {
	h := newHarness(t)
	// The service refuses to store one, so this is the file-edited-by-hand case.
	stored := &Routine{
		Agent: "atlas", ID: "hand-written", Name: "Broken", Status: Enabled,
		Triggers: []Trigger{{Type: Scheduled, Config: TriggerConfig{Cron: "every friday"}}},
	}
	if err := h.repo.Create(context.Background(), stored); err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.ProcessScheduled(asAgent("atlas"), start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Broken) != 1 || out.Broken[0] != "hand-written" {
		t.Fatalf("broken = %v", out.Broken)
	}

	view, err := h.svc.Get(asAgent("atlas"), GetInput{ID: "hand-written"})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Warnings) == 0 {
		t.Fatal("a routine that can never fire reads as healthy")
	}
}

// TestScopeIsWhatTheRoutineMayDo, and it reaches the runtime with the run.
func TestScopeIsWhatTheRoutineMayDo(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{
		Name:  "Triage",
		Scope: Scope{AllowCreateTasks: true},
	})
	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err != nil {
		t.Fatal(err)
	}
	if !h.executor.calls[0].Scope.AllowCreateTasks {
		t.Fatal("the scope did not reach the runtime")
	}
}

// TestWhatAScopeAllows.
func TestWhatAScopeAllows(t *testing.T) {
	cases := []struct {
		name  string
		scope Scope
		tool  string
		want  bool
	}{
		{"the default withholds task creation", Scope{}, "tasks_create", false},
		{"the default withholds the network", Scope{}, "web_fetch", false},
		{"the default allows an ordinary tool", Scope{}, "memories_recall", true},
		{"the flag opens task creation", Scope{AllowCreateTasks: true}, "tasks_create", true},
		{"the flag opens the network", Scope{AllowExternalCalls: true}, "web_fetch", true},
		{"an allowlist is exhaustive", Scope{AllowedTools: []string{"memories_recall"}}, "memories_recall", true},
		{"an allowlist excludes the rest", Scope{AllowedTools: []string{"memories_recall"}}, "tasks_list", false},
		{
			"an allowlist beats the flags",
			Scope{AllowCreateTasks: true, AllowedTools: []string{"memories_recall"}},
			"tasks_create", false,
		},
	}
	for _, tc := range cases {
		if got := tc.scope.Allows(tc.tool); got != tc.want {
			t.Errorf("%s: %q allowed = %v, want %v", tc.name, tc.tool, got, tc.want)
		}
	}
}

// TestARoutineCannotBelongToAnAgentThatIsNotThere.
func TestARoutineCannotBelongToAnAgentThatIsNotThere(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Create(asAgent("ghost"), CreateInput{Name: "Orphan"}); err == nil {
		t.Fatal("a routine was created under an agent that does not exist")
	}
}

// TestDeletingARoutineTakesItsRuns is the collection cascade, declared once in
// the registry rather than hooked here.
func TestDeletingARoutineTakesItsRuns(t *testing.T) {
	h := newHarness(t)
	out := h.create(t, CreateInput{Name: "Doomed"})
	if _, err := h.svc.Fire(asAgent("atlas"), FireInput{ID: out.Routine.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Delete(asAgent("atlas"), DeleteInput{ID: out.Routine.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Get(asAgent("atlas"), GetInput{ID: out.Routine.ID}); err == nil {
		t.Fatal("the deleted routine is still readable")
	}
}
