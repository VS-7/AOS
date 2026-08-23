package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/sandbox"
)

// The composition this package does is proved end to end by the conversation
// suite in internal/app. What is tested here is the part that is not wiring:
// the translation between a stored conversation and what a model reads, and the
// translation between an agent's file and what it may reach.

var at = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// TestAStoredConversationBecomesWhatTheModelReads. The order is the part that
// matters: every provider requires a tool result to follow the call that
// produced it, and a transcript that reorders them is rejected.
func TestAStoredConversationBecomesWhatTheModelReads(t *testing.T) {
	stored := &chat.Chat{Messages: []chat.Message{
		{Role: chat.RoleUser, Parts: []chat.Part{{Type: chat.PartText, Text: "read the readme"}}, CreatedAt: at},
		{
			Role: chat.RoleAssistant, CreatedAt: at.Add(time.Second),
			Parts: []chat.Part{
				{Type: chat.PartReasoning, Text: "it is at the root"},
				{Type: chat.PartToolCall, ToolName: "Read", ToolCallID: "c1", Input: json.RawMessage(`{"file_path":"README.md"}`)},
				{Type: chat.PartToolResult, ToolName: "Read", ToolCallID: "c1", Output: json.RawMessage(`{"data":"hello"}`)},
				{Type: chat.PartText, Text: "it says hello"},
			},
		},
		// A system message is not part of the conversation the model reads:
		// its instructions are the assembled document.
		{Role: chat.RoleSystem, Parts: []chat.Part{{Type: chat.PartText, Text: "ignored"}}},
	}}

	got := transcript(stored)
	if len(got) != 3 {
		t.Fatalf("got %d messages: %+v", len(got), got)
	}
	if got[0].Role != agentloop.RoleUser || got[0].Text != "read the readme" {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Role != agentloop.RoleAssistant || got[1].Text != "it says hello" {
		t.Fatalf("second = %+v", got[1])
	}
	if got[1].Reasoning != "it is at the root" || len(got[1].ToolCalls) != 1 {
		t.Fatalf("the assistant turn lost something: %+v", got[1])
	}
	if got[2].Role != agentloop.RoleTool || got[2].CallID != "c1" {
		t.Fatalf("the result does not follow its call: %+v", got[2])
	}
}

// TestAnAssistantTurnWithNothingInItIsDropped, so a failed turn does not leave
// a blank the next model call has to read.
func TestAnAssistantTurnWithNothingInItIsDropped(t *testing.T) {
	got := transcript(&chat.Chat{Messages: []chat.Message{
		{Role: chat.RoleAssistant, CreatedAt: at},
		{Role: chat.RoleUser, Parts: []chat.Part{{Type: chat.PartText, Text: "hello"}}},
	}})
	if len(got) != 1 || got[0].Role != agentloop.RoleUser {
		t.Fatalf("got %+v", got)
	}
}

// TestAnAgentWithNoPolicyGetsTheStrictOne. The default is the point of
// ADR-0006: an agent's reach is a decision somebody makes and writes down.
func TestAnAgentWithNoPolicyGetsTheStrictOne(t *testing.T) {
	r := &Runner{deps: Deps{WorkspaceRoot: t.TempDir()}}

	box, err := r.sandboxFor(&agent.Agent{ID: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	perms := box.Permissions()
	if !perms.Read || perms.Write || perms.Delete || perms.Execute {
		t.Fatalf("the default policy is %+v", perms)
	}
	if _, err := box.VerifyExec("ls", nil); err == nil {
		t.Fatal("an agent with no policy could run a command")
	}
}

// TestTheAgentsFileDecidesWhatItMayReach.
func TestTheAgentsFileDecidesWhatItMayReach(t *testing.T) {
	r := &Runner{deps: Deps{WorkspaceRoot: t.TempDir()}}

	box, err := r.sandboxFor(&agent.Agent{
		ID: "builder",
		Sandbox: &agent.Sandbox{
			Permissions: []string{"read", "write", "execute"},
			Exec: &agent.Exec{
				Policy: sandbox.PolicyAllowlist,
				Allow:  []string{"git"},
				DenyArgs: []string{
					"git push --force*",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	perms := box.Permissions()
	if !perms.Write || !perms.Execute || perms.Delete {
		t.Fatalf("permissions = %+v", perms)
	}
	if _, err := box.VerifyExec("git", []string{"push", "--force"}); err == nil {
		t.Fatal("the denied pattern from the agent's file was not applied")
	}
}

// TestUsageCrossesTheBoundaryIntact, because the number on the message is what
// a person reads when they ask what a conversation cost.
func TestUsageCrossesTheBoundaryIntact(t *testing.T) {
	got := usageOf(agentloop.Usage{Input: 10, Output: 5, Reasoning: 3, Cached: 2, Total: 18, CostUSD: 0.04})
	want := chat.TokenUsage{Input: 10, Output: 5, Reasoning: 3, Cached: 2, Total: 18, CostUSD: 0.04}
	if got != want {
		t.Fatalf("usage = %+v, want %+v", got, want)
	}
}

// TestTheLastUserMessageIsWhenTheAgentThinksItWasAsked.
func TestTheLastUserMessageIsWhenTheAgentThinksItWasAsked(t *testing.T) {
	later := at.Add(5 * time.Minute)
	stored := &chat.Chat{Messages: []chat.Message{
		{Role: chat.RoleUser, CreatedAt: at},
		{Role: chat.RoleAssistant, CreatedAt: at.Add(time.Minute)},
		{Role: chat.RoleUser, CreatedAt: later},
	}}
	if got := lastUserAt(stored); !got.Equal(later) {
		t.Fatalf("lastUserAt = %s, want %s", got, later)
	}
	if got := lastUserAt(&chat.Chat{}); !got.IsZero() {
		t.Fatalf("a conversation nobody has written to reports %s", got)
	}
}

// fakeBots records every Deliver call, or fails it once if told to.
type fakeBots struct {
	calls []deliverCall
	fail  error
}

type deliverCall struct{ provider, agentID, chatID, text string }

func (f *fakeBots) Deliver(_ context.Context, provider, agentID, chatID, text string) error {
	f.calls = append(f.calls, deliverCall{provider, agentID, chatID, text})
	return f.fail
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// TestDeliverToChannelPushesTheAnswerOut is the property bot.Registry.Deliver
// existed for and nothing called: a conversation bound to an external
// channel gets its turn's answer pushed back out, addressed by the
// channel's own provider and chat id, not this system's own.
func TestDeliverToChannelPushesTheAnswerOut(t *testing.T) {
	bots := &fakeBots{}
	r := &Runner{deps: Deps{Bots: bots}, log: discardLogger()}
	conversation := &chat.Chat{
		ID:      "c1",
		Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "12345"},
	}

	r.deliverToChannel(context.Background(), conversation, "atlas", &agentloop.Result{Text: "the answer"})

	if len(bots.calls) != 1 {
		t.Fatalf("Deliver was called %d times, want 1", len(bots.calls))
	}
	got := bots.calls[0]
	if got.provider != "telegram" || got.agentID != "atlas" || got.chatID != "12345" || got.text != "the answer" {
		t.Fatalf("Deliver call = %+v", got)
	}
}

func TestDeliverToChannelSkipsAConversationWithNoChannel(t *testing.T) {
	bots := &fakeBots{}
	r := &Runner{deps: Deps{Bots: bots}, log: discardLogger()}

	r.deliverToChannel(context.Background(), &chat.Chat{ID: "c1"}, "atlas", &agentloop.Result{Text: "the answer"})

	if len(bots.calls) != 0 {
		t.Fatalf("Deliver was called for a conversation with no channel: %+v", bots.calls)
	}
}

func TestDeliverToChannelSkipsWhenNoBotsAreWired(t *testing.T) {
	r := &Runner{deps: Deps{}, log: discardLogger()} // Bots left nil, the state a build with no channel provider is in
	conversation := &chat.Chat{ID: "c1", Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "12345"}}

	// Must not panic on a nil Bots.
	r.deliverToChannel(context.Background(), conversation, "atlas", &agentloop.Result{Text: "the answer"})
}

func TestDeliverToChannelSkipsATurnWithNoText(t *testing.T) {
	bots := &fakeBots{}
	r := &Runner{deps: Deps{Bots: bots}, log: discardLogger()}
	conversation := &chat.Chat{ID: "c1", Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "12345"}}

	r.deliverToChannel(context.Background(), conversation, "atlas", &agentloop.Result{Text: ""})

	if len(bots.calls) != 0 {
		t.Fatalf("Deliver was called for a turn with no text: %+v", bots.calls)
	}
}

// TestDeliverToChannelFailureIsNotPropagated: the answer is already on the
// chat record by the time this runs (persist happens first in Run) — a
// delivery failure is a degraded experience for whoever is on the other
// side of the channel, not a reason to fail a turn that already succeeded.
func TestDeliverToChannelFailureIsNotPropagated(t *testing.T) {
	bots := &fakeBots{fail: errors.New("telegram is down")}
	r := &Runner{deps: Deps{Bots: bots}, log: discardLogger()}
	conversation := &chat.Chat{ID: "c1", Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "12345"}}

	// Must not panic, and there is nothing to assert on the return value —
	// deliverToChannel returns nothing, which is itself the point.
	r.deliverToChannel(context.Background(), conversation, "atlas", &agentloop.Result{Text: "the answer"})
	if len(bots.calls) != 1 {
		t.Fatalf("Deliver was called %d times, want 1 (attempted once, regardless of the outcome)", len(bots.calls))
	}
}

// The loop's transcript is seeded with the conversation so far, so it holds
// every tool call the chat has ever made. Only this turn's belong on this
// turn's message — otherwise each answer repeats the whole history, and grows
// by it again on the next one.
func TestAnAnswerCarriesOnlyTheToolCallsOfItsOwnTurn(t *testing.T) {
	result := &agentloop.Result{
		Text: "done",
		Messages: []agentloop.Message{
			// An earlier turn, already stored on its own message.
			{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{
				{ID: "old-1", Name: "memories_store"},
			}},
			// This turn.
			{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{
				{ID: "new-1", Name: "tasks_list"},
			}},
		},
		ToolCalls: []agentloop.ToolResult{{CallID: "new-1", Name: "tasks_list"}},
	}

	var calls, results []string
	for _, p := range answerParts(result) {
		switch p.Type {
		case chat.PartToolCall:
			calls = append(calls, p.ToolCallID)
		case chat.PartToolResult:
			results = append(results, p.ToolCallID)
		}
	}

	if len(calls) != 1 || calls[0] != "new-1" {
		t.Errorf("tool calls = %v, want only this turn's", calls)
	}
	if len(results) != 1 || results[0] != "new-1" {
		t.Errorf("tool results = %v, want the one this turn produced", results)
	}
}

func TestAnAnswerWithNoToolsIsJustItsText(t *testing.T) {
	parts := answerParts(&agentloop.Result{Text: "hello"})
	if len(parts) != 1 || parts[0].Type != chat.PartText || parts[0].Text != "hello" {
		t.Fatalf("parts = %#v, want one text part", parts)
	}
}
