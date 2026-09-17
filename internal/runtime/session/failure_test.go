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

// cancelledRefusingChats refuses a write on a context that is already over,
// as the conversation store does: its Get answers AOS_CHAT_NOT_FOUND once the
// context is cancelled, although the conversation is there.
type cancelledRefusingChats struct {
	recordingChats
	deadlines int
}

func (r *cancelledRefusingChats) Reply(ctx context.Context, in chat.ReplyInput) (chat.ReplyOutput, error) {
	if ctx.Err() != nil {
		return chat.ReplyOutput{}, apperr.New("CHAT_NOT_FOUND").Msgf("no chat %q", in.Chat)
	}
	if _, ok := ctx.Deadline(); ok {
		r.deadlines++
	}
	return r.recordingChats.Reply(ctx, in)
}

// A turn cut short — by Stop, or by a daemon shutting down under a scheduled
// routine's run — is handed a context that is already cancelled, and the
// record of how it ended was written on that context. The store refused it
// (logged as "a stopped turn could not be recorded … AOS_CHAT_NOT_FOUND"),
// and the message kept runs[0].status = running for good: a spinner in the
// routine's transcript that nothing would ever end.
func TestATurnCutShortIsRecordedAlthoughItsContextIsOver(t *testing.T) {
	for name, cause := range map[string]error{
		"stopped": context.Canceled,
		"failed":  apperr.New("AGENT_PROVIDER_FAILED").Msgf("the provider did not answer"),
	} {
		t.Run(name, func(t *testing.T) {
			chats := &cancelledRefusingChats{recordingChats: recordingChats{chat: &chat.Chat{ID: "c-1"}}}
			runner := New(Deps{Chats: chats, Log: slog.New(slog.DiscardHandler)})
			over, cancel := context.WithCancel(context.Background())
			cancel()

			runner.recordFailure(over, chat.Turn{ChatID: "c-1", MessageID: "m-1"}, "atlas", at, cause)

			if len(chats.replies) != 1 || chats.replies[0].Failure == nil {
				t.Fatalf("replies = %+v, want how the turn ended recorded once", chats.replies)
			}
			if chats.deadlines != 1 {
				t.Error("the record was written with no deadline; a store that hangs would hold shutdown for good")
			}
		})
	}
}
