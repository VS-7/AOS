package agentloop_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

var refTime = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// recording is a tool that remembers what it was called with, which is how the
// rewriting test proves the tool saw the new payload and not the old one.
type recording struct {
	name string
	mu   sync.Mutex
	seen []string
	fn   func(json.RawMessage) (any, error)
	busy atomic.Int32
	peak atomic.Int32
}

func (r *recording) Name() string { return r.name }
func (r *recording) Spec() toolexec.Spec {
	return toolexec.Spec{Name: r.name, Description: "a tool for a test"}
}
func (r *recording) Invoke(_ context.Context, in json.RawMessage) (any, error) {
	now := r.busy.Add(1)
	for {
		peak := r.peak.Load()
		if now <= peak || r.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	defer r.busy.Add(-1)

	r.mu.Lock()
	r.seen = append(r.seen, string(in))
	r.mu.Unlock()

	if r.fn != nil {
		return r.fn(in)
	}
	time.Sleep(20 * time.Millisecond)
	return map[string]any{"ok": true}, nil
}

func (r *recording) inputs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.seen...)
}

func state() *agentloop.State {
	return &agentloop.State{
		SessionID: "s-1", AgentID: "atlas", Workspace: "atelier",
		Instructions: "<context></context>",
		Messages:     []agentloop.Message{{Role: agentloop.RoleUser, Text: "do the thing"}},
		Model:        "gpt-5",
	}
}

func loop(t *testing.T, p agentloop.LLMProvider, tools *toolexec.Registry, hooks agentloop.Hooks) *agentloop.Loop {
	t.Helper()
	return agentloop.New(agentloop.Deps{
		Provider: p, Tools: tools, Hooks: hooks,
		Clock: &clockx.Stepping{At: refTime, Step: time.Second},
		Log:   quiet(),
	})
}

// bus builds a hook bus with the handlers a test registers.
func bus(t *testing.T, handlers ...event.Handler) (*event.Service, *recorder) {
	t.Helper()
	rec := &recorder{}
	b := event.NewService(event.Deps{
		Log: rec, Clock: clockx.Fixed{At: refTime}, IDs: &ids.Sequence{Prefix: "rec"},
		Logger: quiet(),
	})
	for _, h := range handlers {
		b.Register(h)
	}
	return b, rec
}

type recorder struct {
	mu      sync.Mutex
	records []event.Record
}

func (r *recorder) Append(_ context.Context, rec event.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	return nil
}

func (r *recorder) types() []event.Type {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]event.Type, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, rec.Type)
	}
	return out
}

func hook(id string, t event.Type, fn func(event.Event) (event.Outcome, error)) event.Handler {
	return event.FuncHandler{
		Name: id, Events: []event.Type{t},
		Fn: func(_ context.Context, e event.Event) (event.Outcome, error) { return fn(e) },
	}
}

// TestATurnCallsAToolAndThenAnswers is the shape of everything else: the model
// asks for a tool, the loop runs it, the model reads the result and speaks.
func TestATurnCallsAToolAndThenAnswers(t *testing.T) {
	tool := &recording{name: "Read"}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Read", map[string]any{"file_path": "README.md"})},
			Usage: agentloop.Usage{Input: 100, Output: 20}},
		{Text: "The readme says hello.", Usage: agentloop.Usage{Input: 140, Output: 8}},
	}}

	res, err := loop(t, p, toolexec.NewRegistry().Add(tool), agentloop.NoHooks{}).
		Run(context.Background(), state())
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "The readme says hello." {
		t.Fatalf("Text = %q", res.Text)
	}
	if res.Steps != 2 {
		t.Errorf("Steps = %d", res.Steps)
	}
	if res.Usage.Total != 268 {
		t.Errorf("Usage.Total = %d, want the sum of both calls", res.Usage.Total)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "Read" {
		t.Fatalf("ToolCalls = %+v", res.ToolCalls)
	}
	if len(tool.inputs()) != 1 {
		t.Fatalf("the tool was called %d times", len(tool.inputs()))
	}
	// The second request carries the tool's answer, which is what the model
	// read in order to speak.
	second := p.Requests()[1]
	if last := second.Messages[len(second.Messages)-1]; last.Role != agentloop.RoleTool {
		t.Fatalf("the tool result did not reach the model: %+v", last)
	}
}

// TestABlockingPromptHookEndsTheTurnBeforeATokenIsSpent.
func TestABlockingPromptHookEndsTheTurnBeforeATokenIsSpent(t *testing.T) {
	p := fake.Text("this should never be said")
	b, rec := bus(t, hook("freeze", event.UserPromptSubmit, func(event.Event) (event.Outcome, error) {
		return event.Outcome{Decision: event.DecisionBlock, Reason: "the repository is frozen"}, nil
	}))

	_, err := loop(t, p, toolexec.NewRegistry(), &agentloop.EventHooks{Bus: b}).
		Run(context.Background(), state())
	if err == nil {
		t.Fatal("the turn ran")
	}
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_HOOK_BLOCKED" {
		t.Fatalf("err = %v", err)
	}
	if p.Steps() != 0 {
		t.Fatalf("the provider was called %d times", p.Steps())
	}
	// One handler, one invocation, one record: the bus records what ran, and
	// an event nobody listens for has nothing to record.
	if got := rec.types(); len(got) != 1 || got[0] != event.UserPromptSubmit {
		t.Fatalf("events = %v", got)
	}
}

// TestAHookRewritesWhatTheToolActuallyDoes. The third superpower, checked from
// the tool's side: what it received is the new payload, not the one the model
// sent.
func TestAHookRewritesWhatTheToolActuallyDoes(t *testing.T) {
	tool := &recording{name: "Bash"}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Bash", map[string]any{"command": "git", "args": []string{"push", "--force"}})}},
		{Text: "pushed"},
	}}
	b, _ := bus(t, hook("sanitiser", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		return event.Outcome{
			PermissionDecision: event.PermissionAllow,
			UpdatedInput:       json.RawMessage(`{"command":"git","args":["push"]}`),
		}, nil
	}))

	if _, err := loop(t, p, toolexec.NewRegistry().Add(tool), &agentloop.EventHooks{Bus: b}).
		Run(context.Background(), state()); err != nil {
		t.Fatal(err)
	}
	seen := tool.inputs()
	if len(seen) != 1 {
		t.Fatalf("the tool ran %d times", len(seen))
	}
	if strings.Contains(seen[0], "--force") {
		t.Fatalf("the tool received the original payload: %s", seen[0])
	}
}

// TestADenialIsAResultTheModelReadsAndNotAnError. Propagating it as a Go error
// would abort the turn over a decision that was made on purpose.
func TestADenialIsAResultTheModelReadsAndNotAnError(t *testing.T) {
	tool := &recording{name: "Bash"}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Bash", map[string]any{"command": "rm"})}},
		{Text: "I was not allowed to do that, so here is what I suggest instead."},
	}}
	b, _ := bus(t, hook("policy", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		return event.Outcome{PermissionDecision: event.PermissionDeny, Reason: "removal is not permitted here"}, nil
	}))

	res, err := loop(t, p, toolexec.NewRegistry().Add(tool), &agentloop.EventHooks{Bus: b}).
		Run(context.Background(), state())
	if err != nil {
		t.Fatalf("a denial aborted the turn: %v", err)
	}
	if len(tool.inputs()) != 0 {
		t.Fatal("the denied tool ran anyway")
	}
	if !res.ToolCalls[0].Denied {
		t.Fatalf("the result does not record the denial: %+v", res.ToolCalls[0])
	}
	// The model saw it, which is the whole reason a denial is a result.
	second := p.Requests()[1]
	last := second.Messages[len(second.Messages)-1]
	if !strings.Contains(string(last.Result), "removal is not permitted here") {
		t.Fatalf("the model did not see the reason: %s", last.Result)
	}
}

// TestAskReachesAHumanAndTheAnswerDecides — ADR-0007 from the loop's side, in
// its three outcomes.
func TestAskReachesAHumanAndTheAnswerDecides(t *testing.T) {
	askHook := hook("careful", event.PreToolUse, func(event.Event) (event.Outcome, error) {
		return event.Outcome{PermissionDecision: event.PermissionAsk, Reason: "this one needs a person"}, nil
	})

	run := func(t *testing.T, approver event.Approver) (*agentloop.Result, *recording, error) {
		t.Helper()
		tool := &recording{name: "Bash"}
		p := &fake.Provider{Script: []fake.Step{
			{Calls: []agentloop.ToolCall{fake.Call("c1", "Bash", map[string]any{"command": "gh"})}},
			{Text: "done"},
		}}
		b, _ := bus(t, askHook)
		res, err := loop(t, p, toolexec.NewRegistry().Add(tool),
			&agentloop.EventHooks{Bus: b, Approver: approver}).Run(context.Background(), state())
		return res, tool, err
	}

	t.Run("a person says yes", func(t *testing.T) {
		_, tool, err := run(t, answers{approved: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(tool.inputs()) != 1 {
			t.Fatal("the approved call did not run")
		}
	})

	t.Run("a person says no", func(t *testing.T) {
		res, tool, err := run(t, answers{approved: false, reason: "not on a Friday"})
		if err != nil {
			t.Fatal(err)
		}
		if len(tool.inputs()) != 0 {
			t.Fatal("the refused call ran")
		}
		if !strings.Contains(res.ToolCalls[0].Error, "not on a Friday") {
			t.Fatalf("the person's reason was lost: %+v", res.ToolCalls[0])
		}
	})

	t.Run("a person corrects the payload before saying yes", func(t *testing.T) {
		_, tool, err := run(t, answers{approved: true, updated: `{"command":"gh","args":["pr","view"]}`})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(tool.inputs()[0], "pr") {
			t.Fatalf("the correction did not reach the tool: %s", tool.inputs()[0])
		}
	})

	t.Run("there is nobody to ask", func(t *testing.T) {
		res, tool, err := run(t, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(tool.inputs()) != 0 {
			t.Fatal("a call nobody approved ran")
		}
		if !strings.Contains(res.ToolCalls[0].Error, "no approval channel") {
			t.Fatalf("the reason does not distinguish a missing channel from a policy denial: %q",
				res.ToolCalls[0].Error)
		}
	})

	t.Run("the headless approver denies without waiting", func(t *testing.T) {
		start := time.Now()
		res, tool, err := run(t, event.NoopApprover{})
		if err != nil {
			t.Fatal(err)
		}
		if len(tool.inputs()) != 0 || res.ToolCalls[0].Denied != true {
			t.Fatal("the headless run approved a call")
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("it waited %s for a human who is not there", elapsed)
		}
	})
}

// TestIndependentToolsRunAtOnce, because the master prompt tells the agent to
// call them in parallel and a runtime that serialises them makes that a lie.
func TestIndependentToolsRunAtOnce(t *testing.T) {
	tool := &recording{name: "Read"}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{
			fake.Call("c1", "Read", map[string]any{"file_path": "a"}),
			fake.Call("c2", "Read", map[string]any{"file_path": "b"}),
			fake.Call("c3", "Read", map[string]any{"file_path": "c"}),
		}},
		{Text: "read all three"},
	}}

	if _, err := loop(t, p, toolexec.NewRegistry().Add(tool), agentloop.NoHooks{}).
		Run(context.Background(), state()); err != nil {
		t.Fatal(err)
	}
	if got := tool.peak.Load(); got < 2 {
		t.Fatalf("the highest number of tools running at once was %d", got)
	}
	if len(tool.inputs()) != 3 {
		t.Fatalf("%d of three calls ran", len(tool.inputs()))
	}
}

// TestTheParallelCeilingIsRespected.
func TestTheParallelCeilingIsRespected(t *testing.T) {
	tool := &recording{name: "Read"}
	var calls []agentloop.ToolCall
	for i := range 8 {
		calls = append(calls, fake.Call(string(rune('a'+i)), "Read", map[string]any{"file_path": "x"}))
	}
	p := &fake.Provider{Script: []fake.Step{{Calls: calls}, {Text: "done"}}}

	l := agentloop.New(agentloop.Deps{
		Provider: p, Tools: toolexec.NewRegistry().Add(tool), Hooks: agentloop.NoHooks{},
		Clock: clockx.Fixed{At: refTime}, Log: quiet(),
		Limits: agentloop.Limits{MaxToolsPerStep: 2},
	})
	if _, err := l.Run(context.Background(), state()); err != nil {
		t.Fatal(err)
	}
	if got := tool.peak.Load(); got > 2 {
		t.Fatalf("%d tools ran at once with a ceiling of 2", got)
	}
}

// TestTheResultsKeepTheOrderTheModelAskedIn, whatever order they finished in.
func TestTheResultsKeepTheOrderTheModelAskedIn(t *testing.T) {
	slow := &recording{name: "Slow", fn: func(json.RawMessage) (any, error) {
		time.Sleep(50 * time.Millisecond)
		return "slow", nil
	}}
	quick := &recording{name: "Quick", fn: func(json.RawMessage) (any, error) {
		return "quick", nil
	}}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{
			fake.Call("c1", "Slow", nil),
			fake.Call("c2", "Quick", nil),
		}},
		{Text: "done"},
	}}

	res, err := loop(t, p, toolexec.NewRegistry().Add(slow, quick), agentloop.NoHooks{}).
		Run(context.Background(), state())
	if err != nil {
		t.Fatal(err)
	}
	if res.ToolCalls[0].Name != "Slow" || res.ToolCalls[1].Name != "Quick" {
		t.Fatalf("results came back in finishing order: %+v", res.ToolCalls)
	}
}

// TestAToolErrorIsAResultAndTheModelGetsToRespond. Two of them are what the
// Two-Strike rule in the master prompt turns into a change of approach.
func TestAToolErrorIsAResultAndTheModelGetsToRespond(t *testing.T) {
	broken := &recording{name: "Read", fn: func(json.RawMessage) (any, error) {
		return nil, errors.New("no such file or directory")
	}}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Read", map[string]any{"file_path": "gone"})}},
		{Text: "that file does not exist; I will look for it"},
	}}

	res, err := loop(t, p, toolexec.NewRegistry().Add(broken), agentloop.NoHooks{}).
		Run(context.Background(), state())
	if err != nil {
		t.Fatalf("a tool error aborted the turn: %v", err)
	}
	if res.ToolCalls[0].Error == "" || res.ToolCalls[0].Denied {
		t.Fatalf("a failure was recorded as a denial: %+v", res.ToolCalls[0])
	}
}

// TestTheNineEventsFireWhereTheySay. The table in the specification, checked
// against a turn that touches all of them.
func TestTheNineEventsFireWhereTheySay(t *testing.T) {
	failing := &recording{name: "Broken", fn: func(json.RawMessage) (any, error) {
		return nil, errors.New("it broke")
	}}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Read", nil)}},
		{Calls: []agentloop.ToolCall{fake.Call("c2", "Broken", nil)}},
		{Text: "finished"},
	}}

	var seen []event.Type
	var mu sync.Mutex
	watch := func(types ...event.Type) event.Handler {
		return event.FuncHandler{
			Name: "watch", Events: types,
			Fn: func(_ context.Context, e event.Event) (event.Outcome, error) {
				mu.Lock()
				seen = append(seen, e.Type)
				mu.Unlock()
				return event.Outcome{}, nil
			},
		}
	}
	b, _ := bus(t, watch(event.Types...))

	if _, err := loop(t, p, toolexec.NewRegistry().Add(&recording{name: "Read"}, failing),
		&agentloop.EventHooks{Bus: b}).Run(context.Background(), state()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []event.Type{
		event.SessionStart, event.UserPromptSubmit,
		event.PreToolUse, event.PostToolUse,
		event.PreToolUse, event.PostToolUseFailure,
		event.Stop,
	}
	if len(seen) != len(want) {
		t.Fatalf("events = %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("event %d = %s, want %s (all: %v)", i, seen[i], want[i], seen)
		}
	}
}

// TestABlockingStopHookSaysYouAreNotFinished.
func TestABlockingStopHookSaysYouAreNotFinished(t *testing.T) {
	b, _ := bus(t, hook("reviewer", event.Stop, func(event.Event) (event.Outcome, error) {
		return event.Outcome{Decision: event.DecisionBlock, Reason: "the tests were not run"}, nil
	}))
	_, err := loop(t, fake.Text("all done"), toolexec.NewRegistry(), &agentloop.EventHooks{Bus: b}).
		Run(context.Background(), state())
	if err == nil {
		t.Fatal("the turn ended")
	}
	if !strings.Contains(err.Error(), "the tests were not run") {
		t.Fatalf("err = %v", err)
	}
}

// TestAHookInjectsContextTheNextCallCarries.
func TestAHookInjectsContextTheNextCallCarries(t *testing.T) {
	p := &fake.Provider{Script: []fake.Step{{Text: "understood"}}}
	b, _ := bus(t, hook("policy", event.UserPromptSubmit, func(event.Event) (event.Outcome, error) {
		return event.Outcome{AdditionalContext: "the release is frozen until Monday"}, nil
	}))

	if _, err := loop(t, p, toolexec.NewRegistry(), &agentloop.EventHooks{Bus: b}).
		Run(context.Background(), state()); err != nil {
		t.Fatal(err)
	}
	first := p.Requests()[0]
	var found bool
	for _, m := range first.Messages {
		if strings.Contains(m.Text, "frozen until Monday") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the injected context did not reach the model: %+v", first.Messages)
	}
	// It goes in as a turn, not as policy: the context document declares the
	// authority of every block it holds, and spliced text would inherit one.
	if strings.Contains(first.Instructions, "frozen until Monday") {
		t.Error("injected context was spliced into the trusted instructions")
	}
}

// TestAModelThatNeverStopsHitsTheCeiling, and the error says what it cost.
func TestAModelThatNeverStopsHitsTheCeiling(t *testing.T) {
	var script []fake.Step
	for range 20 {
		script = append(script, fake.Step{
			Calls: []agentloop.ToolCall{fake.Call("c", "Read", nil)},
			Usage: agentloop.Usage{Total: 1000},
		})
	}
	l := agentloop.New(agentloop.Deps{
		Provider: &fake.Provider{Script: script},
		Tools:    toolexec.NewRegistry().Add(&recording{name: "Read"}),
		Hooks:    agentloop.NoHooks{}, Clock: clockx.Fixed{At: refTime}, Log: quiet(),
		Limits: agentloop.Limits{MaxSteps: 3},
	})

	_, err := l.Run(context.Background(), state())
	if err == nil {
		t.Fatal("the loop ran forever")
	}
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_AGENT_STEPS_EXHAUSTED" {
		t.Fatalf("err = %v", err)
	}
	if app.Issues["tokens"] != 3000 {
		t.Errorf("the error does not say what the runaway cost: %+v", app.Issues)
	}
}

// TestCancellingTheTurnStopsIt.
func TestCancellingTheTurnStopsIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blocking := &recording{name: "Read", fn: func(json.RawMessage) (any, error) {
		cancel()
		time.Sleep(20 * time.Millisecond)
		return "x", nil
	}}
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Read", nil)}},
		{Text: "should not get here"},
	}}

	_, err := loop(t, p, toolexec.NewRegistry().Add(blocking), agentloop.NoHooks{}).Run(ctx, state())
	if err == nil {
		t.Fatal("a cancelled turn finished")
	}
}

// TestAProviderOutageIsReportedAsOne, with a status that says whose fault it is.
func TestAProviderOutageIsReportedAsOne(t *testing.T) {
	p := &fake.Provider{Script: []fake.Step{{Err: errors.New("429 rate limited")}}}
	_, err := loop(t, p, toolexec.NewRegistry(), agentloop.NoHooks{}).Run(context.Background(), state())
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_AGENT_PROVIDER_FAILED" {
		t.Fatalf("err = %v", err)
	}
	if app.HTTPStatus != apperr.StatusBadGateway {
		t.Errorf("status = %d, want 502 — the failure is upstream", app.HTTPStatus)
	}
}

// TestATurnWithNoProviderSaysSo.
func TestATurnWithNoProviderSaysSo(t *testing.T) {
	l := agentloop.New(agentloop.Deps{Clock: clockx.Fixed{At: refTime}, Log: quiet()})
	if _, err := l.Run(context.Background(), state()); err == nil {
		t.Fatal("a turn ran with no provider")
	}
}

// TestAnUnknownToolIsAResultTheModelCanRecoverFrom.
func TestAnUnknownToolIsAResultTheModelCanRecoverFrom(t *testing.T) {
	p := &fake.Provider{Script: []fake.Step{
		{Calls: []agentloop.ToolCall{fake.Call("c1", "Imagined", nil)}},
		{Text: "that tool does not exist; using Read instead"},
	}}
	res, err := loop(t, p, toolexec.NewRegistry().Add(&recording{name: "Read"}), agentloop.NoHooks{}).
		Run(context.Background(), state())
	if err != nil {
		t.Fatalf("an invented tool name aborted the turn: %v", err)
	}
	if !strings.Contains(res.ToolCalls[0].Error, "TOOL_UNKNOWN") {
		t.Fatalf("result = %+v", res.ToolCalls[0])
	}
}

// TestTheAnswerIsStreamedAndTheChunksAddUp. A stream whose pieces do not
// reproduce the answer shows the user something other than what was said.
func TestTheAnswerIsStreamedAndTheChunksAddUp(t *testing.T) {
	var b strings.Builder
	var reasoning strings.Builder
	l := agentloop.New(agentloop.Deps{
		Provider: &fake.Provider{Script: []fake.Step{
			{Text: "the readme says hello", Reasoning: "checking the file first"},
		}},
		Tools: toolexec.NewRegistry(), Hooks: agentloop.NoHooks{},
		Clock: clockx.Fixed{At: refTime}, Log: quiet(),
		Emitter: emitter(func(c agentloop.Chunk) {
			b.WriteString(c.Text)
			reasoning.WriteString(c.Reasoning)
		}),
	})

	res, err := l.Run(context.Background(), state())
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != "the readme says hello" {
		t.Fatalf("the chunks add up to %q", b.String())
	}
	if reasoning.String() != "checking the file first" {
		t.Errorf("reasoning = %q", reasoning.String())
	}
	if res.Text != "the readme says hello" {
		t.Errorf("the final answer disagrees with the stream: %q", res.Text)
	}
}

type emitter func(agentloop.Chunk)

func (e emitter) Delta(_ context.Context, c agentloop.Chunk) { e(c) }

type answers struct {
	approved bool
	reason   string
	updated  string
}

func (a answers) RequestApproval(context.Context, event.ApprovalRequest) (event.ApprovalResult, error) {
	res := event.ApprovalResult{Approved: a.approved, Reason: a.reason}
	if a.updated != "" {
		res.UpdatedInput = json.RawMessage(a.updated)
	}
	return res, nil
}
