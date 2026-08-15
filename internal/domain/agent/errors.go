package agent

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errInvalidID(id string) error {
	return apperr.New("AGENT_INVALID_ID").
		Causer("agent.Service.Create").
		Msgf("%q is not a usable agent slug", id).
		Issue("id", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "use a lowercase slug without spaces, such as \"atlas\"",
			Command: build.Name + " agents create atlas",
			Tool:    "agents_create",
		})
}
