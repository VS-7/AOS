package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/runtime/agentloop"
)

// A task conversation, the way the orchestrator opens one: the delegation is
// Luara's message, stored as an assistant message because an agent wrote it,
// and the member's answer follows with the tools it used on the way.
func delegatedTask() *chat.Chat {
	luara := &chat.Author{Type: chat.ActorAgent, ID: "luara"}
	builder := &chat.Author{Type: chat.ActorAgent, ID: "api-builder"}
	return &chat.Chat{Kind: chat.KindTask, Agent: "api-builder", Messages: []chat.Message{
		{
			Role: chat.RoleAssistant, Author: luara, CreatedAt: at,
			Parts: []chat.Part{{Type: chat.PartText, Text: "@api-builder Implement the minimal library API"}},
		},
		{
			Role: chat.RoleAssistant, Author: builder, CreatedAt: at.Add(time.Minute),
			Parts: []chat.Part{
				{Type: chat.PartToolCall, ToolName: "projects_get", ToolCallID: "c1", Input: json.RawMessage(`{"step":0}`)},
				{Type: chat.PartToolCall, ToolName: "goals_get", ToolCallID: "c2", Input: json.RawMessage(`{"step":0}`)},
				{Type: chat.PartToolResult, ToolName: "projects_get", ToolCallID: "c1", Output: json.RawMessage(`{"data":"API DE TESTE"}`)},
				{Type: chat.PartToolResult, ToolName: "goals_get", ToolCallID: "c2", Output: json.RawMessage(`{"data":"none"}`)},
				{Type: chat.PartText, Text: "Blocked before any change to the code."},
			},
		},
	}}
}

// Another agent's words were replayed as the words of the agent taking the
// turn. The member read the orchestrator's delegation as a reply it had given
// itself, and Gemini refused the conversation outright: the member's tool calls
// came straight after a model turn — Luara's — instead of after a user turn.
func TestAnotherAgentsMessageIsNotReplayedAsTheTurnTakersOwn(t *testing.T) {
	gemini := &strictGemini{}
	backend := httptest.NewServer(gemini)
	t.Cleanup(backend.Close)

	if err := replayToGemini(t, backend, delegatedTask()); err != nil {
		t.Fatalf("the member's next turn was refused: %v (%v)", err, gemini.refusals())
	}
}

// Each agent in the conversation reads its own messages as its turns and the
// other's as somebody talking to it — with the name of who is talking, and
// without the steps the other agent took on the way.
func TestEachAgentReadsTheOthersMessagesAsSomebodyTalkingToIt(t *testing.T) {
	stored := delegatedTask()
	names := map[string]string{"luara": "Luara", "api-builder": "API Builder"}

	member := transcript(stored, speaker{self: "api-builder", names: names})
	if len(member) != 4 {
		t.Fatalf("the member read %d messages: %+v", len(member), member)
	}
	if member[0].Role != agentloop.RoleUser || member[0].Text != "[Luara]: @api-builder Implement the minimal library API" {
		t.Fatalf("the member read the delegation as %+v", member[0])
	}
	if member[1].Role != agentloop.RoleAssistant || len(member[1].ToolCalls) != 2 ||
		member[2].Role != agentloop.RoleTool || member[3].Role != agentloop.RoleTool {
		t.Fatalf("the member's own step lost its calls or results: %+v", member[1:])
	}

	orchestrator := transcript(stored, speaker{self: "luara", names: names})
	if len(orchestrator) != 2 {
		t.Fatalf("the orchestrator read %d messages: %+v", len(orchestrator), orchestrator)
	}
	if orchestrator[0].Role != agentloop.RoleAssistant || orchestrator[0].Text != "@api-builder Implement the minimal library API" {
		t.Fatalf("the orchestrator's own delegation is %+v", orchestrator[0])
	}
	if got := orchestrator[1]; got.Role != agentloop.RoleUser || got.Text != "[API Builder]: Blocked before any change to the code." ||
		len(got.ToolCalls) > 0 {
		t.Fatalf("the orchestrator read the member's report as %+v", got)
	}
}

// The Responses API pairs every function_call_output with its call. Reading the
// delegation as a user turn must leave the member's own calls paired, and the
// orchestrator — who did not make them — must not be handed their outputs.
func TestACodexTurnStillPairsItsCallsWithTheirResults(t *testing.T) {
	member := delegatedTask()
	items := bodyOfNextTurn(t, member)
	if outputs, calls := orphansIn(items); outputs != 0 || calls != 0 {
		t.Fatalf("the member's body has %d orphan outputs and %d orphan calls: %+v", outputs, calls, items)
	}
	if first := items[0]; first["role"] != "user" || !strings.Contains(asJSON(first), "[luara]: @api-builder") {
		t.Fatalf("the delegation reached Codex as %+v", first)
	}
	offered := 0
	for _, item := range items {
		if item["type"] == "function_call" {
			offered++
		}
	}
	if offered != 2 {
		t.Fatalf("the member's two calls reached Codex as %d", offered)
	}

	orchestrator := delegatedTask()
	orchestrator.Agent = "luara"
	for _, item := range bodyOfNextTurn(t, orchestrator) {
		if item["type"] == "function_call" || item["type"] == "function_call_output" {
			t.Fatalf("the orchestrator was handed the member's step: %+v", item)
		}
	}
}

func asJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// directoryOf answers for the agents it holds and refuses the rest, the way the
// agent aggregate refuses one that was deleted.
type directoryOf map[string]agent.Agent

func (d directoryOf) Get(_ context.Context, in agent.GetInput) (*agent.Agent, error) {
	if found, ok := d[in.ID]; ok {
		return &found, nil
	}
	return nil, apperr.New("AGENT_NOT_FOUND").Msgf("no agent %q", in.ID)
}

// The name in the attribution is the one a person gave the agent, and an agent
// deleted since its message was written is still called something.
func TestTheOtherAgentsAreCalledByTheirNames(t *testing.T) {
	stored := delegatedTask()
	stored.Messages = append(stored.Messages, chat.Message{
		Role: chat.RoleAssistant, Author: &chat.Author{Type: chat.ActorAgent, ID: "gone"}, CreatedAt: at.Add(2 * time.Minute),
		Parts: []chat.Part{{Type: chat.PartText, Text: "I was here too."}},
	})
	runner := New(Deps{
		Agents: directoryOf{"luara": {ID: "luara", Name: "Luara"}},
		Log:    slog.New(slog.DiscardHandler),
	})

	got := transcript(stored, runner.speakersOf(context.Background(), stored, "api-builder"))
	if got[0].Text != "[Luara]: @api-builder Implement the minimal library API" {
		t.Errorf("the orchestrator is called %q", got[0].Text)
	}
	if last := got[len(got)-1]; last.Text != "[gone]: I was here too." {
		t.Errorf("a deleted agent's message reads %q", last.Text)
	}
}
