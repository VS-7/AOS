package instruction

import "github.com/OWNER/aos/internal/core/apperr"

// errNameRequired fires when Create is asked to name an instruction with
// nothing to derive a slug from.
func errNameRequired() error {
	return apperr.New("INSTRUCTION_NAME_REQUIRED").
		Causer("instruction.Service.Create").
		Msgf("an instruction needs a name to derive its identifier from").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "give the instruction a name"})
}

// errNotFound fires when no instruction has this ID.
func errNotFound(id string) error {
	return apperr.New("INSTRUCTION_NOT_FOUND").
		Causer("instruction.Service.Get").
		Msgf("no instruction %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "list configured instructions",
			Tool:  "instructions_list",
		})
}

// errAlreadyExists fires when Create's derived slug collides with one that
// already exists — the original's own guard, kept: two instructions sharing
// an id would mean one silently overwrites the other's file on disk.
func errAlreadyExists(id string) error {
	return apperr.New("INSTRUCTION_ALREADY_EXISTS").
		Causer("instruction.Service.Create").
		Msgf("an instruction %q already exists", id).
		Issue("id", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "update the existing instruction instead, or choose a different name",
			Tool:  "instructions_update",
		})
}

// errNotApproved fires when Create or Update is called by an agent and a
// human either refused, or nothing was there to ask — a timeout is a
// denial, never an approval (event.Broker's own invariant), so this is the
// one path out of a missing "yes".
func errNotApproved(op, id, reason string) error {
	return apperr.New("INSTRUCTION_NOT_APPROVED").
		Causer("instruction.Service."+op).
		Msgf("%s of instruction %q was not approved: %s", op, id, reason).
		Issue("id", id).
		Issue("reason", reason).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "a human must approve a workspace-wide instruction change before it is written; retry once approved",
		})
}

// errReadFailed wraps a repository failure that is not "not found".
func errReadFailed(op string, cause error) error {
	return apperr.New("INSTRUCTION_READ_FAILED").
		Causer("instruction.Service."+op).
		Msgf("could not read instructions: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

// errWriteFailed wraps a repository failure while writing.
func errWriteFailed(op string, cause error) error {
	return apperr.New("INSTRUCTION_WRITE_FAILED").
		Causer("instruction.Service."+op).
		Msgf("could not save the instruction: %v", cause).
		Issue("operation", op).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
