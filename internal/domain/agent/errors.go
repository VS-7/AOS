package agent

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// errInvalidID says what the caller actually gave. A caller who sent only a
// name — the settings form, which has no id field — used to read `"" is not a
// usable agent slug` and a CTA to type an id on the command line.
func errInvalidID(id, name string) error {
	if id == "" {
		return apperr.New("AGENT_INVALID_ID").
			Causer("agent.Service.Create").
			Msgf("%q has no letters or digits to make an agent id from", name).
			Issue("name", name).
			Status(apperr.StatusBadRequest).
			CTA(apperr.CallToAction{
				Label: "use a name with letters or digits, such as \"Atlas\"; the id is made from it",
				Tool:  "agents_create",
				Input: map[string]any{"name": "Atlas"},
			})
	}
	return apperr.New("AGENT_INVALID_ID").
		Causer("agent.Service.Create").
		Msgf("%q is not a usable agent id", id).
		Issue("id", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "use a lowercase id without spaces, such as \"atlas\", or leave it out to make one from the name",
			Command: build.Name + " agents create atlas",
			Tool:    "agents_create",
		})
}

// errAlreadyExists names the agent in the way. The storage layer's own
// sentence named a collection and a key, which tells a person what the file
// system thinks and not which agent to open or which name to change.
func errAlreadyExists(id string, existing *Agent) error {
	return apperr.New("AGENT_ALREADY_EXISTS").
		Causer("agent.Service.Create").
		Msgf("%s already has the id %q", existing.DisplayName(), id).
		Issue("id", id).
		Status(apperr.StatusConflict).
		CTA(
			apperr.CallToAction{
				Label: "choose another name, or change the existing agent instead",
				Tool:  "agents_get",
				Input: map[string]any{"id": id},
			},
		)
}

// errLeaderCycle names the loop it found. A caller told only that the write was
// refused would have to reconstruct the org chart by hand to see why.
func errLeaderCycle(id string, chain []string) error {
	return apperr.New("AGENT_LEADER_CYCLE").
		Causer("agent.Service.checkLeaderChain").
		Msgf("that leader would close a loop in the org chart: %s", strings.Join(chain, " → ")).
		Issue("agent", id).
		Issue("chain", chain).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "point the agent at a leader that does not report back to it",
			Command: build.Name + " agents list",
			Tool:    "agents_list",
		})
}

func errLeaderChainTooDeep(id string, limit int) error {
	return apperr.New("AGENT_LEADER_CHAIN_TOO_DEEP").
		Causer("agent.Service.checkLeaderChain").
		Msgf("the chain of leaders above %q is longer than %d", id, limit).
		Issue("agent", id).
		Issue("limit", limit).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "flatten the hierarchy: an org chart this deep is not walked, it is guessed at",
			Command: build.Name + " agents list",
			Tool:    "agents_list",
		})
}

// errNoOrchestrator is what a human terminal gets when it asks who it is
// talking to and the workspace has nobody to answer.
func errNoOrchestrator() error {
	return apperr.New("AGENT_NO_ORCHESTRATOR").
		Causer("agent.Service.Me").
		Msgf("this workspace has no orchestrator, so there is nobody to answer by default").
		Status(apperr.StatusNotFound).
		CTA(
			apperr.CallToAction{
				Label:   "promote an existing agent",
				Command: build.Name + " agents update atlas --orchestrator",
				Tool:    "agents_update",
				Input:   map[string]any{"id": "atlas", "orchestrator": true},
			},
			apperr.CallToAction{
				Label:   "re-register the workspace, which creates one",
				Command: build.Name + " workspace introspect",
				Tool:    "workspace_introspect",
			},
		)
}
