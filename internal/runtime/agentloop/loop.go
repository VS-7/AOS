package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// Limits are the ceilings on one turn.
//
// The original has stopWhen and no explicit numbers. An agent loop without a
// ceiling is a cost incident waiting for the first prompt that makes a model
// call the same tool forever, and the ceiling has to be somewhere a person can
// read it.
type Limits struct {
	MaxSteps        int
	MaxToolsPerStep int
	StepTimeout     time.Duration
	TotalTimeout    time.Duration
}

// DefaultLimits are forty steps, four parallel tools, five minutes per model
// call and thirty minutes per turn.
func DefaultLimits() Limits {
	return Limits{
		MaxSteps:        40,
		MaxToolsPerStep: 4,
		StepTimeout:     5 * time.Minute,
		TotalTimeout:    30 * time.Minute,
	}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxSteps <= 0 {
		l.MaxSteps = d.MaxSteps
	}
	if l.MaxToolsPerStep <= 0 {
		l.MaxToolsPerStep = d.MaxToolsPerStep
	}
	if l.StepTimeout <= 0 {
		l.StepTimeout = d.StepTimeout
	}
	if l.TotalTimeout <= 0 {
		l.TotalTimeout = d.TotalTimeout
	}
	return l
}

// PreCompacter is the optional hook fired before the history is pruned.
// EventHooks implements it; NoHooks does not, and the loop asks with a type
// assertion rather than making every implementation carry a method.
type PreCompacter interface {
	PreCompact(ctx context.Context, s *State) error
}

// Deps is what the loop is built from.
type Deps struct {
	Provider LLMProvider
	Tools    *toolexec.Registry
	Hooks    Hooks
	Compact  *Compactor
	Clock    clockx.Clock
	Limits   Limits
	Emitter  Emitter
	Log      *slog.Logger
}

// Loop runs one turn.
type Loop struct {
	provider LLMProvider
	tools    *toolexec.Registry
	hooks    Hooks
	compact  *Compactor
	clock    clockx.Clock
	limits   Limits
	emitter  Emitter
	log      *slog.Logger
}

// New wires the loop.
func New(d Deps) *Loop {
	hooks := d.Hooks
	if hooks == nil {
		hooks = NoHooks{}
	}
	compact := d.Compact
	if compact == nil {
		compact = NewCompactor()
	}
	clock := d.Clock
	if clock == nil {
		clock = clockx.System{}
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Loop{
		provider: d.Provider, tools: d.Tools, hooks: hooks, compact: compact,
		clock: clock, limits: d.Limits.withDefaults(), emitter: d.Emitter, log: log,
	}
}

// Run executes one turn: a model call, the tools it asked for, and again until
// it stops asking or a limit is reached.
func (l *Loop) Run(ctx context.Context, s *State) (*Result, error) {
	if l.provider == nil {
		return nil, errNoProvider()
	}
	started := l.clock.Now()
	s.Started = started

	ctx, cancel := context.WithTimeout(ctx, l.limits.TotalTimeout)
	defer cancel()

	// A blocking UserPromptSubmit hook ends the turn here, before a token is
	// spent. That is the point of the hook being at this position.
	if err := l.hooks.PrepareCall(ctx, s); err != nil {
		return nil, err
	}

	var calls []ToolResult
	stop := StopEnd

	for s.Steps < l.limits.MaxSteps {
		// The loop checks cancellation itself rather than trusting the
		// provider to notice. A turn cancelled while a tool was running would
		// otherwise make one more model call — paid for, and thrown away.
		if err := ctx.Err(); err != nil {
			return nil, errTurnCancelled(s.Steps, s.Usage, err)
		}
		if err := l.hooks.PrepareStep(ctx, s); err != nil {
			return nil, err
		}
		if l.compact.ShouldCompact(s.Chars()) {
			if err := l.compactNow(ctx, s); err != nil {
				return nil, err
			}
		}

		resp, err := l.call(ctx, s)
		if err != nil {
			return nil, err
		}
		s.Steps++
		s.Usage.Add(resp.Usage)
		s.DrainPending()
		s.Append(resp.Message)

		if len(resp.ToolCalls) == 0 {
			stop = resp.StopReason
			if stop == "" {
				stop = StopEnd
			}
			break
		}

		results, err := l.runTools(ctx, s, resp.ToolCalls)
		if err != nil {
			return nil, err
		}
		calls = append(calls, results...)
		s.AppendToolResults(results, l.clock.Now())

		if s.Steps >= l.limits.MaxSteps {
			// The loop stops here rather than after another model call, so the
			// error names the step that hit the ceiling.
			return nil, errStepsExhausted(l.limits.MaxSteps, s.Usage)
		}
	}

	if err := l.hooks.OnEnd(ctx, s); err != nil {
		return nil, err
	}

	return &Result{
		Text:        lastAssistantText(s.Messages),
		Messages:    s.Messages,
		Usage:       s.Usage,
		Steps:       s.Steps,
		Compactions: s.Compactions,
		StopReason:  stop,
		ToolCalls:   calls,
		Duration:    l.clock.Now().Sub(started),
	}, nil
}

// call makes one model call, streaming when somebody is watching.
func (l *Loop) call(ctx context.Context, s *State) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, l.limits.StepTimeout)
	defer cancel()

	req := s.Request()
	if l.emitter == nil {
		resp, err := l.provider.Generate(ctx, req)
		if err != nil {
			return Response{}, errProviderFailed(l.provider.Name(), err)
		}
		return resp, nil
	}

	stream, err := l.provider.Stream(ctx, req)
	if err != nil {
		return Response{}, errProviderFailed(l.provider.Name(), err)
	}
	defer func() { _ = stream.Close() }()

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Response{}, errProviderFailed(l.provider.Name(), err)
		}
		l.emitter.Delta(ctx, chunk)
	}
	return stream.Response(), nil
}

// compactNow prunes the history, after giving the hooks a chance to say what
// must survive it.
func (l *Loop) compactNow(ctx context.Context, s *State) error {
	if pc, ok := l.hooks.(PreCompacter); ok {
		if err := pc.PreCompact(ctx, s); err != nil {
			return err
		}
	}
	before := len(s.Messages)
	s.Messages = Prune(s.Messages, l.compact.Policy)
	s.Compactions++
	l.log.Info("pruned the history to fit the context",
		"session", s.SessionID, "before", before, "after", len(s.Messages))
	return nil
}

// runTools executes the calls of one step.
//
// Concurrently, because the master prompt tells the agent to call independent
// tools in parallel and a runtime that then serialises them makes that
// instruction a lie. Dependent calls arrive in separate steps by construction:
// the model cannot ask for a tool whose input depends on a result it has not
// seen.
func (l *Loop) runTools(ctx context.Context, s *State, calls []ToolCall) ([]ToolResult, error) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(l.limits.MaxToolsPerStep)

	results := make([]ToolResult, len(calls))
	var mu sync.Mutex

	for i, c := range calls {
		g.Go(func() error {
			res, err := l.runOne(gctx, s, c, &mu)
			if err != nil {
				return err
			}
			results[i] = res
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// runOne approves, executes and reports one call.
//
// The mutex guards the state, which the hooks write to. It is held only around
// the hook calls rather than around the tool, so two tools still run at once.
func (l *Loop) runOne(ctx context.Context, s *State, c ToolCall, mu *sync.Mutex) (ToolResult, error) {
	mu.Lock()
	decision, err := l.hooks.ApproveTool(ctx, &c, s)
	mu.Unlock()
	if err != nil {
		return ToolResult{}, err
	}

	if !decision.Allow {
		// A denial is a result, not a failure. The model has to see it to
		// reason about it, and propagating it as an error would abort the turn
		// over a decision that was made on purpose.
		return ToolResult{
			CallID: c.ID, Name: c.Name, Denied: true,
			Error: reasonOr(decision.Reason, "the call was not allowed"),
		}, nil
	}
	if len(decision.UpdatedInput) > 0 {
		c.Input = decision.UpdatedInput
	}
	if decision.AddContext != "" {
		mu.Lock()
		s.AppendContext(decision.AddContext)
		mu.Unlock()
	}

	result := l.invoke(ctx, c)

	mu.Lock()
	err = l.hooks.AfterTool(ctx, &c, &result, s)
	mu.Unlock()
	if err != nil {
		return ToolResult{}, err
	}
	return result, nil
}

func (l *Loop) invoke(ctx context.Context, c ToolCall) ToolResult {
	out := ToolResult{CallID: c.ID, Name: c.Name}
	if l.tools == nil {
		out.Error = "no tools are available in this turn"
		return out
	}
	value, err := l.tools.Invoke(toolexec.WithCallID(ctx, c.ID), c.Name, c.Input)
	if err != nil {
		// A tool error is a result too: the model reads it, and the Two-Strike
		// rule in the master prompt is what turns two of them into a change of
		// approach rather than a third attempt.
		out.Error = err.Error()
		out.Output = encode(err)
		return out
	}
	out.Output = encode(value)
	return out
}

func encode(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		if e, ok := v.(error); ok {
			raw, _ = json.Marshal(map[string]string{"error": e.Error()})
			return raw
		}
		return json.RawMessage(`{"error":"the tool result could not be encoded"}`)
	}
	return raw
}

func reasonOr(reason, fallback string) string {
	if reason == "" {
		return fallback
	}
	return reason
}

func lastAssistantText(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleAssistant && messages[i].Text != "" {
			return messages[i].Text
		}
	}
	return ""
}
