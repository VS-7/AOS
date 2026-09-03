package activity

import (
	"time"

	"github.com/OWNER/aos/internal/core/command"
)

// ListInput selects a page of the inbox.
type ListInput struct {
	Namespace string `json:"namespace,omitempty" cli:"flag" jsonschema:"Only entries about this kind of thing. Example: \"task\"."`
	Event     string `json:"event,omitempty" cli:"flag" jsonschema:"Only this event within the namespace. Example: \"status_changed\"."`
	Unread    bool   `json:"unread,omitempty" cli:"flag" jsonschema:"Only what you have not read yet."`
	Since     string `json:"since,omitempty" cli:"flag" jsonschema:"RFC3339 instant; only entries at or after it."`

	// Actor overrides whose inbox is read. It exists for the desktop, which
	// shows a person their own inbox while the request carries no agent.
	Actor string `json:"actor,omitempty" cli:"flag" jsonschema:"Whose read state to apply. Defaults to you."`

	Limit  int `json:"limit,omitempty" cli:"flag" jsonschema:"How many to return. Defaults to 50."`
	Offset int `json:"offset,omitempty" cli:"flag" jsonschema:"How many to skip."`

	command.Reasoning
}

// ListOutput is a page of the inbox.
type ListOutput struct {
	Activities []Activity `json:"activities" jsonschema:"The entries, newest first."`
	Total      int        `json:"total" jsonschema:"How many matched before the page was cut."`
	Unread     int        `json:"unread" jsonschema:"How many of the matches you have not read."`
	Actor      string     `json:"actor" jsonschema:"Whose read state was applied."`
}

// GetInput names one entry.
type GetInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the entry." validate:"required,notblank"`

	command.Reasoning
}

// PublishInput records something that happened.
//
// There is no Actor field, and that absence is the same rule Comment enforces:
// authorship comes from the ambient identity, never from the payload.
type PublishInput struct {
	Namespace string `json:"namespace" cli:"arg" jsonschema:"What kind of thing this happened to. Example: \"task\"." validate:"required,notblank"`
	Event     string `json:"event" cli:"arg" jsonschema:"What happened. Example: \"status_changed\"." validate:"required,notblank"`
	Title     string `json:"title" jsonschema:"One line a person can read without opening anything." validate:"required,notblank"`

	Body string `json:"body,omitempty" jsonschema:"Optional detail, in Markdown."`
	Icon string `json:"icon,omitempty" jsonschema:"Icon name the desktop app renders."`

	Data map[string]any `json:"data,omitempty" jsonschema:"Structured payload routine filters read."`

	command.Reasoning
}

// MarkInput marks one entry read.
type MarkInput struct {
	ID    string `json:"id" cli:"arg" jsonschema:"Identifier of the entry." validate:"required,notblank"`
	Actor string `json:"actor,omitempty" cli:"flag" jsonschema:"Whose read state to change. Defaults to you."`

	command.Reasoning
}

// MarkAllInput marks the whole inbox read.
type MarkAllInput struct {
	Actor string `json:"actor,omitempty" cli:"flag" jsonschema:"Whose read state to change. Defaults to you."`

	command.Reasoning
}

// MarkOutput reports whether the read state moved.
type MarkOutput struct {
	Actor   string `json:"actor" jsonschema:"Whose read state was changed."`
	Changed bool   `json:"changed" jsonschema:"False when it was already read."`
}

// DeleteInput removes one entry.
type DeleteInput struct {
	ID string `json:"id" cli:"arg" jsonschema:"Identifier of the entry." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput names what was removed and which partition was rewritten.
type DeleteOutput struct {
	ID    string `json:"id" jsonschema:"The entry that was removed."`
	Month string `json:"month" jsonschema:"The partition that had to be rewritten to remove it."`
}

// PurgeInput drops what is past the retention window.
type PurgeInput struct {
	OlderThanDays int `json:"olderThanDays,omitempty" cli:"flag" jsonschema:"Override the retention window, in days. Defaults to the configured 90."`

	command.Reasoning
}

// PurgeOutput says exactly what the purge did.
//
// Dropped and Rewritten are separate because they are different operations on
// the log: a dropped partition is a file removed, a rewritten one is history
// edited in place. Only the second can lose an entry to a crash, so a caller
// that wants to know whether that risk was taken can see it.
type PurgeOutput struct {
	Removed   int       `json:"removed" jsonschema:"How many entries were removed."`
	Dropped   []string  `json:"dropped,omitempty" jsonschema:"Partitions removed whole."`
	Rewritten []string  `json:"rewritten,omitempty" jsonschema:"Partitions rewritten because only part of them expired."`
	Cutoff    time.Time `json:"cutoff" jsonschema:"Everything before this instant was removed."`
}

// EventsInput narrows the catalogue to one namespace.
type EventsInput struct {
	Namespace string `json:"namespace,omitempty" cli:"flag" jsonschema:"Only the events of this namespace. Empty means all of them."`

	command.Reasoning
}

// EventsOutput is the catalogue of what a routine can react to.
type EventsOutput struct {
	Events     []EventKind `json:"events" jsonschema:"Every event kind, ordered by namespace and then by lifecycle."`
	Namespaces []string    `json:"namespaces" jsonschema:"The namespaces present, in the same order."`
}
