package session

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/chat"
)

// A failed turn is recorded on the message that asked for it, and the card a
// person reads is built from that record alone. It kept the code and the
// message and dropped the call to action, so "luara could not answer" never
// said where the thing to change was.
func TestAFailedTurnKeepsItsCallToAction(t *testing.T) {
	chats := &recordingChats{chat: &chat.Chat{ID: "c-1"}}
	runner := New(Deps{Chats: chats, Log: slog.New(slog.DiscardHandler)})

	cause := apperr.New("AGENT_MODEL_UNAVAILABLE").
		Msgf("the codex provider refused the model %q", "gpt-5.4-mini").
		CTA(apperr.CallToAction{Label: "choose another model", Tool: "models_list"})
	runner.recordFailure(context.Background(), chat.Turn{ChatID: "c-1", MessageID: "m-1"}, "luara", at, cause)

	if len(chats.replies) != 1 || chats.replies[0].Failure == nil {
		t.Fatalf("replies = %+v", chats.replies)
	}
	failure := chats.replies[0].Failure
	if len(failure.Actions) != 1 || failure.Actions[0].Tool != "models_list" {
		t.Fatalf("failure = %+v, want the call to action recorded with it", failure)
	}
}

// Which setting to change depends on where the model came from: an agent that
// names none answers with the Default slot, one that names its own answers
// with that.
func TestARefusedModelSaysWhichSettingToChange(t *testing.T) {
	refused := func() error {
		return apperr.New("AGENT_MODEL_UNAVAILABLE").
			Msgf("the codex provider refused the model %q", "gpt-5.4-mini").
			Issue("provider", "codex").Issue("model", "gpt-5.4-mini").
			CTA(apperr.CallToAction{Label: "choose a model", Tool: "models_list"})
	}

	slot, _ := apperr.As(adviseModel(refused(), &agent.Agent{ID: "luara"}))
	if slot.Issues["setting"] != "default slot" || slot.Actions[0].Tool != "config_update" ||
		!strings.Contains(slot.Actions[0].Label, "Default") {
		t.Errorf("agent without a model: %+v, want the Default slot named first", slot)
	}

	own, _ := apperr.As(adviseModel(refused(), &agent.Agent{ID: "api-builder", Provider: "codex", Model: "gpt-5.4-mini"}))
	if own.Issues["setting"] != "agent" || own.Actions[0].Tool != "agents_update" {
		t.Errorf("agent with its own model: %+v, want the agent's settings named first", own)
	}

	other := apperr.New("AGENT_PROVIDER_FAILED")
	if got, _ := apperr.As(adviseModel(other, &agent.Agent{ID: "luara"})); len(got.Actions) != 0 || got.Issues != nil {
		t.Errorf("a different failure was annotated: %+v", got)
	}
}
