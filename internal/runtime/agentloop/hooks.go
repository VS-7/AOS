package agentloop

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/runtime/toolexec"
)

// Decision is what the approval point returns.
type Decision struct {
	Allow  bool
	Reason string

	// UpdatedInput replaces the payload before the tool runs. It is the third
	// superpower of the hook system and the most dangerous: whoever controls
	// the hooks controls what the agent actually does, regardless of what the
	// model decided. That is why every rewrite is in the audit log with the id
	// of the hook that made it.
	UpdatedInput json.RawMessage

	AddContext string
}

// Hooks are the five points where policy intervenes in a turn.
//
// They are an explicit interface rather than five callbacks on a config struct
// because the set is the contract: a reader can see all five, and a new one
// cannot be added without every implementation noticing.
type Hooks interface {
	PrepareCall(ctx context.Context, s *State) error
	PrepareStep(ctx context.Context, s *State) error
	ApproveTool(ctx context.Context, c *ToolCall, s *State) (Decision, error)
	AfterTool(ctx context.Context, c *ToolCall, r *ToolResult, s *State) error
	OnEnd(ctx context.Context, s *State) error
}

// NoHooks is a turn with no policy attached.
type NoHooks struct{}

func (NoHooks) PrepareCall(context.Context, *State) error { return nil }
func (NoHooks) PrepareStep(context.Context, *State) error { return nil }
func (NoHooks) ApproveTool(context.Context, *ToolCall, *State) (Decision, error) {
	return Decision{Allow: true}, nil
}
func (NoHooks) AfterTool(context.Context, *ToolCall, *ToolResult, *State) error { return nil }
func (NoHooks) OnEnd(context.Context, *State) error                             { return nil }

// EventHooks translates the five points into the nine events.
type EventHooks struct {
	Bus      *event.Service
	Approver event.Approver

	// Risk answers how dangerous a call is, from the tool's own annotations.
	// A function rather than a table so the loop can ask the registry it was
	// built with rather than a global.
	Risk func(toolName string) event.RiskLevel

	// ApprovalDeadline bounds a request. Zero means the domain default.
	ApprovalDeadline time.Duration

	// Directory is where a hook runs, which the contract carries as cwd.
	Directory string

	// started is set on the first PrepareCall so SessionStart fires once.
	started bool
}

// PrepareCall fires SessionStart on the first call of a session and
// UserPromptSubmit on every one. UserPromptSubmit can block, and a block here
// ends the turn before a single token is spent.
func (h *EventHooks) PrepareCall(ctx context.Context, s *State) error {
	if h.Bus == nil {
		return nil
	}
	if !h.started {
		h.started = true
		out, err := h.Bus.Emit(ctx, h.base(event.SessionStart, s))
		if err != nil {
			return err
		}
		s.AppendContext(out.AdditionalContext)
	}

	e := h.base(event.UserPromptSubmit, s)
	e.Prompt = lastUserText(s.Messages)
	out, err := h.Bus.Emit(ctx, e)
	if err != nil {
		return err
	}
	if out.Blocked() {
		return event.Blocked(event.UserPromptSubmit, out)
	}
	s.AppendContext(out.AdditionalContext)
	return nil
}

// PrepareStep runs before every model call after the first. It is where
// PreCompact fires, which is why the compactor asks the hooks before it prunes
// rather than after: an extension that wants something to survive the prune has
// to be able to say so first.
func (h *EventHooks) PrepareStep(context.Context, *State) error { return nil }

// PreCompact fires before the history is pruned.
func (h *EventHooks) PreCompact(ctx context.Context, s *State) error {
	if h.Bus == nil {
		return nil
	}
	e := h.base(event.PreCompact, s)
	e.Trigger = "auto"
	out, err := h.Bus.Emit(ctx, e)
	if err != nil {
		return err
	}
	s.AppendContext(out.AdditionalContext)
	return nil
}

// ApproveTool is PreToolUse, and the place where this system diverges from the
// original most consequentially.
//
// The original turns `ask` into `deny` with a message that reads like a policy
// decision. The effect is perverse: an author who wants confirmation gets a
// system that silently never does the thing, so they learn to write `allow`.
// Here `ask` reaches a human (ADR-0007).
func (h *EventHooks) ApproveTool(ctx context.Context, c *ToolCall, s *State) (Decision, error) {
	if h.Bus == nil {
		return Decision{Allow: true}, nil
	}
	e := h.base(event.PreToolUse, s)
	e.Tool = c.Name
	e.Input = c.Input

	out, err := h.Bus.Emit(ctx, e)
	if err != nil {
		return Decision{}, err
	}

	switch out.PermissionDecision {
	case event.PermissionDeny:
		return Decision{Allow: false, Reason: out.Reason}, nil

	case event.PermissionAsk:
		if h.Approver == nil {
			// No channel at all is a different fact from a hook denying, and
			// the reason says which. The original's message is
			// indistinguishable from a policy denial.
			return Decision{Allow: false, Reason: "no approval channel is available in this run mode"}, nil
		}
		res, err := h.Approver.RequestApproval(ctx, event.ApprovalRequest{
			SessionID: s.SessionID, Workspace: s.Workspace, AgentID: s.AgentID,
			ToolName: c.Name, Input: c.Input, Reason: out.Reason,
			Risk: h.riskOf(c.Name), Deadline: h.ApprovalDeadline,
		})
		if err != nil {
			return Decision{Allow: false, Reason: "the approval channel failed: " + err.Error()}, nil
		}
		return Decision{
			Allow: res.Approved, Reason: res.Reason, UpdatedInput: res.UpdatedInput,
		}, nil

	default:
		if out.Decision == event.DecisionBlock {
			return Decision{Allow: false, Reason: out.Reason}, nil
		}
		return Decision{
			Allow: true, UpdatedInput: out.UpdatedInput, AddContext: out.AdditionalContext,
		}, nil
	}
}

// AfterTool fires PostToolUse, or PostToolUseFailure when the call failed.
func (h *EventHooks) AfterTool(ctx context.Context, c *ToolCall, r *ToolResult, s *State) error {
	if h.Bus == nil {
		return nil
	}
	kind := event.PostToolUse
	if r.Error != "" && !r.Denied {
		kind = event.PostToolUseFailure
	}
	e := h.base(kind, s)
	e.Tool = c.Name
	e.Input = c.Input
	e.Output = r.Output
	e.Reason = r.Error

	out, err := h.Bus.Emit(ctx, e)
	if err != nil {
		return err
	}
	if out.Blocked() {
		return event.Blocked(kind, out)
	}
	s.AppendContext(out.AdditionalContext)
	return nil
}

// OnEnd fires Stop. A blocking Stop hook is how "you are not finished" is said
// to an agent that thinks it is.
func (h *EventHooks) OnEnd(ctx context.Context, s *State) error {
	if h.Bus == nil {
		return nil
	}
	out, err := h.Bus.Emit(ctx, h.base(event.Stop, s))
	if err != nil {
		return err
	}
	if out.Blocked() {
		return event.Blocked(event.Stop, out)
	}
	return nil
}

func (h *EventHooks) base(t event.Type, s *State) event.Event {
	return event.Event{
		Type:      t,
		SessionID: s.SessionID,
		Agent:     s.AgentID,
		Workspace: s.Workspace,
		Directory: h.Directory,
	}
}

func (h *EventHooks) riskOf(tool string) event.RiskLevel {
	if h.Risk == nil {
		return event.RiskMedium
	}
	return h.Risk(tool)
}

func lastUserText(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			return messages[i].Text
		}
	}
	return ""
}

// RiskFromRegistry answers the risk question from what each tool declares about
// itself, so the answer changes when a tool's annotations change and nobody has
// to maintain a second list.
func RiskFromRegistry(r *toolexec.Registry) func(string) event.RiskLevel {
	return func(name string) event.RiskLevel {
		for _, s := range r.Specs() {
			if s.Name == name {
				return event.Risk(
					s.Annotations.ReadOnlyHint,
					s.Annotations.DestructiveHint,
					s.Annotations.OpenWorldHint,
				)
			}
		}
		return event.RiskMedium
	}
}
