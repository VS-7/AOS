// Package activity is the durable event log of a workspace: the notification
// inbox, and the reactive bus that routine triggers listen on.
//
// The two roles are one record on purpose. A notification a person reads and an
// event an automation reacts to are the same fact about the workspace, and
// keeping two logs would mean the inbox and the automation could disagree about
// what happened.
package activity

import (
	"strings"
	"time"
)

// Activity is one thing that happened.
//
// Namespace and Event are the coordinates a routine trigger matches on:
// namespace "task", event "status_changed". Title and Body are what a person
// reads. Data carries the structured payload both the filters and the desktop
// need.
type Activity struct {
	ID string `json:"id" jsonschema:"Identifier of this entry."`

	Namespace string `json:"namespace" jsonschema:"What kind of thing this happened to. Example: \"task\"."`
	Event     string `json:"event" jsonschema:"What happened to it. Example: \"status_changed\"."`

	Title string `json:"title" jsonschema:"One line a person can read without opening anything."`
	Body  string `json:"body,omitempty" jsonschema:"Optional detail, in Markdown."`
	Icon  string `json:"icon,omitempty" jsonschema:"Icon name the desktop app renders."`

	// Data is what a routine filter reads. Its shape is the namespace's, not
	// this package's: a task event carries the task, a memory event the memory.
	Data map[string]any `json:"data,omitempty" jsonschema:"Structured payload of the event, shaped by its namespace."`

	Actor     string `json:"actor" jsonschema:"Who caused it."`
	ActorType string `json:"actorType" jsonschema:"agent, user or system."`

	CreatedAt time.Time `json:"createdAt" jsonschema:"When it happened."`
}

// The three kinds of actor an activity can be attributed to. System is the one
// the original lacks: a purge or a scheduled tick has no person and no agent
// behind it, and attributing it to whoever happened to be logged in is a lie
// the audit trail then carries forever.
const (
	ActorAgent  = "agent"
	ActorUser   = "user"
	ActorSystem = "system"
)

// Key is the coordinate a routine trigger declares.
type Key struct {
	Namespace string
	Event     string
}

// Matches reports whether the activity is the event this key names. An empty
// Event matches every event in the namespace, which is how a routine says "any
// change to a task".
func (k Key) Matches(a Activity) bool {
	if !strings.EqualFold(k.Namespace, a.Namespace) {
		return false
	}
	return k.Event == "" || strings.EqualFold(k.Event, a.Event)
}

// Month is the partition an activity is written to: JSONL under
// .aos/activity/{yyyy-mm}.jsonl. Retention then drops whole files rather than
// rewriting a growing one.
func (a Activity) Month() string { return a.CreatedAt.UTC().Format("2006-01") }

// ReadState is the per-actor overlay of what has been seen.
//
// It is kept beside the log rather than inside it, and that is the decision
// that lets the log stay append-only: marking a notification read must not
// rewrite the record of what happened. The watermark is what makes "mark all as
// read" O(1) instead of a write per entry.
type ReadState struct {
	// Watermark records that everything at or before this instant is read.
	Watermark map[string]time.Time `json:"watermark,omitempty"`

	// IDs records the entries read individually, after the watermark.
	IDs map[string][]string `json:"ids,omitempty"`
}

// IsRead reports whether actor has seen this entry.
func (s ReadState) IsRead(actor string, a Activity) bool {
	if mark, ok := s.Watermark[actor]; ok && !a.CreatedAt.After(mark) {
		return true
	}
	for _, id := range s.IDs[actor] {
		if id == a.ID {
			return true
		}
	}
	return false
}

// Mark records that actor has read one entry, and reports whether that changed
// anything — a caller that saves unconditionally would rewrite the file on
// every re-read of the same notification.
func (s *ReadState) Mark(actor string, a Activity) bool {
	if s.IsRead(actor, a) {
		return false
	}
	if s.IDs == nil {
		s.IDs = map[string][]string{}
	}
	s.IDs[actor] = append(s.IDs[actor], a.ID)
	return true
}

// MarkAll moves the actor's watermark to at, and drops the individual entries
// it now covers.
func (s *ReadState) MarkAll(actor string, at time.Time) {
	if s.Watermark == nil {
		s.Watermark = map[string]time.Time{}
	}
	if current, ok := s.Watermark[actor]; ok && !at.After(current) {
		return
	}
	s.Watermark[actor] = at
	delete(s.IDs, actor)
}
