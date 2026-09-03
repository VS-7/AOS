package chat

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errNotFound(id string) error {
	return apperr.New("CHAT_NOT_FOUND").
		Causer("chat.Service.Get").
		Msgf("no conversation %q", id).
		Issue("chat", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the conversations that exist",
			Command: build.Name + " chats list",
			Tool:    "chats_list",
		})
}

func errChannelNotFound(provider, chatID string) error {
	return apperr.New("CHAT_CHANNEL_NOT_FOUND").
		Causer("chat.Service.GetByChannel").
		Msgf("no conversation bound to %s chat %q", provider, chatID).
		Issue("provider", provider).
		Issue("chatId", chatID).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "create one first, with Channel set to this provider and chat id"})
}

func errInvalidKind(got string) error {
	names := make([]string, len(Kinds))
	for i, k := range Kinds {
		names[i] = string(k)
	}
	return apperr.New("CHAT_INVALID_KIND").
		Causer("chat.Service.Create").
		Msgf("%q is not a conversation kind", got).
		Issue("kind", got).
		Issue("allowed", strings.Join(names, ", ")).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "omit the kind to open an ordinary channel",
			Tool:  "chats_create",
		})
}

func errInvalidVisibility(got string) error {
	return apperr.New("CHAT_INVALID_VISIBILITY").
		Causer("chat.Service.Create").
		Msgf("%q is not a visibility", got).
		Issue("visibility", got).
		Issue("allowed", string(VisibilityPrivate)+", "+string(VisibilityWorkspace)).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use private to restrict to the participants, or workspace to open it to every member",
			Tool:  "chats_create",
		})
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("CHAT_WRITE_FAILED").
		Causer("chat.Service." + op).
		Msgf("the conversation could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errReadFailed(op string, cause error) error {
	return apperr.New("CHAT_READ_FAILED").
		Causer("chat.Service." + op).
		Msgf("the conversations could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

// errNoRuntime is what a build with no agent runtime answers when asked to
// stop a turn: the button can never work here, and saying so beats reporting
// "there was nothing running" forever.
func errNoRuntime(chatID string) error {
	return apperr.New("CHAT_NO_RUNTIME").
		Causer("chat.Service.Stop").
		Msgf("this installation has no agent runtime, so no turn can be running").
		Issue("chat", chatID).
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{Label: "start the daemon, which is what runs turns"})
}

// errNoActor refuses a reaction nobody owns. A reaction is a person's mark on
// a message; without an identity there is nothing to attribute it to, and an
// anonymous one could never be removed by whoever left it.
func errNoActor(chatID string) error {
	return apperr.New("CHAT_ACTOR_REQUIRED").
		Causer("chat.Service.React").
		Msgf("a reaction belongs to whoever left it, and this call has no identity").
		Issue("chat", chatID).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "sign in, or say who is calling with --agent"})
}

func errReactionEmpty(chatID string) error {
	return apperr.New("CHAT_REACTION_EMPTY").
		Causer("chat.Service.React").
		Msgf("a reaction with no value is not a reaction").
		Issue("chat", chatID).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "send the emoji to apply, such as \"👍\""})
}

func errMessageNotFound(chatID, messageID string) error {
	return apperr.New("CHAT_MESSAGE_NOT_FOUND").
		Causer("chat.Service.React").
		Msgf("no message %q in conversation %q", messageID, chatID).
		Issue("chat", chatID).
		Issue("message", messageID).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "read the conversation to see which messages it holds", Tool: "chats_get"})
}
