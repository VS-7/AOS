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
	"strings"
	"sync"
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

// Bots pushes a turn's answer back out to the external channel its
// conversation is bound to. Nil means this build has no channel provider
// wired at all — legitimate, not a Deliver call skipped by accident; see
// deliverToChannel's own doc comment.
type Bots interface {
	Deliver(ctx context.Context, provider, agentID, chatID, text string) error
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
	Bots     Bots
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
	// ChatStarted says an agent has picked up a conversation, before it has
	// produced anything. It is what puts "Atlas is working…" on screen
	// during the wait for a first token, which on a reasoning model is most
	// of the turn.
	ChatStarted(ctx context.Context, workspace, chatID, agentID string)

	ChatDelta(ctx context.Context, workspace, chatID string, text, reasoning string)

	// ChatDone carries the agent as well as the usage: the interface has to
	// know *whose* work ended to take that agent off the conversation, and
	// a turn that ended without saying so leaves it working forever.
	ChatDone(ctx context.Context, workspace, chatID, agentID string, usage chat.TokenUsage)

	// ChatMessage publishes the answer as it is being written: the same
	// message shape the transcript stores, carrying everything the turn has
	// produced so far.
	//
	// ChatDelta carries text and nothing else, which is only part of what a
	// turn does — a tool call is often the slowest and most interesting
	// stretch of one, and it never appeared until the turn ended. A snapshot
	// carries all of it, and lets the interface render an in-progress answer
	// with exactly the component that renders a finished one.
	ChatMessage(ctx context.Context, workspace, chatID string, message chat.Message)
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
func (r *Runner) Run(ctx context.Context, in chat.Turn) (result *agentloop.Result, err error) {
	// The turn began here, not just before the model call: prompt assembly
	// and sandbox setup are part of how long somebody waited, and a turn that
	// fails before ever reaching the provider has to have a start time too.
	started := r.deps.Clock.Now()

	conversation, err := r.deps.Chats.Get(ctx, chat.GetInput{Chat: in.ChatID})
	if err != nil {
		// Nothing to record the failure against — the conversation itself is
		// what recordFailure would write to.
		return nil, err
	}

	// Every failure from here on is written onto the message that asked for
	// it, in one place rather than at each return.
	//
	// recordFailure used to be called from exactly one of them: the model
	// call. Everything before it — resolving the agent, resolving its model,
	// building the sandbox, assembling the prompt — returned bare, so those
	// turns left no trace anywhere a person could see. That is not an even
	// spread of risk: `Models.For` is where a fresh installation fails, every
	// time, with AOS_AGENT_PROVIDER_NOT_ENABLED until a model slot is set,
	// and it was the one failure guaranteed to be invisible. The message sat
	// in the conversation with no answer and no reason, and the only record
	// was a line in the daemon's log.
	agentID := in.AgentID
	if r.deps.Events != nil {
		r.deps.Events.ChatStarted(ctx, r.deps.WorkspaceID, conversation.ID, agentID)
	}
	defer func() {
		if err == nil {
			return
		}
		r.recordFailure(ctx, in, agentID, started, err)
		// Same signal a completed turn sends, so a window watching this
		// conversation refetches and shows the failure instead of waiting
		// for an answer that is never coming. `chat.done` means the turn
		// ended; it does not promise the turn succeeded.
		if r.deps.Events != nil {
			r.deps.Events.ChatDone(ctx, r.deps.WorkspaceID, conversation.ID, agentID, chat.TokenUsage{})
		}
	}()

	worker, err := r.deps.Agents.Get(ctx, agent.GetInput{ID: in.AgentID})
	if err != nil {
		return nil, err
	}
	agentID = worker.ID

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

	// The id the answer will be stored under, minted before the loop so the
	// snapshots published while it is written and the message finally
	// persisted are the same message — see chat.ReplyInput.MessageID.
	answerID := r.newID()

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
		Emitter: r.emitter(conversation.ID, answerID, worker.ID),
		Log:     r.log,
	})

	result, err = loop.Run(ctx, state)
	if err != nil {
		// Recorded by the deferred handler at the top, along with every
		// other way this turn can fail.
		return nil, err
	}
	// Priced here, not inside agentloop: the loop talks to LLMProvider, not to
	// the pricing table, and internal/runtime/providers already imports
	// agentloop for the interface it builds — the reverse import would cycle.
	result.Usage.CostUSD = providers.CostUSD(model.Provider, model.Model, result.Usage)

	if err = r.persist(ctx, in, worker.ID, answerID, started, result); err != nil {
		return result, err
	}
	if r.deps.Events != nil {
		r.deps.Events.ChatDone(ctx, r.deps.WorkspaceID, conversation.ID, worker.ID, usageOf(result.Usage))
	}
	r.deliverToChannel(ctx, conversation, worker.ID, result)
	r.observe(ctx, worker, conversation.ID, state)
	return result, nil
}

// deliverToChannel pushes a turn's answer back out to the external channel
// its conversation is bound to, when it has one — the Telegram side of a
// conversation a person can otherwise only see get answered from the
// desktop app. r.deps.Bots is nil in a build with no channel provider wired
// (bot.Registry.Deliver otherwise), conversation.Channel is nil for every
// chat that is not bound to one, and a turn that produced no text has
// nothing to send — any of the three is a normal reason to do nothing here,
// not an error.
//
// A delivery failure is logged and not returned: persist above already
// wrote the answer to the chat record, which is the fact this turn actually
// promises. Telegram not hearing back is a degraded experience for whoever
// is on that side of the conversation, not a failed turn — the same
// reasoning observe's own doc comment gives for not letting a slow
// subconscious delay or fail the answer.
func (r *Runner) deliverToChannel(ctx context.Context, conversation *chat.Chat, agentID string, result *agentloop.Result) {
	if r.deps.Bots == nil || conversation.Channel == nil || result.Text == "" {
		return
	}
	if err := r.deps.Bots.Deliver(ctx, conversation.Channel.Provider, agentID, conversation.Channel.ChatID, result.Text); err != nil {
		r.log.Warn("could not deliver the turn's answer to its external channel",
			"chat", conversation.ID, "provider", conversation.Channel.Provider, "err", err)
	}
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

func (r *Runner) emitter(chatID, messageID, agentID string) agentloop.Emitter {
	if r.deps.Events == nil {
		return nil
	}
	return &liveAnswer{
		runner: r, chatID: chatID, messageID: messageID, agentID: agentID,
		startedAt: r.deps.Clock.Now(),
	}
}

// snapshotInterval bounds how often a growing answer is republished.
//
// A streamed answer arrives a few tokens at a time; forwarding every one of
// them as a whole-message snapshot would spend more on the socket than on the
// model. Tool lifecycle ignores this and publishes immediately — those are
// rare, and each one is a visible change of what the agent is doing.
const snapshotInterval = 120 * time.Millisecond

// liveAnswer accumulates what a turn has produced and publishes it as the
// message the transcript will eventually hold.
//
// It carries the id the answer will be stored under (see chat.ReplyInput's
// MessageID) so that the in-progress message and the finished one are the
// same message, not two.
type liveAnswer struct {
	runner    *Runner
	chatID    string
	messageID string
	agentID   string
	startedAt time.Time

	mu       sync.Mutex
	text     strings.Builder
	calls    []chat.Part
	results  []chat.Part
	lastSent time.Time

	// reasoning is the block the current model call is writing; blocks holds
	// the ones the calls before it finished.
	//
	// One turn is several calls and each writes its own thought. Kept in a
	// single builder, as this was, they arrived glued end to end — the last
	// sentence of one running straight into the first word of the next — and
	// rendered as one wall of text.
	reasoning strings.Builder
	blocks    []string
}

// StepStarted closes the reasoning block the previous model call wrote.
// agentloop calls it before each new call; see agentloop.StepStarter.
func (l *liveAnswer) StepStarted(context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closeBlockLocked()
}

func (l *liveAnswer) closeBlockLocked() {
	if block := strings.TrimSpace(l.reasoning.String()); block != "" {
		l.blocks = append(l.blocks, block)
	}
	l.reasoning.Reset()
}

func (l *liveAnswer) Delta(ctx context.Context, c agentloop.Chunk) {
	// The text-only event stays: it is what a caller that only wants the
	// answer streaming (and not the whole message) still listens to.
	l.runner.deps.Events.ChatDelta(ctx, l.runner.deps.WorkspaceID, l.chatID, c.Text, c.Reasoning)

	l.mu.Lock()
	l.text.WriteString(c.Text)
	l.reasoning.WriteString(c.Reasoning)
	now := l.runner.deps.Clock.Now()
	due := now.Sub(l.lastSent) >= snapshotInterval
	if due {
		l.lastSent = now
	}
	snapshot := l.snapshotLocked()
	l.mu.Unlock()

	if due {
		l.publish(ctx, snapshot)
	}
}

func (l *liveAnswer) ToolStarted(ctx context.Context, call agentloop.ToolCall) {
	l.mu.Lock()
	l.calls = append(l.calls, chat.Part{
		Type: chat.PartToolCall, ToolName: call.Name, ToolCallID: call.ID, Input: call.Input,
	})
	l.lastSent = l.runner.deps.Clock.Now()
	snapshot := l.snapshotLocked()
	l.mu.Unlock()
	l.publish(ctx, snapshot)
}

func (l *liveAnswer) ToolFinished(ctx context.Context, result agentloop.ToolResult) {
	l.mu.Lock()
	l.results = append(l.results, chat.Part{
		Type: chat.PartToolResult, ToolName: result.Name, ToolCallID: result.CallID, Output: result.Output,
	})
	l.lastSent = l.runner.deps.Clock.Now()
	snapshot := l.snapshotLocked()
	l.mu.Unlock()
	l.publish(ctx, snapshot)
}

// snapshotLocked renders what has arrived so far, in the order persist writes
// it — so the live message and the stored one describe the same turn the same
// way, and the last snapshot is not visibly rearranged when the real one
// lands.
func (l *liveAnswer) snapshotLocked() chat.Message {
	parts := make([]chat.Part, 0, 2+len(l.blocks)+len(l.calls)+len(l.results))
	if text := l.text.String(); text != "" {
		parts = append(parts, chat.Part{Type: chat.PartText, Text: text})
	}
	// One part per block, so the interface renders one thinking step per model
	// call instead of a single run of text.
	for _, block := range l.blocks {
		parts = append(parts, chat.Part{Type: chat.PartReasoning, Text: block})
	}
	if reasoning := strings.TrimSpace(l.reasoning.String()); reasoning != "" {
		parts = append(parts, chat.Part{Type: chat.PartReasoning, Text: reasoning})
	}
	parts = append(parts, l.calls...)
	parts = append(parts, l.results...)

	return chat.Message{
		ID:        l.messageID,
		Role:      chat.RoleAssistant,
		Author:    &chat.Author{Type: chat.ActorAgent, ID: l.agentID},
		Parts:     parts,
		CreatedAt: l.startedAt,
	}
}

func (l *liveAnswer) publish(ctx context.Context, message chat.Message) {
	if len(message.Parts) == 0 {
		return
	}
	l.runner.deps.Events.ChatMessage(ctx, l.runner.deps.WorkspaceID, l.chatID, message)
}

// persist writes the answer back into the conversation.
//
// Everything the turn did goes in: the text, the reasoning, and every tool call
// with what it returned. A transcript that shows only the answer cannot answer
// "why did it do that", which is the question people actually ask.
// answerParts renders one turn's result as the parts of a stored message:
// what the agent said, the tools it called, and what they returned.
//
// Pure and separate from persist so the scoping rule below is testable
// without standing up the conversation aggregate.
func answerParts(result *agentloop.Result) []chat.Part {
	parts := []chat.Part{}
	if result.Text != "" {
		parts = append(parts, chat.Part{Type: chat.PartText, Text: result.Text})
	}

	// Only the calls this turn actually made.
	//
	// `result.Messages` is the loop's whole working transcript, and the loop
	// is seeded with the conversation so far — so walking it wrote every tool
	// call *ever made in this conversation* into this one answer, again, on
	// every turn. A chat that had run a few tools showed them repeated across
	// each new message, growing by the whole history each time.
	//
	// `result.ToolCalls` holds the results of this turn (its name is the
	// loop's, not this layer's). A call whose id produced one of them is a
	// call this turn made; anything else belongs to an earlier message that
	// already carries it.
	thisTurn := make(map[string]bool, len(result.ToolCalls))
	for _, c := range result.ToolCalls {
		thisTurn[c.CallID] = true
	}
	for _, m := range result.Messages {
		for _, c := range m.ToolCalls {
			if !thisTurn[c.ID] {
				continue
			}
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
	return parts
}

func (r *Runner) persist(ctx context.Context, in chat.Turn, agentID, answerID string, started time.Time, result *agentloop.Result) error {
	parts := answerParts(result)

	_, err := r.deps.Chats.Reply(ctx, chat.ReplyInput{
		Chat:      in.ChatID,
		ReplyTo:   in.MessageID,
		AgentID:   agentID,
		MessageID: answerID,
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
		// The outermost message names which layer gave up — "the google
		// provider did not answer" — and says nothing about why. The reason
		// is in the cause, which is exactly what AGENT_PROVIDER_FAILED's own
		// call to action promises ("the provider's own message is in the
		// cause"), and it was being dropped here: a person reading the
		// conversation got the useless half of a two-part error, and so did
		// anyone debugging from it. A wrong model id, an exhausted quota and
		// a rejected credential all looked identical.
		if detail := deepestMessage(app); detail != "" && detail != message {
			message += ": " + detail
		}
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

// deepestMessage returns the innermost apperr's message in a chain — the one
// closest to what actually went wrong, which for a provider failure is the
// provider's own words.
func deepestMessage(from error) string {
	out := ""
	for err := from; err != nil; {
		var app *apperr.Error
		if !errors.As(err, &app) {
			break
		}
		if app.Message != "" {
			out = app.Message
		}
		err = app.Cause
	}
	return out
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
