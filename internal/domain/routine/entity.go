// Package routine is the durable entry point for autonomous work.
//
// A routine belongs to an agent, not to the workspace — the agent is in its
// path — and it fires from one of three triggers: a schedule, a webhook, or an
// activity in the workspace. Every firing records a run, successful or not. A
// routine without a run history is exactly what the master prompt warns about:
// only a promise that something happened.
package routine

import (
	"strings"
	"time"
)

// Status is whether a routine may fire at all.
type Status string

const (
	Enabled  Status = "enabled"
	Disabled Status = "disabled"
)

// Valid reports whether s is one of the two.
func (s Status) Valid() bool { return s == Enabled || s == Disabled }

// TriggerType is the discriminant of the trigger union.
type TriggerType string

const (
	// Scheduled fires on a cron expression, evaluated once per tick window.
	Scheduled TriggerType = "scheduled"

	// Webhook fires on an authenticated POST.
	Webhook TriggerType = "webhook"

	// Activity fires on an event inside the workspace. This is the reactive
	// half of the system: "when a bug enters in_review, run this".
	Activity TriggerType = "activity"
)

// TriggerTypes lists the three.
var TriggerTypes = []TriggerType{Scheduled, Webhook, Activity}

// EnumValues publishes the trigger kinds to every schema.
func (t TriggerType) EnumValues() []string {
	out := make([]string, 0, len(TriggerTypes))
	for _, v := range TriggerTypes {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether t is one of the three.
func (t TriggerType) Valid() bool {
	for _, known := range TriggerTypes {
		if t == known {
			return true
		}
	}
	return false
}

// Routine is one durable entry point.
type Routine struct {
	// Agent and ID come from the path: a routine lives at
	// .aos/agents/{agent}/routines/{id}/ROUTINE.md, and deleting it removes
	// the directory with its runs.
	Agent string `yaml:"-" json:"agent" collection:"path" jsonschema:"Slug of the agent this routine belongs to."`
	ID    string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this routine."`

	Name     string    `yaml:"name" json:"name" jsonschema:"What the routine does. Example: \"Triage new bugs each morning\"."`
	Triggers []Trigger `yaml:"triggers" json:"triggers" jsonschema:"What makes it fire. A routine with no trigger only fires by hand."`
	Status   Status    `yaml:"status" json:"status" jsonschema:"enabled or disabled."`

	// Scope is what this routine may do. The master prompt forbids a routine
	// from creating routines, agents or external effects "unless explicitly
	// allowed by its configuration" — and the original has nowhere to put that
	// configuration. This is it, and the tool registry is filtered by it.
	Scope Scope `yaml:"scope,omitempty" json:"scope,omitempty" jsonschema:"What this routine is allowed to do when it runs."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When it was created."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it last changed."`

	// LastFiredAt is what the tick window is measured from, so a cron that was
	// missed while the daemon was down fires once on the next tick rather than
	// once for every window that passed.
	LastFiredAt *time.Time `yaml:"lastFiredAt,omitempty" json:"lastFiredAt,omitempty" jsonschema:"When it last fired."`

	Content string `yaml:"-" json:"content,omitempty" collection:"content" jsonschema:"The routine's prompt, in Markdown: what the agent is to do when it fires."`
}

// Trigger is a discriminated union over TriggerType.
type Trigger struct {
	Type   TriggerType   `yaml:"type" json:"type" jsonschema:"One of: scheduled, webhook, activity."`
	Config TriggerConfig `yaml:"config" json:"config" jsonschema:"The trigger's settings, shaped by its type."`

	// Filters narrow an activity trigger to the events that matter. They are
	// meaningless on the other two and rejected there rather than ignored.
	Filters []Filter `yaml:"filters,omitempty" json:"filters,omitempty" jsonschema:"Conditions on the activity payload. Only for activity triggers."`
}

// TriggerConfig is the union's payload. Which fields are read depends on Type,
// and the ones that do not belong are rejected at validation rather than left
// to confuse whoever reads the file later.
type TriggerConfig struct {
	Cron string `yaml:"cron,omitempty" json:"cron,omitempty" jsonschema:"Five-field cron expression, for a scheduled trigger. Example: \"0 9 * * 1-5\"."`

	// TokenHash is the webhook secret at rest. The original stores the token in
	// clear in front matter that is committed to Git; here the file holds only
	// a hash, the token is shown once at creation, and it can be rotated.
	TokenHash string `yaml:"tokenHash,omitempty" json:"tokenHash,omitempty" jsonschema:"Hash of the webhook token. The token itself is shown once, at creation."`

	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" jsonschema:"Activity namespace to react to. Example: \"task\"."`
	Event     string `yaml:"event,omitempty" json:"event,omitempty" jsonschema:"Activity event to react to. Empty means any event in the namespace."`
}

// Filter is one condition on an activity payload.
type Filter struct {
	Field    string `yaml:"field" json:"field" jsonschema:"Key in the activity's data. Example: \"type\"."`
	Operator string `yaml:"operator" json:"operator" jsonschema:"One of: eq, neq, contains."`
	Value    any    `yaml:"value" json:"value" jsonschema:"What to compare against."`
}

// The three operators a filter can use.
const (
	OpEq       = "eq"
	OpNeq      = "neq"
	OpContains = "contains"
)

// Scope declares what a routine may do while it runs.
type Scope struct {
	AllowCreateTasks   bool     `yaml:"allowCreateTasks" json:"allowCreateTasks" jsonschema:"Let this routine create tasks."`
	AllowExternalCalls bool     `yaml:"allowExternalCalls" json:"allowExternalCalls" jsonschema:"Let this routine reach outside the machine."`
	AllowedTools       []string `yaml:"allowedTools,omitempty" json:"allowedTools,omitempty" jsonschema:"Exact set of tools it may use. Empty means the agent's usual registry, minus what the flags above withhold."`
}

// Allows reports whether a routine running under this scope may use a tool.
//
// The allowlist, when present, is exhaustive: a routine that declares its tools
// gets those and nothing else. Without one, the two flags withhold the two
// classes of action the master prompt singles out.
func (s Scope) Allows(tool string) bool {
	if len(s.AllowedTools) > 0 {
		for _, allowed := range s.AllowedTools {
			if allowed == tool {
				return true
			}
		}
		return false
	}
	if !s.AllowCreateTasks && tool == "tasks_create" {
		return false
	}
	if !s.AllowExternalCalls && isExternal(tool) {
		return false
	}
	return true
}

// isExternal names the tools that reach off the machine. It is a prefix match
// on the group, because a group whose whole purpose is the network cannot have
// a member that is not.
func isExternal(tool string) bool {
	for _, prefix := range []string{"web_", "fetch_", "gateway_"} {
		if strings.HasPrefix(tool, prefix) {
			return true
		}
	}
	return false
}

// RunStatus is how a firing ended.
type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunTimedOut  RunStatus = "timed_out"
	RunSkipped   RunStatus = "skipped"
)

// Run is the audit record of one firing.
//
// Every firing produces one, including the ones that fail and the ones that are
// skipped. Failing without a record is the worst outcome for an audit trail:
// afterwards nobody can tell the difference between "it ran and did nothing"
// and "it never ran".
type Run struct {
	// Agent, Routine and ID come from the path.
	Agent   string `json:"agent" collection:"path"`
	Routine string `json:"routine" collection:"path"`
	ID      string `json:"id" collection:"path"`

	Trigger TriggerType    `json:"trigger" jsonschema:"Which kind of trigger fired it."`
	Payload map[string]any `json:"payload,omitempty" jsonschema:"What the trigger carried."`
	ChatID  string         `json:"chatId,omitempty" jsonschema:"The conversation the run executed in."`

	Status    RunStatus  `json:"status" jsonschema:"running, succeeded, failed, timed_out or skipped."`
	StartedAt time.Time  `json:"startedAt" jsonschema:"When it began."`
	EndedAt   *time.Time `json:"endedAt,omitempty" jsonschema:"When it ended."`
	Error     string     `json:"error,omitempty" jsonschema:"Why it failed."`

	Usage Usage `json:"usage" jsonschema:"What the run cost."`
}

// Usage is what one run consumed.
type Usage struct {
	Input   int     `json:"input"`
	Output  int     `json:"output"`
	Total   int     `json:"total"`
	CostUSD float64 `json:"costUsd"`
}

// Matches reports whether an activity fires this trigger.
func (t Trigger) Matches(namespace, event string, data map[string]any) bool {
	if t.Type != Activity {
		return false
	}
	if !strings.EqualFold(t.Config.Namespace, namespace) {
		return false
	}
	if t.Config.Event != "" && !strings.EqualFold(t.Config.Event, event) {
		return false
	}
	for _, f := range t.Filters {
		if !f.Matches(data) {
			return false
		}
	}
	return true
}

// Matches reports whether one filter holds for a payload.
//
// A missing field never matches, including under neq. That is deliberate and it
// is the one place this differs from a naive reading: "type is not bug" firing
// for an event that has no type at all would make every unrelated activity in
// the namespace trigger the routine.
func (f Filter) Matches(data map[string]any) bool {
	raw, present := data[f.Field]
	if !present {
		return false
	}
	got, want := render(raw), render(f.Value)
	switch f.Operator {
	case OpEq:
		return got == want
	case OpNeq:
		return got != want
	case OpContains:
		return strings.Contains(strings.ToLower(got), strings.ToLower(want))
	default:
		return false
	}
}
