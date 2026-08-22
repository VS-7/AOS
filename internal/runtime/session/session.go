// Package session is what turns a message in a conversation into a turn of an
// agent, and the answer back into a message.
//
// It is the layer where the pieces meet: the agent's file becomes a sandbox and
// a model, the workspace becomes a context document, the command registry
// becomes a tool list, and the result becomes something a person can read
// tomorrow. Everything it does is composition; the behaviour lives in the
// packages it composes.
package session

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/safe"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/execguard"
	"github.com/OWNER/aos/internal/runtime/prompt"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/sandbox"
	"github.com/OWNER/aos/internal/runtime/subconscious"
	"github.com/OWNER/aos/internal/runtime/toolexec"
	"github.com/OWNER/aos/internal/runtime/toolexec/tools"
)

// Agents is what this package needs from the agent aggregate.
type Agents interface {
	Get(ctx context.Context, in agent.GetInput) (*agent.Agent, error)
}

// Chats is what it needs from the conversation aggregate.
type Chats interface {
	Get(ctx context.Context, in chat.GetInput) (*chat.Chat, error)
	Reply(ctx context.Context, in chat.ReplyInput) (chat.ReplyOutput, error)
}

// Models resolves an agent to a provider, so the composition root owns the
// configuration and this package owns nothing.
type Models interface {
	For(ctx context.Context, a *agent.Agent) (agentloop.LLMProvider, agentloop.ModelRef, error)
}

// Deps is what the runner is built from.
type Deps struct {
	Agents   Agents
	Chats    Chats
	Models   Models
	Registry *command.Registry
	Bus      *event.Service
	Approver event.Approver
	Prompt   *prompt.Assembler
	Spiller  *toolexec.Spiller
	Events   Publisher
	Clock    clockx.Clock
	IDs      interface{ New() string }
	Log      *slog.Logger

	// WorkspaceRoot is the directory an agent is confined to when its task
	// does not put it in a worktree of its own.
	WorkspaceRoot string

	// WorkspaceID names the workspace in the assembled document and in the
	// events published for it.
	WorkspaceID string

	// TmpDir is the spillover directory, readable by the sandbox and never
	// writable.
	TmpDir string

	// Limits override the loop's ceilings.
	Limits agentloop.Limits
}

// Publisher is how a turn reaches whoever is watching it happen.
type Publisher interface {
	ChatDelta(ctx context.Context, workspace, chatID string, text, reasoning string)
	ChatDone(ctx context.Context, workspace, chatID string, usage chat.TokenUsage)
}

// Observer is the background cognitive layer, fired when a turn ends.
//
// It is an interface here rather than the concrete observer so that the session
// package does not depend on the subconscious: the turn's contract is "somebody
// may want to know this happened", and what that somebody does with it is not
// the turn's business.
type Observer interface {
	Schedule(ctx context.Context, in subconscious.Input)
}

// Runner executes turns.
type Runner struct {
	deps     Deps
	log      *slog.Logger
	observer Observer
}

// SetObserver attaches the background observer after both exist.
//
// The observer resolves its own model through the same configuration the runner
// reads, so one of the two has to be handed to the other once both are built.
// It is set once at boot, before a turn can arrive.
func (r *Runner) SetObserver(o Observer) { r.observer = o }

// New wires the runner.
func New(d Deps) *Runner {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	if d.Clock == nil {
		d.Clock = clockx.System{}
	}
	return &Runner{deps: d, log: log}
}

// Dispatch starts a turn and returns immediately.
//
// It is the chat aggregate's port: a message is persisted first and dispatched
// second, so the turn is work that outlives the request that asked for it. The
// context is detached for the same reason — a person closing a browser tab is
// not a person cancelling the agent.
func (r *Runner) Dispatch(ctx context.Context, in chat.Turn) (string, error) {
	jobID := r.newID()
	detached := context.WithoutCancel(ctx)

	_ = safe.Go(detached, "turn "+jobID, func(c context.Context) error {
		if _, err := r.Run(c, in); err != nil {
			r.log.Error("the turn failed",
				"chat", in.ChatID, "agent", in.AgentID, "job", jobID, "err", err)
		}
		return nil
	})
	return jobID, nil
}

// Run executes one turn and persists the answer.
//
// It is exported and synchronous because that is what makes the delivery
// provable: a test can run a turn and assert on what came out of it, rather
// than on a goroutine it has to wait for.
func (r *Runner) Run(ctx context.Context, in chat.Turn) (*agentloop.Result, error) {
	conversation, err := r.deps.Chats.Get(ctx, chat.GetInput{Chat: in.ChatID})
	if err != nil {
		return nil, err
	}
	worker, err := r.deps.Agents.Get(ctx, agent.GetInput{ID: in.AgentID})
	if err != nil {
		return nil, err
	}

	provider, model, err := r.deps.Models.For(ctx, worker)
	if err != nil {
		return nil, err
	}

	// From here the ambient actor is the agent, not whoever sent the message.
	// It is what makes a memory the agent stores belong to the agent, and what
	// puts its name on every record the turn writes.
	ctx = identity.With(ctx, identity.Identity{
		WorkspaceID: r.deps.WorkspaceID,
		AgentID:     worker.ID,
		RequestID:   identity.From(ctx).RequestID,
	})

	box, err := r.sandboxFor(worker)
	if err != nil {
		return nil, err
	}
	registry := r.toolsFor(box)

	// A cli toolset's Call needs the calling agent's own sandbox to clear
	// before it runs anything — the second of the two doors
	// internal/domain/toolset's decision doc requires closed. box is built
	// fresh per turn, above, so it is attached here rather than at
	// composition root: internal/adapters/cliclient reads it back off ctx
	// through internal/runtime/execguard, three layers away, without either
	// package importing the other.
	ctx = execguard.With(ctx, box)

	instructions, err := r.deps.Prompt.Assemble(ctx, prompt.AssembleInput{
		Agent: prompt.AgentRef{
			ID: worker.ID, Name: worker.DisplayName(), Role: worker.Role,
			Instructions: worker.Content, Orchestrator: worker.Orchestrator,
		},
		Workspace:         r.deps.WorkspaceID,
		SessionStartedAt:  conversation.CreatedAt,
		LastUserMessageAt: lastUserAt(conversation),
	})
	if err != nil {
		return nil, err
	}

	state := &agentloop.State{
		SessionID:    conversation.ID,
		AgentID:      worker.ID,
		Workspace:    r.deps.WorkspaceID,
		Instructions: instructions,
		Messages:     transcript(conversation),
		Tools:        registry.Specs(),
		Model:        model.Model,
		Reasoning:    model.Reasoning,
	}

	loop := agentloop.New(agentloop.Deps{
		Provider: provider,
		Tools:    registry,
		Hooks: &agentloop.EventHooks{
			Bus:       r.deps.Bus,
			Approver:  r.deps.Approver,
			Risk:      agentloop.RiskFromRegistry(registry),
			Directory: box.Root(),
		},
		Clock:   r.deps.Clock,
		Limits:  r.deps.Limits,
		Emitter: r.emitter(conversation.ID),
		Log:     r.log,
	})

	started := r.deps.Clock.Now()
	result, runErr := loop.Run(ctx, state)
	if runErr != nil {
		r.recordFailure(ctx, in, worker.ID, started, runErr)
		return nil, runErr
	}
	// Priced here, not inside agentloop: the loop talks to LLMProvider, not to
	// the pricing table, and internal/runtime/providers already imports
	// agentloop for the interface it builds — the reverse import would cycle.
	result.Usage.CostUSD = providers.CostUSD(model.Provider, model.Model, result.Usage)

	if err := r.persist(ctx, in, worker.ID, started, result); err != nil {
		return result, err
	}
	if r.deps.Events != nil {
		r.deps.Events.ChatDone(ctx, r.deps.WorkspaceID, conversation.ID, usageOf(result.Usage))
	}
	r.observe(ctx, worker, conversation.ID, state)
	return result, nil
}

// observe hands the finished turn to the background layer.
//
// It is the last thing Run does and it does not wait: the observation is fired
// on a detached context with its own timeout, so a slow or failing observer
// never delays the answer somebody is reading. Losing an observation is a
// warning; losing the answer would not be.
func (r *Runner) observe(ctx context.Context, worker *agent.Agent, sessionID string, state *agentloop.State) {
	if r.observer == nil {
		return
	}
	r.observer.Schedule(ctx, subconscious.Input{
		AgentID:   worker.ID,
		AgentName: worker.DisplayName(),
		SessionID: sessionID,
		Workspace: r.deps.WorkspaceID,
		Messages:  state.Messages,
	})
}

// sandboxFor builds the confinement from the agent's own file.
func (r *Runner) sandboxFor(a *agent.Agent) (*sandbox.Sandbox, error) {
	opts := sandbox.Options{
		WorkspacePath: r.deps.WorkspaceRoot,
		TmpDir:        r.deps.TmpDir,
		Permissions:   sandbox.DefaultPermissions(),
		Exec:          sandbox.DefaultExecPolicy(),
	}
	if a.Sandbox != nil {
		if len(a.Sandbox.Permissions) > 0 {
			opts.Permissions = sandbox.PermissionsFrom(a.Sandbox.Permissions)
		}
		if e := a.Sandbox.Exec; e != nil {
			opts.Exec = sandbox.ExecPolicy{
				Policy: e.Policy, Allow: e.Allow,
				DenyArgs: e.DenyArgs, AllowShell: e.AllowShell,
			}
		}
	}
	return sandbox.New(opts)
}

// toolsFor is the tool list one agent sees: the native filesystem toolset over
// its own sandbox, plus every domain command that belongs in an agent's reach.
//
// The registry is built per turn rather than once, because two agents have
// different sandboxes and a tool bound to one must not be callable by another.
func (r *Runner) toolsFor(box *sandbox.Sandbox) *toolexec.Registry {
	registry := toolexec.NewRegistry()
	wrap := func(t toolexec.Tool) toolexec.Tool {
		return toolexec.Wrap(t,
			toolexec.WithSpill(r.deps.Spiller),
			toolexec.WithClock(r.deps.Clock.Now),
		)
	}
	for _, t := range tools.FS(box) {
		registry.Add(wrap(t))
	}
	if r.deps.Registry != nil {
		for _, d := range r.deps.Registry.AgentTools() {
			registry.Add(wrap(toolexec.FromCommand(d)))
		}
	}
	return registry
}

func (r *Runner) emitter(chatID string) agentloop.Emitter {
	if r.deps.Events == nil {
		return nil
	}
	return emitterFunc(func(ctx context.Context, c agentloop.Chunk) {
		r.deps.Events.ChatDelta(ctx, r.deps.WorkspaceID, chatID, c.Text, c.Reasoning)
	})
}

type emitterFunc func(context.Context, agentloop.Chunk)

func (f emitterFunc) Delta(ctx context.Context, c agentloop.Chunk) { f(ctx, c) }

// persist writes the answer back into the conversation.
//
// Everything the turn did goes in: the text, the reasoning, and every tool call
// with what it returned. A transcript that shows only the answer cannot answer
// "why did it do that", which is the question people actually ask.
func (r *Runner) persist(ctx context.Context, in chat.Turn, agentID string, started time.Time, result *agentloop.Result) error {
	parts := []chat.Part{}
	if result.Text != "" {
		parts = append(parts, chat.Part{Type: chat.PartText, Text: result.Text})
	}
	for _, m := range result.Messages {
		for _, c := range m.ToolCalls {
			parts = append(parts, chat.Part{
				Type: chat.PartToolCall, ToolName: c.Name, ToolCallID: c.ID, Input: c.Input,
			})
		}
	}
	for _, c := range result.ToolCalls {
		parts = append(parts, chat.Part{
			Type: chat.PartToolResult, ToolName: c.Name, ToolCallID: c.CallID, Output: c.Output,
		})
	}

	_, err := r.deps.Chats.Reply(ctx, chat.ReplyInput{
		Chat:      in.ChatID,
		ReplyTo:   in.MessageID,
		AgentID:   agentID,
		Parts:     parts,
		Usage:     usageOf(result.Usage),
		StartedAt: started,
	})
	return err
}

// recordFailure writes the failure onto the message that asked for it, so that
// a turn that did not answer is visible in the conversation rather than only in
// a log the person cannot see.
func (r *Runner) recordFailure(ctx context.Context, in chat.Turn, agentID string, started time.Time, cause error) {
	code, message := "AOS_AGENT_TURN_FAILED", cause.Error()
	var app *apperr.Error
	if errors.As(cause, &app) {
		code, message = app.Code, app.Message
	}
	if _, err := r.deps.Chats.Reply(ctx, chat.ReplyInput{
		Chat: in.ChatID, ReplyTo: in.MessageID, AgentID: agentID,
		StartedAt: started,
		Failure:   &chat.RunError{Code: code, Message: message},
	}); err != nil {
		r.log.Error("the failure of a turn could not be recorded",
			"chat", in.ChatID, "err", err)
	}
}

// transcript turns a stored conversation into the messages a model reads.
func transcript(c *chat.Chat) []agentloop.Message {
	out := make([]agentloop.Message, 0, len(c.Messages))
	for _, m := range c.Messages {
		switch m.Role {
		case chat.RoleUser:
			if text := m.Text(); text != "" {
				out = append(out, agentloop.Message{Role: agentloop.RoleUser, Text: text, At: m.CreatedAt})
			}

		case chat.RoleAssistant:
			msg := agentloop.Message{Role: agentloop.RoleAssistant, At: m.CreatedAt}
			var results []agentloop.Message
			for _, p := range m.Parts {
				switch p.Type {
				case chat.PartText:
					msg.Text += p.Text
				case chat.PartReasoning:
					msg.Reasoning += p.Text
				case chat.PartToolCall:
					msg.ToolCalls = append(msg.ToolCalls, agentloop.ToolCall{
						ID: p.ToolCallID, Name: p.ToolName, Input: p.Input,
					})
				case chat.PartToolResult:
					results = append(results, agentloop.Message{
						Role: agentloop.RoleTool, CallID: p.ToolCallID,
						Name: p.ToolName, Result: p.Output, At: m.CreatedAt,
					})
				}
			}
			if msg.Text != "" || len(msg.ToolCalls) > 0 {
				out = append(out, msg)
			}
			// The results follow the call that produced them, which is the
			// order every provider requires.
			out = append(out, results...)
		}
	}
	return out
}

func lastUserAt(c *chat.Chat) time.Time {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Role == chat.RoleUser {
			return c.Messages[i].CreatedAt
		}
	}
	return time.Time{}
}

func usageOf(u agentloop.Usage) chat.TokenUsage {
	return chat.TokenUsage{
		Input: u.Input, Output: u.Output, Reasoning: u.Reasoning,
		Cached: u.Cached, Total: u.Total, CostUSD: u.CostUSD,
	}
}

func (r *Runner) newID() string {
	if r.deps.IDs == nil {
		return "job"
	}
	return r.deps.IDs.New()
}
