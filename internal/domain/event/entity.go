// Package event is the hook system: nine points where policy can watch the
// agent, add to what it knows, rewrite what a tool is about to do, or stop it.
//
// The contract is Claude Code's, deliberately (ADR-0016). A hook written for
// that tool runs here without modification, which is what makes the ecosystem
// of hooks people already have worth anything to this system.
//
// Two properties are worth stating up front, because they are what makes the
// audit log trustworthy. Nothing but a real execution produces a record: there
// is no create command, no MCP tool and no update path anywhere in the tree.
// And every invocation is recorded, including the handlers that decided
// nothing — a log with gaps cannot answer "why was that tool blocked".
package event

import (
	"encoding/json"
	"strings"
	"time"
)

// Type is one of the nine points in the agent's turn.
type Type string

// The nine events, with the names of the contract they implement.
const (
	SessionStart       Type = "SessionStart"       // a session begins
	UserPromptSubmit   Type = "UserPromptSubmit"   // the human said something; may block
	PreToolUse         Type = "PreToolUse"         // a tool is about to run; may block or rewrite
	PostToolUse        Type = "PostToolUse"        // a tool returned; may block what follows
	PostToolUseFailure Type = "PostToolUseFailure" // a tool failed
	SubagentStart      Type = "SubagentStart"      // a delegated turn begins
	SubagentStop       Type = "SubagentStop"       // a delegated turn ends; may block
	Stop               Type = "Stop"               // the turn ends; may block
	PreCompact         Type = "PreCompact"         // the history is about to be pruned
	Generic            Type = "GenericHook"        // anything a skill defines for itself
)

// Types lists the nine plus the generic one, in the order they can occur.
var Types = []Type{
	SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, PostToolUseFailure,
	SubagentStart, SubagentStop, Stop, PreCompact, Generic,
}

// Valid reports whether t is a known event type.
func (t Type) Valid() bool {
	for _, known := range Types {
		if t == known {
			return true
		}
	}
	return false
}

// CanBlock reports whether a handler for this event may stop what comes next.
//
// The table is part of the contract rather than a convention: a handler that
// returns "block" on SessionStart is making a promise the runtime does not
// keep, and silently ignoring it would be the worst of the three options.
func (t Type) CanBlock() bool {
	switch t {
	case UserPromptSubmit, PreToolUse, PostToolUse, SubagentStop, Stop:
		return true
	default:
		return false
	}
}

// Event is one occurrence handed to the registered handlers.
//
// Input is the tool payload on PreToolUse and the only field a handler may
// rewrite; Output is what the tool returned on PostToolUse. Both are raw JSON
// because a hook is an arbitrary program and the runtime has no business
// re-encoding a payload it does not understand.
type Event struct {
	Type      Type   `json:"hook_event_name"`
	SessionID string `json:"session_id,omitempty"`
	Agent     string `json:"agent_id,omitempty"`
	Workspace string `json:"workspace_id,omitempty"`
	Directory string `json:"cwd,omitempty"`

	Prompt string `json:"prompt,omitempty"` // UserPromptSubmit

	Tool   string          `json:"tool_name,omitempty"`     // Pre/PostToolUse
	Input  json.RawMessage `json:"tool_input,omitempty"`    // Pre/PostToolUse
	Output json.RawMessage `json:"tool_response,omitempty"` // PostToolUse
	Reason string          `json:"error,omitempty"`         // PostToolUseFailure

	Trigger string `json:"trigger,omitempty"` // PreCompact: auto | manual
	At      time.Time
}

// Decision values a handler may return.
const (
	DecisionBlock    = "block"
	DecisionContinue = "continue"
)

// Permission decisions a PreToolUse handler may return.
const (
	PermissionAllow = "allow"
	PermissionDeny  = "deny"
	PermissionAsk   = "ask"
)

// Outcome is what a handler decided, and what the bus accumulates across them.
type Outcome struct {
	Decision           string          `json:"decision,omitempty"`
	PermissionDecision string          `json:"permissionDecision,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	AdditionalContext  string          `json:"additionalContext,omitempty"`
	UpdatedInput       json.RawMessage `json:"updatedInput,omitempty"`

	// HookID names the handler that decided. Without it, an installation with
	// four skills cannot answer "which one blocked this", and the answer to
	// that question is the difference between an audit log and a diary.
	HookID string `json:"hookId,omitempty"`
}

// Blocked reports whether this outcome stops what comes next.
func (o Outcome) Blocked() bool {
	return o.Decision == DecisionBlock || o.PermissionDecision == PermissionDeny
}

// Merge folds a handler's outcome into the accumulated one.
//
// Context accumulates: two handlers each adding a paragraph both get heard.
// Decisions do not: the first handler to decide owns the decision, and a later
// handler cannot quietly widen a permission another one narrowed.
func (o *Outcome) Merge(next Outcome) {
	if next.AdditionalContext != "" {
		if o.AdditionalContext == "" {
			o.AdditionalContext = next.AdditionalContext
		} else {
			o.AdditionalContext += "\n\n" + next.AdditionalContext
		}
	}
	if len(next.UpdatedInput) > 0 {
		o.UpdatedInput = next.UpdatedInput
	}
	if o.Decision == "" && next.Decision != "" {
		o.Decision = next.Decision
	}
	if o.PermissionDecision == "" && next.PermissionDecision != "" {
		o.PermissionDecision = next.PermissionDecision
	}
	if o.Reason == "" && next.Reason != "" {
		o.Reason = next.Reason
	}
	if o.HookID == "" && next.HookID != "" {
		o.HookID = next.HookID
	}
}

// Record is one line of the append-only audit log.
type Record struct {
	ID        string          `json:"id"`
	Agent     string          `json:"agent,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Type      Type            `json:"type"`
	Hook      string          `json:"hook,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Outcome   Outcome         `json:"outcome"`
	Error     string          `json:"error,omitempty"`
	Duration  int64           `json:"durationMs"`
	CreatedAt time.Time       `json:"createdAt"`
}

// RiskLevel is how much a tool call can hurt, derived from the tool's own
// annotations rather than from a list somebody has to maintain.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Risk classifies a call from what the tool declares about itself.
//
// Destructive and open-world are the two properties that matter: the first can
// undo work, the second reaches outside the machine. Read-only is the floor.
func Risk(readOnly, destructive, openWorld bool) RiskLevel {
	switch {
	case destructive:
		return RiskHigh
	case openWorld:
		return RiskMedium
	case readOnly:
		return RiskLow
	default:
		return RiskMedium
	}
}

// RememberScope is how long a human's approval lasts.
//
// The master prompt is explicit that approving once is not approving forever,
// which is why session is the default the interface suggests and always takes
// a second, deliberate action.
type RememberScope string

const (
	RememberNone    RememberScope = "none"
	RememberSession RememberScope = "session"
	RememberAlways  RememberScope = "always"
)

// ApprovalRequest is one tool call waiting on a human.
type ApprovalRequest struct {
	ID        string          `json:"id" jsonschema:"Identifier of the pending request."`
	SessionID string          `json:"sessionId,omitempty" jsonschema:"Session the call belongs to."`
	Workspace string          `json:"workspace,omitempty" jsonschema:"Workspace the call belongs to."`
	AgentID   string          `json:"agent,omitempty" jsonschema:"Agent that wants to make the call."`
	ToolName  string          `json:"tool" jsonschema:"Tool the agent is asking to run."`
	Input     json.RawMessage `json:"input,omitempty" jsonschema:"Payload the tool would be called with."`
	Reason    string          `json:"reason,omitempty" jsonschema:"Why the hook asked instead of deciding."`
	Risk      RiskLevel       `json:"risk" jsonschema:"low, medium or high, derived from the tool's annotations."`
	Deadline  time.Duration   `json:"-"`
	ExpiresAt time.Time       `json:"expiresAt" jsonschema:"When the request stops waiting and becomes a denial."`
	CreatedAt time.Time       `json:"createdAt" jsonschema:"When the agent asked."`
}

// ApprovalResult is the human's answer.
type ApprovalResult struct {
	Approved bool          `json:"approved"`
	Reason   string        `json:"reason,omitempty"`
	Remember RememberScope `json:"remember,omitempty"`

	// UpdatedInput lets the person fix the payload before saying yes, which is
	// often what they actually want: not "no", but "not like that".
	UpdatedInput json.RawMessage `json:"updatedInput,omitempty"`
}

// DefaultApprovalDeadline is how long a request waits before it becomes a
// denial. It is a denial and never an approval: a timeout is the absence of a
// decision, and the absence of a decision cannot mean yes.
const DefaultApprovalDeadline = 120 * time.Second

// LogDay is the file a record belongs to. Rotation is by day and by agent, so
// pruning removes whole files and never edits one — an audit log that gets
// rewritten to drop old lines is an audit log somebody can rewrite.
func LogDay(t time.Time) string { return t.UTC().Format("2006-01-02") }

// SanitizeAgent makes an agent id safe to use as a directory name. An id is a
// slug everywhere it is created, but the log is written from a payload that may
// have come from outside, and a path is not the place to find out.
func SanitizeAgent(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "unknown"
	}
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, id)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "unknown"
	}
	return clean
}
