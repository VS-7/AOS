// Package hookexec runs a hook that lives outside this process.
//
// The wire format is Claude Code's, and that is the whole point of the package:
// a hook somebody already wrote for that tool runs here unchanged (ADR-0016).
// The translation is thin because the event shape was adopted rather than
// invented — see internal/domain/event, whose JSON tags are that contract.
package hookexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/domain/event"
)

// BlockExitCode is the convention: a hook that exits 2 has blocked, and what it
// wrote to stderr is the reason. It exists because the majority of hooks people
// write are shell scripts, and `exit 2` is easier to get right than JSON.
const BlockExitCode = 2

// Spec declares one external hook.
type Spec struct {
	// ID names the hook in the audit log and in the error a block produces.
	ID string

	// Events are the points this hook wants. Empty means it is never called,
	// which is a configuration mistake rather than a wildcard.
	Events []event.Type

	// Command and Args are the program to run. The command is not passed
	// through a shell: a hook that wants a pipeline names its shell itself,
	// and the difference is one less place where quoting decides behaviour.
	Command string
	Args    []string

	// Dir is where the hook runs. Empty means the workspace root.
	Dir string

	// Env is added to a minimal environment. The parent's environment is not
	// inherited: a hook is third-party code and does not need the daemon's
	// token to decide whether a tool may run.
	Env []string

	// Timeout bounds this hook specifically. Zero defers to the bus.
	Timeout time.Duration
}

// Handler is one external hook, adapted to the bus.
type Handler struct {
	spec Spec
	root string
}

// New builds a handler for a spec. root is the workspace directory a hook with
// no Dir runs in.
func New(spec Spec, root string) *Handler { return &Handler{spec: spec, root: root} }

// ID names the hook.
func (h *Handler) ID() string { return h.spec.ID }

// Handles reports the events it wants.
func (h *Handler) Handles() []event.Type { return h.spec.Events }

// Handle runs the program, feeding it the event and reading back its decision.
func (h *Handler) Handle(ctx context.Context, e event.Event) (event.Outcome, error) {
	payload, err := json.Marshal(Input{
		Event:     e,
		Directory: h.dir(e),
	})
	if err != nil {
		return event.Outcome{}, err
	}

	if h.spec.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.spec.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, h.spec.Command, h.spec.Args...) //nolint:gosec // the command is declared in workspace configuration, not derived from a payload
	cmd.Dir = h.dir(e)
	cmd.Env = h.env(e)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	var exit *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exit) && exit.ExitCode() == BlockExitCode:
		// The shell-script path: exit 2 blocks, stderr is the reason.
		return event.Outcome{
			Decision:           event.DecisionBlock,
			PermissionDecision: permissionFor(e.Type, event.PermissionDeny),
			Reason:             reasonFrom(&stderr),
			HookID:             h.spec.ID,
		}, nil
	default:
		return event.Outcome{}, fmt.Errorf("hook %s: %w: %s", h.spec.ID, runErr, reasonFrom(&stderr))
	}

	out, err := Decode(stdout.Bytes())
	if err != nil {
		return event.Outcome{}, fmt.Errorf("hook %s wrote something that is not a decision: %w", h.spec.ID, err)
	}
	out.HookID = h.spec.ID
	return out, nil
}

func (h *Handler) dir(e event.Event) string {
	if h.spec.Dir != "" {
		return h.spec.Dir
	}
	if e.Directory != "" {
		return e.Directory
	}
	return h.root
}

// env builds the child's environment from a short allowlist plus whatever the
// spec adds. PATH and HOME are there because a hook that cannot find its own
// interpreter is a hook that never runs; nothing else is inherited.
func (h *Handler) env(e event.Event) []string {
	out := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"AOS_HOOK_EVENT=" + string(e.Type),
	}
	if e.Agent != "" {
		out = append(out, "AOS_AGENT_ID="+e.Agent)
	}
	if e.SessionID != "" {
		out = append(out, "AOS_SESSION_ID="+e.SessionID)
	}
	return append(out, h.spec.Env...)
}

func reasonFrom(b *bytes.Buffer) string {
	return strings.TrimSpace(b.String())
}

// Input is what the hook reads on stdin. The event's own JSON tags are the
// Claude Code field names, so the struct embeds it rather than copying it
// field by field — a copy is where the two contracts would drift apart.
type Input struct {
	event.Event
	Directory string `json:"cwd,omitempty"`
}

// Output is what a hook may write on stdout.
type Output struct {
	Continue   bool   `json:"continue"`
	StopReason string `json:"stopReason,omitempty"`
	Decision   string `json:"decision,omitempty"`
	Reason     string `json:"reason,omitempty"`

	Specific *SpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// SpecificOutput is the per-event half of the contract, where PreToolUse says
// allow, deny or ask, and where a hook rewrites a payload.
type SpecificOutput struct {
	EventName                string          `json:"hookEventName,omitempty"`
	PermissionDecision       string          `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string          `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string          `json:"additionalContext,omitempty"`
	UpdatedInput             json.RawMessage `json:"updatedInput,omitempty"`
}

// Decode translates what a hook wrote into an outcome.
//
// Silence is consent: a hook that prints nothing has declined to decide, which
// is by far the most common case and must not be an error.
func Decode(raw []byte) (event.Outcome, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return event.Outcome{}, nil
	}
	// A hook that logs to stdout before its JSON is a hook people write. The
	// decision is the last line that parses as an object; anything before it
	// is treated as logging rather than as a malformed decision.
	if trimmed[0] != '{' {
		if last := lastJSONLine(trimmed); last != nil {
			trimmed = last
		} else {
			return event.Outcome{}, nil
		}
	}

	var out Output
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return event.Outcome{}, err
	}

	res := event.Outcome{
		Decision: out.Decision,
		Reason:   out.Reason,
	}
	// `continue: false` is the contract's other way of saying stop.
	if !out.Continue && bytes.Contains(trimmed, []byte(`"continue"`)) {
		res.Decision = event.DecisionBlock
		if res.Reason == "" {
			res.Reason = out.StopReason
		}
	}
	if s := out.Specific; s != nil {
		res.PermissionDecision = s.PermissionDecision
		res.AdditionalContext = s.AdditionalContext
		res.UpdatedInput = s.UpdatedInput
		if res.Reason == "" {
			res.Reason = s.PermissionDecisionReason
		}
	}
	return res, nil
}

func lastJSONLine(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) > 0 && line[0] == '{' && json.Valid(line) {
			return line
		}
	}
	return nil
}

// permissionFor keeps a block on a tool event expressible in the field the
// runtime reads. On the other events the decision alone carries it.
func permissionFor(t event.Type, decision string) string {
	if t == event.PreToolUse {
		return decision
	}
	return ""
}
