package memory

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// errAgentRequired is what a caller gets when it tries to write a memory
// without an identity. Memories are personal: a memory with no owner belongs to
// nobody and would be recalled by everybody.
func errAgentRequired(op string) error {
	return apperr.New("MEMORY_AGENT_REQUIRED").
		Causer("memory.Service." + op).
		Msgf("a memory belongs to an agent, and this call has no agent identity").
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label:   "run this as an agent, or name one explicitly",
			Command: build.Name + " agents me",
			Tool:    "agents_me",
		})
}

func errNotFound(agent, id string) error {
	return apperr.New("MEMORY_NOT_FOUND").
		Causer("memory.Service.Reflect").
		Msgf("no memory %q belonging to %q", id, agent).
		Issue("memory", id).
		Issue("agent", agent).
		Status(apperr.StatusNotFound).
		CTA(
			apperr.CallToAction{
				Label:   "search for it instead of addressing it by identifier",
				Command: build.Name + " memories recall --query \"...\"",
				Tool:    "memories_recall",
			},
			apperr.CallToAction{
				Label:   "map what is there",
				Command: build.Name + " memories graph",
				Tool:    "memories_graph",
			},
		)
}

func errInvalidCategory(got string) error {
	names := make([]string, len(Categories))
	for i, c := range Categories {
		names[i] = string(c)
	}
	return apperr.New("MEMORY_INVALID_CATEGORY").
		Causer("memory.Service.Store").
		Msgf("%q is not a memory category", got).
		Issue("category", got).
		Issue("allowed", strings.Join(names, ", ")).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "pick the function of the knowledge, not its writing style",
			Tool:  "memories_store",
		})
}

func errInvalidConfidence(got float64) error {
	return apperr.New("MEMORY_INVALID_CONFIDENCE").
		Causer("memory.Service.Store").
		Msgf("confidence is a number from 0 to 1, and %v is not", got).
		Issue("confidence", got).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "0.9-1.0 for verified, 0.6-0.8 for strong inference, below 0.6 for a guess",
			Tool:  "memories_store",
		})
}

// errSupersedeTargetMissing fires before anything is written. Storing a
// replacement for a memory that is not there would leave a lineage pointing at
// nothing, which is worse than the write failing.
func errSupersedeTargetMissing(id string) error {
	return apperr.New("MEMORY_SUPERSEDE_TARGET_MISSING").
		Causer("memory.Service.Store").
		Msgf("cannot supersede %q: no such memory", id).
		Issue("memory", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "recall first to find the identifier of the trace you are replacing",
			Command: build.Name + " memories recall --query \"...\"",
			Tool:    "memories_recall",
		})
}

// errReasonTooShort keeps the lineage worth having. The five-character minimum
// is the original's, and the prompt that goes with it is explicit: when in
// doubt, lower the confidence rather than forgetting.
func errReasonTooShort(id string) error {
	return apperr.New("MEMORY_REASON_TOO_SHORT").
		Causer("memory.Service.Forget").
		Msgf("deprecating a memory needs a real reason, not a placeholder").
		Issue("memory", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "say what changed in the world that makes this no longer true — or lower its confidence instead of forgetting it",
			Tool:  "memories_forget",
		})
}

func errAlreadyDeprecated(id string) error {
	return apperr.New("MEMORY_ALREADY_DEPRECATED").
		Causer("memory.Service.Forget").
		Msgf("memory %q is already deprecated", id).
		Issue("memory", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "read it to see why, and when",
			Command: build.Name + " memories reflect " + id,
			Tool:    "memories_reflect",
			Input:   map[string]any{"memory": id},
		})
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("MEMORY_WRITE_FAILED").
		Causer("memory.Service." + op).
		Msgf("the memory could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errReadFailed(op string, cause error) error {
	return apperr.New("MEMORY_READ_FAILED").
		Causer("memory.Service." + op).
		Msgf("the memories could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errInvalidStatus(got string) error {
	names := make([]string, len(Statuses))
	for i, s := range Statuses {
		names[i] = string(s)
	}
	return apperr.New("MEMORY_INVALID_STATUS").
		Causer("memory.Service.Recall").
		Msgf("%q is not a memory status", got).
		Issue("status", got).
		Issue("allowed", strings.Join(names, ", ")).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "omit the filter to see active memories, which is the default",
			Tool:  "memories_recall",
		})
}

func errInvalidExpiry(got string, cause error) error {
	return apperr.New("MEMORY_INVALID_EXPIRY").
		Causer("memory.Service.Store").
		Msgf("%q is not an RFC 3339 timestamp", got).
		Issue("expiresAt", got).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "write it as 2026-12-01T00:00:00Z",
			Tool:  "memories_store",
		})
}
