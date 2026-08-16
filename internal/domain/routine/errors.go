package routine

import (
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errAgentRequired(op string) error {
	return apperr.New("ROUTINE_AGENT_REQUIRED").
		Causer("routine.Service." + op).
		Msgf("a routine belongs to an agent, and this request names none").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "pass the agent whose routine this is",
			Command: build.Name + " routines list --agent atlas",
			Tool:    "routines_list",
		})
}

func errNoSuchAgent(agent string) error {
	return apperr.New("ROUTINE_NO_SUCH_AGENT").
		Causer("routine.Service.Create").
		Msgf("no agent named %q exists to own this routine", agent).
		Issue("agent", agent).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the agents that exist",
			Command: build.Name + " agents list",
			Tool:    "agents_list",
		})
}

func errNotFound(agent, id string) error {
	return apperr.New("ROUTINE_NOT_FOUND").
		Causer("routine.Service.Get").
		Msgf("no routine %q belongs to %q", id, agent).
		Issue("agent", agent).
		Issue("routine", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the routines that exist",
			Command: build.Name + " routines list",
			Tool:    "routines_list",
		})
}

func errInvalidName(name string) error {
	return apperr.New("ROUTINE_INVALID_NAME").
		Causer("routine.Service.Create").
		Msgf("%q does not name a routine", name).
		Issue("name", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "say what the routine does, in a few words",
		})
}

func errInvalidStatus(status string) error {
	return apperr.New("ROUTINE_INVALID_STATUS").
		Causer("routine.Service.Create").
		Msgf("%q is not a routine status", status).
		Issue("status", status).
		Issue("valid", []string{"enabled", "disabled"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use enabled or disabled"})
}

func errUnknownTriggerType(kind string) error {
	return apperr.New("ROUTINE_UNKNOWN_TRIGGER").
		Causer("routine.Service.buildTriggers").
		Msgf("%q is not a kind of trigger", kind).
		Issue("type", kind).
		Issue("valid", []string{"scheduled", "webhook", "activity"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "a routine fires on a schedule, on an authenticated webhook, or on an activity in the workspace",
		})
}

func errInvalidCron(expr string, cause error) error {
	return apperr.New("ROUTINE_INVALID_CRON").
		Causer("routine.Service.buildTriggers").
		Msgf("%q is not a cron expression this scheduler understands", expr).
		Issue("cron", expr).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: `five fields: minute hour day-of-month month day-of-week. "0 9 * * 1-5" is weekdays at nine`,
		})
}

// errFiltersNotApplicable refuses a filter on a trigger that cannot use one,
// rather than storing it where it will silently do nothing.
func errFiltersNotApplicable(kind string) error {
	return apperr.New("ROUTINE_FILTERS_NOT_APPLICABLE").
		Causer("routine.Service.buildTriggers").
		Msgf("filters read an activity payload, and a %s trigger has none", kind).
		Issue("type", kind).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "drop the filters, or make this an activity trigger",
		})
}

func errActivityNamespaceRequired() error {
	return apperr.New("ROUTINE_ACTIVITY_NAMESPACE_REQUIRED").
		Causer("routine.Service.buildTriggers").
		Msgf("an activity trigger has to say what kind of thing it reacts to").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: `name the namespace, as in "task"; leave the event empty to react to any change in it`,
		})
}

func errUnknownOperator(op string) error {
	return apperr.New("ROUTINE_UNKNOWN_OPERATOR").
		Causer("routine.Service.buildTriggers").
		Msgf("%q is not a filter operator", op).
		Issue("operator", op).
		Issue("valid", []string{"eq", "neq", "contains"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use eq, neq or contains"})
}

func errFilterFieldRequired() error {
	return apperr.New("ROUTINE_FILTER_FIELD_REQUIRED").
		Causer("routine.Service.buildTriggers").
		Msgf("a filter with no field compares nothing").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: `name a key of the activity payload, as in "type"`,
		})
}

func errTokensUnavailable() error {
	return apperr.New("ROUTINE_TOKENS_UNAVAILABLE").
		Causer("routine.Service.buildTriggers").
		Msgf("this installation cannot mint webhook tokens").
		Status(apperr.StatusNotImplemented).
		CTA(apperr.CallToAction{
			Label: "use a scheduled or activity trigger instead",
		})
}

func errTokenMint(cause error) error {
	return apperr.New("ROUTINE_TOKEN_MINT_FAILED").
		Causer("routine.Service.buildTriggers").
		Msgf("a webhook token could not be generated").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errNoWebhookTrigger(id string) error {
	return apperr.New("ROUTINE_NO_WEBHOOK_TRIGGER").
		Causer("routine.Service.Rotate").
		Msgf("this routine has no webhook trigger, so there is no token to rotate").
		Issue("routine", id).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{
			Label: "add a webhook trigger, which mints a token as it is created",
			Tool:  "routines_update",
		})
}

// errInvalidToken says nothing about which part was wrong. A webhook endpoint
// that distinguishes "no such routine" from "wrong token" is an oracle.
func errInvalidToken(id string) error {
	return apperr.New("ROUTINE_FIRE_INVALID_TOKEN").
		Causer("routine.Service.FireWebhook").
		Msgf("this request is not authorised to fire that routine").
		Issue("routine", id).
		Status(apperr.StatusUnauthorized).
		CTA(apperr.CallToAction{
			Label: "the token is shown once, when the trigger is created; rotate it if it was lost",
			Tool:  "routines_rotate",
		})
}

func errDisabled(id string) error {
	return apperr.New("ROUTINE_DISABLED").
		Causer("routine.Service.fire").
		Msgf("this routine is disabled, so it did not run").
		Issue("routine", id).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "enable it, or fire it with force to run it once without enabling it",
			Tool:  "routines_fire",
			Input: map[string]any{"id": id, "force": true},
		})
}

func errNoExecutor(id string) error {
	return apperr.New("ROUTINE_NO_EXECUTOR").
		Causer("routine.Service.fire").
		Msgf("this installation has no runtime to execute a routine").
		Issue("routine", id).
		Status(apperr.StatusServiceUnavailable).
		CTA(apperr.CallToAction{
			Label:   "start the daemon; routines run there, not in the terminal",
			Command: build.Name + " gateway start",
			Tool:    "gateway_start",
		})
}

func errRunFailed(id string, cause error) error {
	return apperr.New("ROUTINE_RUN_FAILED").
		Causer("routine.Service.fire").
		Msgf("the routine ran and failed").
		Issue("routine", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label:   "the run was recorded either way; read it to see how far it got",
			Command: build.Name + " routines runs " + id,
			Tool:    "routines_runs",
			Input:   map[string]any{"id": id},
		})
}

func errReadFailed(op string, cause error) error {
	return apperr.New("ROUTINE_READ_FAILED").
		Causer("routine.Service." + op).
		Msgf("the routine could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errWriteFailed(op string, cause error) error {
	return apperr.New("ROUTINE_WRITE_FAILED").
		Causer("routine.Service." + op).
		Msgf("the routine could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
