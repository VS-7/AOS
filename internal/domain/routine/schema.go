package routine

import (
	"time"

	"github.com/OWNER/aos/internal/core/command"
)

// View is a routine with what the file cannot hold: the real resolution of its
// schedule, when it fires next, and anything wrong with it.
type View struct {
	Routine

	// EffectiveInterval is the scheduler's tick. A cron finer than this fires
	// once per tick, and saying so here is the point: the original leaves the
	// user to discover it.
	EffectiveInterval string `json:"effectiveInterval" jsonschema:"How often the scheduler evaluates cron triggers. A finer cron fires once per window."`

	NextRun  *time.Time `json:"nextRun,omitempty" jsonschema:"When the schedule fires next."`
	Warnings []string   `json:"warnings,omitempty" jsonschema:"Anything about this routine that will surprise you later."`
}

// TriggerInput declares one trigger. It is flatter than the stored form because
// a model writing a discriminated union does better with named fields than with
// a nested config object.
type TriggerInput struct {
	Type TriggerType `json:"type" jsonschema:"One of: scheduled, webhook, activity." validate:"required,notblank"`

	Cron string `json:"cron,omitempty" jsonschema:"Five-field cron, for a scheduled trigger. Example: \"0 9 * * 1-5\"."`

	Namespace string   `json:"namespace,omitempty" jsonschema:"Activity namespace, for an activity trigger. Example: \"task\"."`
	Event     string   `json:"event,omitempty" jsonschema:"Activity event. Empty means any event in the namespace."`
	Filters   []Filter `json:"filters,omitempty" jsonschema:"Conditions on the activity payload. Only for activity triggers."`
}

// ListInput selects routines.
type ListInput struct {
	Agent  string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routines. Defaults to yours."`
	Status Status `json:"status,omitempty" cli:"flag" jsonschema:"Only enabled or only disabled."`

	command.Reasoning
}

// ListOutput is the routines with the scheduler's resolution.
type ListOutput struct {
	Routines []View `json:"routines" jsonschema:"The routines."`
	Total    int    `json:"total" jsonschema:"How many there are."`
	Tick     string `json:"tick" jsonschema:"How often the scheduler evaluates cron triggers."`
}

// GetInput names one routine.
type GetInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the routine." validate:"required,notblank"`
	Agent string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`

	command.Reasoning
}

// CreateInput declares a routine.
type CreateInput struct {
	Name string `json:"name" cli:"arg" jsonschema:"What the routine does. Example: \"Triage new bugs each morning\"." validate:"required,notblank"`

	Agent    string         `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`
	Status   Status         `json:"status,omitempty" cli:"flag" jsonschema:"enabled or disabled. Defaults to enabled."`
	Triggers []TriggerInput `json:"triggers,omitempty" jsonschema:"What makes it fire. Without one it only fires by hand."`
	Scope    Scope          `json:"scope,omitempty" jsonschema:"What it is allowed to do when it runs."`

	Content string `json:"content,omitempty" jsonschema:"The routine's prompt: what to do when it fires."`

	command.Reasoning
}

// CreateOutput carries the routine and, for a webhook trigger, the token.
type CreateOutput struct {
	Routine View `json:"routine" jsonschema:"The routine."`

	// Token is shown once and never again — the file holds only its hash. Copy
	// it now; rotating is the only way to get another.
	Token string `json:"token,omitempty" jsonschema:"The webhook token. Shown once; only a hash is stored. Copy it now."`
}

// UpdateInput changes a routine.
type UpdateInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the routine." validate:"required,notblank"`
	Agent string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`

	Name     *string         `json:"name,omitempty" cli:"flag" jsonschema:"New name."`
	Status   *Status         `json:"status,omitempty" cli:"flag" jsonschema:"enabled or disabled."`
	Triggers *[]TriggerInput `json:"triggers,omitempty" jsonschema:"New triggers. Replaces the old ones whole; a webhook among them mints a new token."`
	Scope    *Scope          `json:"scope,omitempty" jsonschema:"New scope."`
	Content  *string         `json:"content,omitempty" jsonschema:"New prompt."`

	command.Reasoning
}

// RotateInput mints a new webhook token.
type RotateInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the routine." validate:"required,notblank"`
	Agent string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`

	command.Reasoning
}

// RotateOutput carries the new token, once.
type RotateOutput struct {
	Token string `json:"token" jsonschema:"The new webhook token. The previous one stops working immediately."`
}

// DeleteInput removes a routine with its runs.
type DeleteInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the routine." validate:"required,notblank"`
	Agent string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`

	command.Reasoning
}

// DeleteOutput names what went.
type DeleteOutput struct {
	ID    string `json:"id" jsonschema:"The routine that was removed, with its run history."`
	Agent string `json:"agent" jsonschema:"Whose it was."`
}

// FireInput runs a routine now.
type FireInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the routine." validate:"required,notblank"`
	Agent string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`

	Trigger TriggerType    `json:"trigger,omitempty" cli:"flag" jsonschema:"What to record as the cause. Defaults to manual."`
	Payload map[string]any `json:"payload,omitempty" jsonschema:"What to hand the routine."`

	// Force fires a disabled routine. Without it, firing one records a skipped
	// run and refuses — which is what disabled has to mean for it to mean
	// anything.
	Force bool `json:"force,omitempty" cli:"flag" jsonschema:"Fire even if the routine is disabled."`

	command.Reasoning
}

// WebhookInput is an authenticated firing from outside.
type WebhookInput struct {
	ID    string `json:"id" validate:"required,notblank"`
	Agent string `json:"agent,omitempty"`
	Token string `json:"token" validate:"required,notblank"`

	Payload map[string]any `json:"payload,omitempty"`
}

// RunsInput reads a routine's audit history.
type RunsInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the routine." validate:"required,notblank"`
	Agent string `json:"agent,omitempty" cli:"flag" jsonschema:"Whose routine. Defaults to yours."`
	Limit int    `json:"limit,omitempty" cli:"flag" jsonschema:"How many to return, newest first."`

	command.Reasoning
}

// RunsOutput is the audit history.
type RunsOutput struct {
	Routine string `json:"routine" jsonschema:"The routine these runs belong to."`
	Runs    []Run  `json:"runs" jsonschema:"The firings, newest first."`
	Total   int    `json:"total" jsonschema:"How many there are."`
}

// ScheduleOutput is what one tick did.
type ScheduleOutput struct {
	Fired  []string `json:"fired,omitempty" jsonschema:"Routines that fired."`
	Failed []string `json:"failed,omitempty" jsonschema:"Routines that fired and failed."`

	// Broken lists the routines whose cron does not parse. They will never fire
	// and nothing else in the system would say so.
	Broken []string `json:"broken,omitempty" jsonschema:"Routines with a cron expression that does not parse. These never fire."`

	Window string    `json:"window" jsonschema:"The tick window this evaluation covered."`
	At     time.Time `json:"at" jsonschema:"When the tick ran."`
}
