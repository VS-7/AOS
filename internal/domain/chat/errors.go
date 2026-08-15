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
