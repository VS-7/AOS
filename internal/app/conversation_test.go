package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
	"github.com/OWNER/aos/internal/runtime/sandbox"
	"github.com/OWNER/aos/internal/runtime/toolexec/tools"
	"github.com/OWNER/aos/internal/transport/realtime"
)

// The model in these tests is a script. Registering it as a provider is how a
// turn runs end to end with no network: everything above the provider — the
// prompt, the sandbox, the tools, the hooks, the persistence — is the real
// thing.
var script struct {
	mu       sync.Mutex
	on       *fake.Provider
	observer *fake.Provider
}

func init() {
	providers.Register("scripted", func(providers.Config) (agentloop.LLMProvider, error) {
		script.mu.Lock()
		defer script.mu.Unlock()
		if script.on == nil {
			return nil, errors.New("no script is loaded")
		}
		return script.on, nil
	})
	// The background observer runs on its own slot, as it does in production:
	// a cheap model watching while an expensive one reasons. Pointing it at the
	// main script instead would make the observation eat the agent's next step.
	providers.Register("observer", func(providers.Config) (agentloop.LLMProvider, error) {
		script.mu.Lock()
		defer script.mu.Unlock()
		if script.observer == nil {
			return nil, errors.New("no observer script is loaded")
		}
		return script.observer, nil
	})
}

// observing loads the answer the background observer will give. Without a call
// to it the observer's provider refuses, which is what an installation with no
// subconscious model configured looks like — the turn is unaffected either way.
func observing(t *testing.T, steps ...fake.Step) *fake.Provider {
	t.Helper()
	p := &fake.Provider{Script: steps, ProviderName: "observer"}
	script.mu.Lock()
	script.observer = p
	script.mu.Unlock()
	t.Cleanup(func() {
		script.mu.Lock()
		script.observer = nil
		script.mu.Unlock()
	})
	return p
}

func play(t *testing.T, steps ...fake.Step) *fake.Provider {
	t.Helper()
	p := &fake.Provider{Script: steps, ProviderName: "scripted"}
	script.mu.Lock()
	script.on = p
	script.mu.Unlock()
	t.Cleanup(func() {
		script.mu.Lock()
		script.on = nil
		script.mu.Unlock()
	})
	return p
}

// conversing builds an installation with a workspace, an agent that may read
// and write inside it, and a model that answers from a script.
func conversing(t *testing.T) (*app.App, string) {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()

	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome:        home,
			env.KeyWorkspaceID: "atelier",
		})),
		WorkspaceRoot: root,
		Clock:         &clockx.Stepping{At: refTime, Step: time.Second},
		IDs:           &ids.Sequence{Prefix: "id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	ctx := context.Background()
	// The slot is a map, so it is set whole. A dotted path into it is not a
	// path the patcher knows, which is why the error that sends somebody here
	// carries the payload rather than a command line.
	if _, err := a.Config.Update(ctx, config.UpdateInput{Set: map[string]any{
		"agents.models": map[string]any{
			"default": map[string]any{
				"provider": "scripted", "model": "test-model", "reasoning": "medium",
			},
			"subconscious": map[string]any{
				"provider": "observer", "model": "small-model",
			},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Workspaces.Create(ctx, workspace.CreateInput{
		Name: "Atelier", Path: root,
	}); err != nil {
		t.Fatal(err)
	}
	// Creating the workspace seeds its orchestrator, so this widens what that
	// agent may do rather than making a second one.
	mission := "# Mission\nKeep the rewrite on track.\n"
	if _, err := a.Agents.Update(ctx, agent.UpdateInput{
		ID: "atlas", Content: &mission,
		Sandbox: &agent.Sandbox{Permissions: []string{"read", "write"}},
	}); err != nil {
		t.Fatal(err)
	}
	return a, root
}

func agentCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{
		WorkspaceID: "atelier", UserID: "vitor",
	})
}

// TestTheDeliveryOfPhaseFive is the phase in one test: a person writes a
// message, an agent reads a file with a tool, records what it learned as a
// memory, and answers — and all of it is on disk afterwards.
func TestTheDeliveryOfPhaseFive(t *testing.T) {
	a, root := conversing(t)
	ctx := agentCtx()

	if err := os.WriteFile(filepath.Join(root, "ROADMAP.md"),
		[]byte("# Roadmap\n\n- ship the runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	provider := play(t,
		fake.Step{
			Reasoning: "the roadmap is the file to read",
			Calls: []agentloop.ToolCall{
				fake.Call("c1", "Read", map[string]any{
					"file_path":  "ROADMAP.md",
					"_reasoning": "the question is about the roadmap, so read it before answering",
				}),
			},
			Usage: agentloop.Usage{Input: 900, Output: 30},
		},
		fake.Step{
			Calls: []agentloop.ToolCall{
				fake.Call("c2", "memories_store", map[string]any{
					"title":       "The roadmap's next milestone is the runtime",
					"description": "ROADMAP.md lists shipping the runtime as the only open item.",
					"category":    "fact",
					"content":     "# Evidence\nRead from ROADMAP.md.\n",
					"_reasoning":  "this changes what I plan next, so it is worth remembering",
				}),
			},
			Usage: agentloop.Usage{Input: 1200, Output: 60},
		},
		fake.Step{
			Text:  "The roadmap has one open item: ship the runtime. I have recorded that.",
			Usage: agentloop.Usage{Input: 1400, Output: 20},
		},
	)

	sent, err := a.Chats.Send(ctx, chat.SendInput{
		Chat: mustChat(t, a), Text: "what is left on the roadmap?", Agent: "atlas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sent.Dispatched {
		t.Fatal("the message was stored and nobody was asked to answer it")
	}

	answered := waitForAnswer(t, a, sent.Message.ID)

	// The answer is in the conversation.
	if !strings.Contains(answered.Text(), "ship the runtime") {
		t.Fatalf("the answer is %q", answered.Text())
	}

	// So is what it did to get there. A transcript that shows only the answer
	// cannot answer "why did it do that".
	var sawCall, sawResult bool
	for _, p := range answered.Parts {
		switch p.Type {
		case chat.PartToolCall:
			sawCall = true
		case chat.PartToolResult:
			sawResult = true
			if p.ToolName == "Read" && !strings.Contains(string(p.Output), "ship the runtime") {
				t.Errorf("the stored result of Read does not carry what it read: %s", p.Output)
			}
		}
	}
	if !sawCall || !sawResult {
		t.Fatalf("the transcript does not record the tool calls: %+v", answered.Parts)
	}

	// The memory is on disk, written by the agent through the same command the
	// CLI and MCP publish.
	recalled, err := a.Memories.Recall(ctx, memory.RecallInput{Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	if recalled.Total != 1 {
		t.Fatalf("the agent stored %d memories", recalled.Total)
	}
	if !strings.Contains(recalled.Memories[0].Title, "runtime") {
		t.Fatalf("memory = %+v", recalled.Memories[0])
	}
	onDisk := filepath.Join(root, ".aos", "agents", "atlas", "memories")
	if entries, err := os.ReadDir(onDisk); err != nil || len(entries) == 0 {
		t.Fatalf("nothing was written under %s: %v", onDisk, err)
	}

	// The cost of the turn is recorded on the message that caused it.
	asked := messageByID(t, a, sent.Message.ID)
	if len(asked.Runs) != 1 {
		t.Fatalf("the message records %d attempts", len(asked.Runs))
	}
	run := asked.Runs[0]
	if run.Status != chat.StatusCompleted || run.AgentID != "atlas" {
		t.Fatalf("run = %+v", run)
	}
	if run.Usage.Total != 3610 {
		t.Errorf("Usage.Total = %d, want the sum of the three calls", run.Usage.Total)
	}

	// The agent read the context document it was given, and the document was
	// the assembled one rather than a placeholder.
	first := provider.Requests()[0]
	for _, fragment := range []string{"<system_instructions", "<identity", "Keep the rewrite on track", "<memories"} {
		if !strings.Contains(first.Instructions, fragment) {
			t.Errorf("the assembled prompt is missing %q", fragment)
		}
	}
	// And it was offered both kinds of tool: the native ones and the domain
	// commands.
	var native, domain bool
	for _, s := range first.Tools {
		switch s.Name {
		case "Read":
			native = true
		case "memories_store":
			domain = true
		}
	}
	if !native || !domain {
		t.Errorf("the tool list is missing one of the two kinds: %d tools", len(first.Tools))
	}
}

// TestTheSecondTurnRemembersTheFirst. Continuity is the whole premise, and it
// is a property of what the runtime hands the model, not of the model.
func TestTheSecondTurnRemembersTheFirst(t *testing.T) {
	a, _ := conversing(t)
	ctx := agentCtx()
	chatID := mustChat(t, a)

	first := play(t, fake.Step{Text: "Noted: you prefer short answers."})
	sent, err := a.Chats.Send(ctx, chat.SendInput{Chat: chatID, Text: "keep answers short", Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	waitForAnswer(t, a, sent.Message.ID)
	_ = first

	second := play(t, fake.Step{Text: "Short, as agreed."})
	next, err := a.Chats.Send(ctx, chat.SendInput{Chat: chatID, Text: "what is 2+2", Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	waitForAnswer(t, a, next.Message.ID)

	messages := second.Requests()[0].Messages
	if len(messages) < 3 {
		t.Fatalf("the second turn saw %d messages, so the first one was lost", len(messages))
	}
	var sawFirstAnswer bool
	for _, m := range messages {
		if strings.Contains(m.Text, "you prefer short answers") {
			sawFirstAnswer = true
		}
	}
	if !sawFirstAnswer {
		t.Fatalf("the second turn did not see what the agent said in the first: %+v", messages)
	}
}

// TestAPolicyHookStopsTheAgentAndTheAuditSaysWhich. The extension mechanism,
// end to end and on disk: a handler registered on the bus refuses a tool call,
// the tool does not run, and the log names the handler that decided.
func TestAPolicyHookStopsTheAgentAndTheAuditSaysWhich(t *testing.T) {
	a, root := conversing(t)
	ctx := agentCtx()

	a.Hooks.Register(event.FuncHandler{
		Name:   "no-writes-on-friday",
		Events: []event.Type{event.PreToolUse},
		Fn: func(_ context.Context, e event.Event) (event.Outcome, error) {
			if e.Tool != "Write" {
				return event.Outcome{}, nil
			}
			return event.Outcome{
				PermissionDecision: event.PermissionDeny,
				Reason:             "the workspace is frozen for the release",
			}, nil
		},
	})

	play(t,
		fake.Step{Calls: []agentloop.ToolCall{
			fake.Call("c1", "Write", map[string]any{
				"file_path": "notes.md", "content": "x",
				"_reasoning": "writing the note the user asked for",
			}),
		}},
		fake.Step{Text: "I was not allowed to write; the workspace is frozen."},
	)

	sent, err := a.Chats.Send(ctx, chat.SendInput{Chat: mustChat(t, a), Text: "write a note", Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	answered := waitForAnswer(t, a, sent.Message.ID)

	if !strings.Contains(answered.Text(), "not allowed") {
		t.Fatalf("the answer is %q", answered.Text())
	}
	if _, err := os.Stat(filepath.Join(root, "notes.md")); err == nil {
		t.Fatal("the refused write happened anyway")
	}

	// The audit log is beside the agent, append-only, and says who decided.
	day := filepath.Join(root, ".aos", "agents", "atlas", "events")
	entries, err := os.ReadDir(day)
	if err != nil || len(entries) == 0 {
		t.Fatalf("nothing was recorded under %s: %v", day, err)
	}
	raw, err := os.ReadFile(filepath.Join(day, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var sawDecision bool
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec event.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Outcome.HookID == "no-writes-on-friday" && rec.Outcome.PermissionDecision == event.PermissionDeny {
			sawDecision = true
			if !strings.Contains(string(rec.Payload), "notes.md") {
				t.Error("the record does not carry the payload that was refused")
			}
		}
	}
	if !sawDecision {
		t.Fatalf("the log does not name the hook that refused:\n%s", raw)
	}
}

// TestAnAgentCannotApproveItsOwnToolCall. The approval channel is worth nothing
// if the thing being restrained can operate it.
func TestAnAgentCannotApproveItsOwnToolCall(t *testing.T) {
	a, _ := conversing(t)
	for _, d := range a.Registry.AgentTools() {
		if strings.HasPrefix(d.Key(), "approvals_") {
			t.Fatalf("%s is in the agent's tool registry", d.Key())
		}
	}
	// And it is on the other surfaces, where a person is.
	if _, _, ok := a.Registry.Lookup("approvals_decide"); !ok {
		t.Fatal("nobody can answer an approval at all")
	}
}

// The rest of the privilege boundary command.Command's Registry field
// describes: "gateway, auth and tunnels stay out of the agent's reach".
//
// The gateway group was inside it. `gateway_stop` offered to a model as an
// ordinary tool terminates the process the turn is running in — the daemon
// signals the pid it recorded for itself — so the turn dies silently and the
// window loses the daemon it is talking to.
func TestTheAgentIsNotOfferedTheInstallationsOwnControls(t *testing.T) {
	a, _ := conversing(t)
	forbidden := []string{"gateway_", "auth_", "tunnel_", "update_"}
	for _, d := range a.Registry.AgentTools() {
		for _, prefix := range forbidden {
			if strings.HasPrefix(d.Key(), prefix) {
				t.Errorf("%s is in the agent's tool registry", d.Key())
			}
		}
	}
	// Still reachable by a person, on every other surface.
	if _, _, ok := a.Registry.Lookup("gateway_status"); !ok {
		t.Error("nobody can ask after the daemon at all")
	}
	// Registering and unregistering a workspace is the shape of the
	// installation, not work inside one.
	for _, key := range []string{"workspace_create", "workspace_delete"} {
		for _, d := range a.Registry.AgentTools() {
			if d.Key() == key {
				t.Errorf("%s is in the agent's tool registry", key)
			}
		}
		if _, _, ok := a.Registry.Lookup(key); !ok {
			t.Errorf("%s is not reachable on any surface", key)
		}
	}
}

// toolCeiling is the smallest documented per-request function limit among the
// providers this runtime speaks to: OpenAI's Chat Completions API accepts "up
// to 128 functions" in one request, which the compat providers (openrouter,
// crof, opencode) all go through. Anthropic's own limit is far higher.
//
// A turn offers the model every agent-registry command plus the native
// filesystem tools, in one list, on every step. At 133 the list was already
// over this line: against a model that enforces it, every turn would be
// refused before a single tool could run — and the list costs several
// thousand prompt tokens per step on every provider besides.
const toolCeiling = 128

// TestTheToolListFitsInOneRequest is a ceiling, not a target. When it fails,
// the answer is not to delete a capability somebody uses: it is to publish
// composite per-group tools to the agent the way internal/transport/mcpserver
// already does for MCP clients — one `Tasks({action})` tool instead of seven
// — which collapses 121 commands into 23 groups and leaves room to grow.
func TestTheToolListFitsInOneRequest(t *testing.T) {
	a, _ := conversing(t)

	box, err := sandbox.New(sandbox.Options{WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	offered := len(a.Registry.AgentTools()) + len(tools.FS(box))

	if offered > toolCeiling {
		t.Errorf("a turn offers the model %d tools, more than the %d one request may carry — "+
			"publish composite per-group tools rather than dropping a capability", offered, toolCeiling)
	}
}

// TestAskReachesTheDesktopAndTheAnswerLetsTheCallRun — the whole of ADR-0007,
// over the same event channel the desktop listens on.
func TestAskReachesTheDesktopAndTheAnswerLetsTheCallRun(t *testing.T) {
	a, root := conversing(t)
	ctx := agentCtx()

	a.Hooks.Register(event.FuncHandler{
		Name:   "confirm-writes",
		Events: []event.Type{event.PreToolUse},
		Fn: func(_ context.Context, e event.Event) (event.Outcome, error) {
			if e.Tool != "Write" {
				return event.Outcome{}, nil
			}
			return event.Outcome{
				PermissionDecision: event.PermissionAsk,
				Reason:             "writing a file needs a person",
			}, nil
		},
	})

	play(t,
		fake.Step{Calls: []agentloop.ToolCall{
			fake.Call("c1", "Write", map[string]any{
				"file_path": "approved.md", "content": "written after a person said yes",
				"_reasoning": "the user asked for this file",
			}),
		}},
		fake.Step{Text: "Written."},
	)

	// Somebody is watching the workspace channel, which is how the desktop
	// learns a request is waiting.
	watcher := &collector{seen: make(chan realtime.Event, 8)}
	go a.Events.Subscribe(ctx, realtime.ChannelFor("atelier"), watcher)
	for a.Events.Subscribers(realtime.ChannelFor("atelier")) == 0 {
		time.Sleep(time.Millisecond)
	}

	sent, err := a.Chats.Send(ctx, chat.SendInput{Chat: mustChat(t, a), Text: "write approved.md", Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}

	// The request arrives, and the person answers it.
	pending := waitForPending(t, a)
	if pending.ToolName != "Write" || pending.Risk == "" {
		t.Fatalf("request = %+v", pending)
	}
	// Answered through the command surface, which is the same path the desktop
	// and a person at a terminal take.
	decide, _, ok := a.Registry.Lookup("approvals_decide")
	if !ok {
		t.Fatal("there is no way to answer an approval")
	}
	decided, err := decide.Invoke(ctx, command.SurfaceHTTP, json.RawMessage(
		`{"id":"`+pending.ID+`","approved":true,"_reasoning":"reversible and inside the workspace"}`))
	if err != nil {
		t.Fatal(err)
	}
	if settled, _ := decided.(app.DecideOutput); !settled.Settled {
		t.Fatalf("the decision did not land: %+v", decided)
	}

	waitForAnswer(t, a, sent.Message.ID)
	if _, err := os.Stat(filepath.Join(root, "approved.md")); err != nil {
		t.Fatalf("the approved write did not happen: %v", err)
	}
	if !watcher.sawType(realtime.EventApprovalRequest) {
		t.Error("the desktop was never told a request was waiting")
	}
}

// TestAWaitingCallThatNobodyAnswersIsRefused, and the file is not written.
func TestAWaitingCallThatNobodyAnswersIsRefused(t *testing.T) {
	a, root := conversing(t)
	ctx := agentCtx()

	a.Hooks.Register(event.FuncHandler{
		Name:   "confirm-writes",
		Events: []event.Type{event.PreToolUse},
		Fn: func(_ context.Context, e event.Event) (event.Outcome, error) {
			if e.Tool != "Write" {
				return event.Outcome{}, nil
			}
			return event.Outcome{PermissionDecision: event.PermissionAsk, Reason: "needs a person"}, nil
		},
	})

	play(t,
		fake.Step{Calls: []agentloop.ToolCall{
			fake.Call("c1", "Write", map[string]any{
				"file_path": "unapproved.md", "content": "x", "_reasoning": "asked to",
			}),
		}},
		fake.Step{Text: "Nobody confirmed, so I did not write it."},
	)

	sent, err := a.Chats.Send(ctx, chat.SendInput{Chat: mustChat(t, a), Text: "write it", Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	// Closing the channel is what a daemon shutting down does, and it is the
	// fastest of the three ways a request stops waiting. All three deny.
	waitForPending(t, a)
	a.Approvals.Close()

	waitForAnswer(t, a, sent.Message.ID)
	if _, err := os.Stat(filepath.Join(root, "unapproved.md")); err == nil {
		t.Fatal("a call nobody approved was executed")
	}
}

// TestATurnThatFailsIsVisibleInTheConversation, rather than only in a log the
// person cannot see.
func TestATurnThatFailsIsVisibleInTheConversation(t *testing.T) {
	a, _ := conversing(t)
	ctx := agentCtx()

	play(t, fake.Step{Err: errorOf("the provider is down")})

	sent, err := a.Chats.Send(ctx, chat.SendInput{Chat: mustChat(t, a), Text: "hello", Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		asked := messageByID(t, a, sent.Message.ID)
		if len(asked.Runs) > 0 {
			if asked.Runs[0].Status != chat.StatusError {
				t.Fatalf("run = %+v", asked.Runs[0])
			}
			if asked.Runs[0].Error == nil || !strings.Contains(asked.Runs[0].Error.Code, "PROVIDER") {
				t.Fatalf("the failure does not say what went wrong: %+v", asked.Runs[0].Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the failed turn left no trace in the conversation")
}

// waitForAnswer waits until the turn that answers a message has finished, and
// returns the answer.
//
// The signal is the attempt recorded on the message that asked, not the arrival
// of some assistant message: in a conversation with more than one turn, the
// previous answer is already there and would satisfy the weaker check
// immediately.
func waitForAnswer(t *testing.T, a *app.App, askedID string) chat.Message {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		c, err := a.Chats.Get(agentCtx(), chat.GetInput{Chat: chatID(t, a)})
		if err != nil {
			t.Fatal(err)
		}
		var finished bool
		for _, m := range c.Messages {
			if m.ID == askedID && len(m.Runs) > 0 {
				finished = true
			}
		}
		if finished {
			for i := len(c.Messages) - 1; i >= 0; i-- {
				if c.Messages[i].Role == chat.RoleAssistant {
					return c.Messages[i]
				}
			}
			return chat.Message{}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no answer arrived")
	return chat.Message{}
}

func waitForPending(t *testing.T, a *app.App) event.ApprovalRequest {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if p := a.Approvals.Pending(); len(p) > 0 {
			return p[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no approval was requested")
	return event.ApprovalRequest{}
}

var chatOnce sync.Map

// mustChat creates the conversation once per installation and returns its id.
func mustChat(t *testing.T, a *app.App) string {
	t.Helper()
	if id, ok := chatOnce.Load(a); ok {
		return id.(string)
	}
	c, err := a.Chats.Create(agentCtx(), chat.CreateInput{
		Title: "Planning",
		Participants: []chat.Participant{
			{Type: chat.ActorAgent, ID: "atlas"},
			{Type: chat.ActorUser, ID: "vitor"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	chatOnce.Store(a, c.ID)
	return c.ID
}

func chatID(t *testing.T, a *app.App) string { return mustChat(t, a) }

func messageByID(t *testing.T, a *app.App, id string) chat.Message {
	t.Helper()
	c, err := a.Chats.Get(agentCtx(), chat.GetInput{Chat: chatID(t, a)})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range c.Messages {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no message %s", id)
	return chat.Message{}
}

type collector struct {
	mu   sync.Mutex
	all  []realtime.Event
	seen chan realtime.Event
}

func (c *collector) Send(_ context.Context, payload []byte) error {
	var e realtime.Event
	if err := json.Unmarshal(payload, &e); err != nil {
		return err
	}
	c.mu.Lock()
	c.all = append(c.all, e)
	c.mu.Unlock()
	select {
	case c.seen <- e:
	default:
	}
	return nil
}

func (c *collector) Close() error { return nil }

func (c *collector) sawType(kind string) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, e := range c.all {
			if e.Type == kind {
				c.mu.Unlock()
				return true
			}
		}
		c.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

type scriptError string

func (e scriptError) Error() string { return string(e) }

func errorOf(s string) error { return scriptError(s) }
