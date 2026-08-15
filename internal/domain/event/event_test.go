package event_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/event"
)

var refTime = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// recorder is the audit sink in memory: the domain must be provable without a
// filesystem, and what the log does with a record is the adapter's contract.
type recorder struct {
	mu      sync.Mutex
	records []event.Record
	fail    error
}

func (r *recorder) Append(_ context.Context, rec event.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail != nil {
		return r.fail
	}
	r.records = append(r.records, rec)
	return nil
}

func (r *recorder) all() []event.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event.Record(nil), r.records...)
}

func newBus(t *testing.T, log event.Log, strict bool) *event.Service {
	t.Helper()
	return event.NewService(event.Deps{
		Log:    log,
		Clock:  &clockx.Stepping{At: refTime, Step: time.Millisecond},
		IDs:    &ids.Sequence{Prefix: "rec"},
		Strict: strict,
	})
}

func hook(id string, t event.Type, fn func(event.Event) (event.Outcome, error)) event.Handler {
	return event.FuncHandler{
		Name:   id,
		Events: []event.Type{t},
		Fn: func(_ context.Context, e event.Event) (event.Outcome, error) {
			return fn(e)
		},
	}
}

// TestTheNineEventsDeclareWhatTheyCanDo locks the capability table. A handler
// author reads this table; a runtime that disagrees with it makes hooks that
// look correct behave differently on two events.
func TestTheNineEventsDeclareWhatTheyCanDo(t *testing.T) {
	blocking := map[event.Type]bool{
		event.UserPromptSubmit: true,
		event.PreToolUse:       true,
		event.PostToolUse:      true,
		event.SubagentStop:     true,
		event.Stop:             true,
	}
	for _, ty := range event.Types {
		if got, want := ty.CanBlock(), blocking[ty]; got != want {
			t.Errorf("%s.CanBlock() = %v, want %v", ty, got, want)
		}
		if !ty.Valid() {
			t.Errorf("%s is not in Types", ty)
		}
	}
	if len(event.Types) != 10 {
		t.Fatalf("expected the nine events plus the generic one, got %d", len(event.Types))
	}
}

// TestContextAccumulatesAndTheFirstBlockWins is the dispatch rule in one test:
// two handlers each add context, the second blocks, and the first one's
// contribution survives into the outcome the runtime acts on.
func TestContextAccumulatesAndTheFirstBlockWins(t *testing.T) {
	log := &recorder{}
	bus := newBus(t, log, false)

	bus.Register(hook("adds", event.UserPromptSubmit, func(event.Event) (event.Outcome, error) {
		return event.Outcome{AdditionalContext: "the repository is in a release freeze"}, nil
	}))
	bus.Register(hook("blocks", event.UserPromptSubmit, func(event.Event) (event.Outcome, error) {
		return event.Outcome{Decision: event.DecisionBlock, Reason: "frozen"}, nil
	}))
	bus.Register(hook("never-runs", event.UserPromptSubmit, func(event.Event) (event.Outcome, error) {
		t.Error("a handler after the block was called")
		return event.Outcome{}, nil
	}))

	out, err := bus.Emit(context.Background(), event.Event{Type: event.UserPromptSubmit, Prompt: "ship it"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Blocked() {
		t.Fatal("the turn was not blocked")
	}
	if out.HookID != "blocks" {
		t.Errorf("HookID = %q, want the hook that decided", out.HookID)
	}
	if !strings.Contains(out.AdditionalContext, "release freeze") {
		t.Errorf("the first hook's context was lost: %q", out.AdditionalContext)
	}
	if got := len(log.all()); got != 2 {
		t.Fatalf("recorded %d invocations, want 2", got)
	}
}

// TestEveryInvocationIsRecordedIncludingTheQuietOnes is what separates an audit
// log from a diary. A handler that decided nothing still ran, and "nobody
// objected" is an answer somebody will need.
func TestEveryInvocationIsRecordedIncludingTheQuietOnes(t *testing.T) {
	log := &recorder{}
	bus := newBus(t, log, false)
	bus.Register(hook("quiet", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		return event.Outcome{}, nil
	}))

	if _, err := bus.Emit(context.Background(), event.Event{Type: event.PreToolUse, Tool: "Bash"}); err != nil {
		t.Fatal(err)
	}
	records := log.all()
	if len(records) != 1 {
		t.Fatalf("recorded %d, want 1", len(records))
	}
	if records[0].Hook != "quiet" || records[0].Type != event.PreToolUse {
		t.Fatalf("record = %+v", records[0])
	}
	if records[0].ID == "" || records[0].CreatedAt.IsZero() {
		t.Errorf("a record without an id or a time is not auditable: %+v", records[0])
	}
}

// TestTheRewrittenPayloadIsInTheRecord. A hook can silently change what a tool
// is about to do; invisibility there would be unacceptable, and the before is
// in the payload while the after is in the outcome.
func TestTheRewrittenPayloadIsInTheRecord(t *testing.T) {
	log := &recorder{}
	bus := newBus(t, log, false)
	bus.Register(hook("sanitiser", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		return event.Outcome{UpdatedInput: json.RawMessage(`{"path":"/safe/notes.md"}`)}, nil
	}))

	out, err := bus.Emit(context.Background(), event.Event{
		Type: event.PreToolUse, Tool: "Write",
		Input: json.RawMessage(`{"path":"/etc/hosts"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(out.UpdatedInput) != `{"path":"/safe/notes.md"}` {
		t.Fatalf("UpdatedInput = %s", out.UpdatedInput)
	}
	rec := log.all()[0]
	if !strings.Contains(string(rec.Payload), "/etc/hosts") {
		t.Errorf("the original payload is not in the record: %s", rec.Payload)
	}
	if !strings.Contains(string(mustJSON(t, rec.Outcome)), "/safe/notes.md") {
		t.Errorf("the rewrite is not in the record: %+v", rec.Outcome)
	}
	if rec.Outcome.HookID != "sanitiser" {
		t.Errorf("the record does not say who rewrote it: %+v", rec.Outcome)
	}
}

// TestABlockOnAnEventThatCannotBlockIsDropped. The capability table is a
// contract in both directions: a handler that returns block on SessionStart is
// wrong, and obeying it would give that event a power the contract denies it.
func TestABlockOnAnEventThatCannotBlockIsDropped(t *testing.T) {
	bus := newBus(t, &recorder{}, false)
	bus.Register(hook("overreach", event.SessionStart, func(event.Event) (event.Outcome, error) {
		return event.Outcome{Decision: event.DecisionBlock, Reason: "no"}, nil
	}))

	out, err := bus.Emit(context.Background(), event.Event{Type: event.SessionStart})
	if err != nil {
		t.Fatal(err)
	}
	if out.Blocked() {
		t.Fatal("SessionStart blocked a turn")
	}
}

// TestAFailingHookIsRecordedAndTheTurnContinues, unless the workspace asked for
// the opposite.
func TestAFailingHookIsRecordedAndTheTurnContinues(t *testing.T) {
	broken := hook("broken", event.Stop, func(event.Event) (event.Outcome, error) {
		return event.Outcome{}, errors.New("no such file or directory")
	})

	t.Run("lenient", func(t *testing.T) {
		log := &recorder{}
		bus := newBus(t, log, false)
		bus.Register(broken)
		bus.Register(hook("after", event.Stop, func(event.Event) (event.Outcome, error) {
			return event.Outcome{AdditionalContext: "still here"}, nil
		}))

		out, err := bus.Emit(context.Background(), event.Event{Type: event.Stop})
		if err != nil {
			t.Fatalf("a broken hook stopped the turn: %v", err)
		}
		if out.AdditionalContext != "still here" {
			t.Error("the handler after the broken one did not run")
		}
		if got := log.all()[0].Error; !strings.Contains(got, "no such file") {
			t.Errorf("the failure was not recorded: %q", got)
		}
	})

	t.Run("strict", func(t *testing.T) {
		bus := newBus(t, &recorder{}, true)
		bus.Register(broken)
		if _, err := bus.Emit(context.Background(), event.Event{Type: event.Stop}); err == nil {
			t.Fatal("strict mode swallowed a hook failure")
		} else if !strings.Contains(err.Error(), "HOOK_FAILED") {
			t.Fatalf("err = %v", err)
		}
	})
}

// TestAPanickingHookCostsOnlyItsOwnOpinion. A hook is third-party code on the
// hottest path in the system.
func TestAPanickingHookCostsOnlyItsOwnOpinion(t *testing.T) {
	log := &recorder{}
	bus := newBus(t, log, false)
	bus.Register(hook("boom", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		panic("nil map")
	}))
	bus.Register(hook("survivor", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		return event.Outcome{AdditionalContext: "ran anyway"}, nil
	}))

	out, err := bus.Emit(context.Background(), event.Event{Type: event.PreToolUse, Tool: "Read"})
	if err != nil {
		t.Fatal(err)
	}
	if out.AdditionalContext != "ran anyway" {
		t.Fatalf("the second hook did not run: %+v", out)
	}
	if got := log.all()[0].Error; !strings.Contains(got, "PANIC") && !strings.Contains(got, "panic") {
		t.Errorf("the panic was not recorded: %q", got)
	}
}

// TestASlowHookIsBoundedByTheTimeout. A hook blocked on stdin must not hold a
// turn open.
func TestASlowHookIsBoundedByTheTimeout(t *testing.T) {
	log := &recorder{}
	bus := event.NewService(event.Deps{
		Log:     log,
		Clock:   clockx.Fixed{At: refTime},
		IDs:     &ids.Sequence{Prefix: "rec"},
		Timeout: 20 * time.Millisecond,
	})
	bus.Register(event.FuncHandler{
		Name:   "slow",
		Events: []event.Type{event.Stop},
		Fn: func(ctx context.Context, _ event.Event) (event.Outcome, error) {
			<-ctx.Done()
			return event.Outcome{}, ctx.Err()
		},
	})

	start := time.Now()
	if _, err := bus.Emit(context.Background(), event.Event{Type: event.Stop}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the hook held the turn for %s", elapsed)
	}
	if got := log.all()[0].Error; got == "" {
		t.Error("the timeout was not recorded")
	}
}

// TestAnUnwritableLogDoesNotStopTheTurn. The audit sink is a thing to report,
// not a reason to refuse to work.
func TestAnUnwritableLogDoesNotStopTheTurn(t *testing.T) {
	bus := newBus(t, &recorder{fail: errors.New("read-only filesystem")}, false)
	bus.Register(hook("any", event.Stop, func(event.Event) (event.Outcome, error) {
		return event.Outcome{}, nil
	}))
	if _, err := bus.Emit(context.Background(), event.Event{Type: event.Stop}); err != nil {
		t.Fatalf("an unwritable log stopped the turn: %v", err)
	}
}

// TestRiskComesFromWhatTheToolDeclares, not from a list somebody maintains.
func TestRiskComesFromWhatTheToolDeclares(t *testing.T) {
	cases := []struct {
		name                             string
		readOnly, destructive, openWorld bool
		want                             event.RiskLevel
	}{
		{"a read is low", true, false, false, event.RiskLow},
		{"reaching outside the machine is medium", false, false, true, event.RiskMedium},
		{"destructive is high whatever else it is", true, true, true, event.RiskHigh},
		{"an unannotated tool is not assumed harmless", false, false, false, event.RiskMedium},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := event.Risk(c.readOnly, c.destructive, c.openWorld); got != c.want {
				t.Fatalf("Risk = %s, want %s", got, c.want)
			}
		})
	}
}

// TestAskReachesAHumanAndTheAnswerComesBack is ADR-0007: the divergence that
// makes a hook's request for confirmation mean what it says.
func TestAskReachesAHumanAndTheAnswerComesBack(t *testing.T) {
	seen := make(chan event.ApprovalRequest, 1)
	broker := event.NewBroker(event.BrokerDeps{
		Clock:    clockx.Fixed{At: refTime},
		IDs:      &ids.Sequence{Prefix: "ask"},
		Notifier: notifier{requested: seen},
		Deadline: 2 * time.Second,
	})

	done := make(chan event.ApprovalResult, 1)
	go func() {
		res, err := broker.RequestApproval(context.Background(), event.ApprovalRequest{
			ToolName: "Bash", Risk: event.RiskHigh,
			Input: json.RawMessage(`{"command":"git push --force"}`),
		})
		if err != nil {
			t.Error(err)
		}
		done <- res
	}()

	req := <-seen
	if req.ToolName != "Bash" || req.ID == "" {
		t.Fatalf("the request that reached the human is wrong: %+v", req)
	}
	if !broker.Decide(req.ID, event.ApprovalResult{
		Approved: true, Remember: event.RememberSession,
		UpdatedInput: json.RawMessage(`{"command":"git push"}`),
	}) {
		t.Fatal("the decision did not land")
	}

	res := <-done
	if !res.Approved || string(res.UpdatedInput) != `{"command":"git push"}` {
		t.Fatalf("result = %+v", res)
	}
	if got := broker.Pending(); len(got) != 0 {
		t.Errorf("the request is still pending: %+v", got)
	}
}

// TestEveryWayOutThatIsNotAYesIsANo. Fail-closed is the invariant of the
// approval channel, and the three ways to leave it without an answer are a
// deadline, a cancelled turn and a daemon shutting down.
func TestEveryWayOutThatIsNotAYesIsANo(t *testing.T) {
	t.Run("the deadline elapses", func(t *testing.T) {
		broker := event.NewBroker(event.BrokerDeps{
			Clock: clockx.Fixed{At: refTime}, IDs: &ids.Sequence{Prefix: "ask"},
			Deadline: 10 * time.Millisecond,
		})
		res, err := broker.RequestApproval(context.Background(), event.ApprovalRequest{ToolName: "Bash"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Approved {
			t.Fatal("a timeout approved a tool call")
		}
		if !strings.Contains(res.Reason, "nobody answered") {
			t.Errorf("reason = %q", res.Reason)
		}
	})

	t.Run("the turn is cancelled", func(t *testing.T) {
		broker := event.NewBroker(event.BrokerDeps{
			Clock: clockx.Fixed{At: refTime}, IDs: &ids.Sequence{Prefix: "ask"},
			Deadline: time.Minute,
		})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for len(broker.Pending()) == 0 {
				time.Sleep(time.Millisecond)
			}
			cancel()
		}()
		res, err := broker.RequestApproval(ctx, event.ApprovalRequest{ToolName: "Bash"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Approved {
			t.Fatal("a cancelled turn approved a tool call")
		}
	})

	t.Run("the daemon shuts down", func(t *testing.T) {
		broker := event.NewBroker(event.BrokerDeps{
			Clock: clockx.Fixed{At: refTime}, IDs: &ids.Sequence{Prefix: "ask"},
			Deadline: time.Minute,
		})
		done := make(chan event.ApprovalResult, 1)
		go func() {
			res, _ := broker.RequestApproval(context.Background(), event.ApprovalRequest{ToolName: "Bash"})
			done <- res
		}()
		for len(broker.Pending()) == 0 {
			time.Sleep(time.Millisecond)
		}
		broker.Close()
		if res := <-done; res.Approved {
			t.Fatal("a shutdown approved a tool call")
		}
	})
}

// TestTwoPeopleClickingAtOnceProduceOneDecision.
func TestTwoPeopleClickingAtOnceProduceOneDecision(t *testing.T) {
	broker := event.NewBroker(event.BrokerDeps{
		Clock: clockx.Fixed{At: refTime}, IDs: &ids.Sequence{Prefix: "ask"},
		Deadline: 2 * time.Second,
	})
	go func() {
		_, _ = broker.RequestApproval(context.Background(), event.ApprovalRequest{ToolName: "Bash"})
	}()
	var id string
	for id == "" {
		if p := broker.Pending(); len(p) > 0 {
			id = p[0].ID
		}
		time.Sleep(time.Millisecond)
	}

	var landed int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if broker.Decide(id, event.ApprovalResult{Approved: i%2 == 0}) {
				mu.Lock()
				landed++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if landed != 1 {
		t.Fatalf("%d decisions landed on one request", landed)
	}
}

// TestHeadlessDeniesImmediatelyAndSaysWhy. The original's message for this case
// is indistinguishable from a policy denial, which sends the reader looking for
// a hook that does not exist.
func TestHeadlessDeniesImmediatelyAndSaysWhy(t *testing.T) {
	start := time.Now()
	res, err := event.NoopApprover{}.RequestApproval(context.Background(),
		event.ApprovalRequest{ToolName: "Bash", Deadline: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res.Approved {
		t.Fatal("the headless approver approved")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("it waited %s for a human who is not there", elapsed)
	}
	if !strings.Contains(res.Reason, "no approval channel") {
		t.Errorf("reason = %q — it should say the channel is missing, not that a hook denied", res.Reason)
	}
}

// TestDecidingSomethingNobodyAskedIsNotAnError, it is a no.
func TestDecidingSomethingNobodyAskedIsNotAnError(t *testing.T) {
	broker := event.NewBroker(event.BrokerDeps{Clock: clockx.Fixed{At: refTime}, IDs: &ids.Sequence{Prefix: "ask"}})
	if broker.Decide("never-asked", event.ApprovalResult{Approved: true}) {
		t.Fatal("an answer landed on a request that does not exist")
	}
}

// TestABlockCarriesAnActionableError: the code, the hook and the reason.
func TestABlockCarriesAnActionableError(t *testing.T) {
	err := event.Blocked(event.UserPromptSubmit, event.Outcome{HookID: "freeze", Reason: "release freeze"})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err = %T", err)
	}
	if app.HTTPStatus != apperr.StatusForbidden {
		t.Errorf("status = %d, want 403 — nothing malfunctioned", app.HTTPStatus)
	}
	if app.Issues["hook"] != "freeze" || !strings.Contains(app.Message, "release freeze") {
		t.Fatalf("error = %+v", app)
	}
}

// TestSanitizeAgentCannotEscapeTheDirectory. The log path is built from an id
// that may have arrived in a payload.
func TestSanitizeAgentCannotEscapeTheDirectory(t *testing.T) {
	cases := map[string]string{
		"atlas":            "atlas",
		"Atlas":            "atlas",
		"../../etc/passwd": "etc-passwd",
		"":                 "unknown",
		"///":              "unknown",
	}
	for in, want := range cases {
		if got := event.SanitizeAgent(in); got != want {
			t.Errorf("SanitizeAgent(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSortedTypesReportsCoverage.
func TestSortedTypesReportsCoverage(t *testing.T) {
	got := event.SortedTypes([]event.Handler{
		event.FuncHandler{Name: "a", Events: []event.Type{event.Stop, event.PreToolUse}},
		event.FuncHandler{Name: "b", Events: []event.Type{event.PreToolUse}},
	})
	if len(got) != 2 || got[0] != event.PreToolUse || got[1] != event.Stop {
		t.Fatalf("SortedTypes = %v", got)
	}
}

// TestAHandlerThatWantsNothingIsHarmless.
func TestAHandlerThatWantsNothingIsHarmless(t *testing.T) {
	bus := newBus(t, &recorder{}, false)
	bus.Register(nil)
	bus.Register(event.FuncHandler{Name: "idle"})
	if got := bus.Handlers(event.Stop); len(got) != 0 {
		t.Fatalf("Handlers = %v", got)
	}
}

type notifier struct{ requested chan event.ApprovalRequest }

func (n notifier) ApprovalRequested(_ context.Context, req event.ApprovalRequest) {
	n.requested <- req
}
func (n notifier) ApprovalSettled(context.Context, event.ApprovalRequest, event.ApprovalResult) {}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
