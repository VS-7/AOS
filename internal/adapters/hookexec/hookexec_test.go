package hookexec_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/hookexec"
	"github.com/OWNER/aos/internal/domain/event"
)

// The hook under test is this test binary re-executed. It is the portable way
// to run a real program: a shell script would not run on Windows, and building
// a helper binary costs a compile per test.
const helperEnv = "AOS_HOOKEXEC_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(helperEnv); mode != "" {
		os.Exit(helper(mode))
	}
	os.Exit(m.Run())
}

// helper is the hook. It reads the event on stdin exactly as a Claude Code hook
// does, and answers in that contract.
func helper(mode string) int {
	var in map[string]any
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fmt.Fprintln(os.Stderr, "could not read the event:", err)
		return 1
	}
	switch mode {
	case "quiet":
		return 0

	case "echo-event":
		fmt.Printf(`{"continue":true,"hookSpecificOutput":{"hookEventName":%q,"additionalContext":%q}}`,
			in["hook_event_name"], fmt.Sprint(in["tool_name"], "|", in["cwd"], "|", os.Getenv("AOS_AGENT_ID")))
		return 0

	case "deny":
		fmt.Print(`{"continue":true,"hookSpecificOutput":{"hookEventName":"PreToolUse",` +
			`"permissionDecision":"deny","permissionDecisionReason":"pushing to main is not allowed"}}`)
		return 0

	case "ask":
		fmt.Print(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask",` +
			`"permissionDecisionReason":"this one needs a human"}}`)
		return 0

	case "rewrite":
		fmt.Print(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow",` +
			`"updatedInput":{"command":"git push"}}}`)
		return 0

	case "chatty":
		fmt.Println("checking the branch...")
		fmt.Println("branch is main")
		fmt.Print(`{"decision":"block","reason":"main is protected"}`)
		return 0

	case "exit-two":
		fmt.Fprintln(os.Stderr, "refusing: the working tree is dirty")
		return hookexec.BlockExitCode

	case "broken":
		fmt.Fprintln(os.Stderr, "cannot find python")
		return 127

	case "garbage":
		fmt.Print(`{"decision":`)
		return 0

	case "stop":
		fmt.Print(`{"continue":false,"stopReason":"the budget for this session is spent"}`)
		return 0

	case "no-secrets":
		for _, k := range []string{"AOS_TOKEN", "OPENAI_API_KEY", "AOS_SECRET"} {
			if os.Getenv(k) != "" {
				fmt.Printf(`{"decision":"block","reason":"the hook can read %s"}`, k)
				return 0
			}
		}
		return 0

	case "slow":
		time.Sleep(5 * time.Second)
		return 0
	}
	return 3
}

func handler(t *testing.T, mode string, events ...event.Type) *hookexec.Handler {
	t.Helper()
	if len(events) == 0 {
		events = []event.Type{event.PreToolUse}
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return hookexec.New(hookexec.Spec{
		ID:      "hook-" + mode,
		Events:  events,
		Command: exe,
		Env:     []string{helperEnv + "=" + mode},
	}, t.TempDir())
}

// TestAHookWrittenForClaudeCodeRunsUnchanged is the point of ADR-0016: the
// input it receives carries the field names of that contract, and the JSON it
// answers with is read without translation.
func TestAHookWrittenForClaudeCodeRunsUnchanged(t *testing.T) {
	h := handler(t, "echo-event")
	out, err := h.Handle(context.Background(), event.Event{
		Type: event.PreToolUse, Tool: "Bash", Agent: "atlas",
		Directory: t.TempDir(),
		Input:     json.RawMessage(`{"command":"ls"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.AdditionalContext, "Bash|") {
		t.Fatalf("the hook did not receive the Claude Code field names: %q", out.AdditionalContext)
	}
	if !strings.HasSuffix(out.AdditionalContext, "|atlas") {
		t.Errorf("the agent id did not reach the hook's environment: %q", out.AdditionalContext)
	}
	if out.HookID != "hook-echo-event" {
		t.Errorf("HookID = %q", out.HookID)
	}
}

// TestTheThreePermissionDecisionsArriveIntact — allow, deny and ask. The third
// is the one the original throws away.
func TestTheThreePermissionDecisionsArriveIntact(t *testing.T) {
	cases := []struct {
		mode, want, reason string
	}{
		{"deny", event.PermissionDeny, "pushing to main"},
		{"ask", event.PermissionAsk, "needs a human"},
		{"rewrite", event.PermissionAllow, ""},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			out, err := handler(t, c.mode).Handle(context.Background(),
				event.Event{Type: event.PreToolUse, Tool: "Bash"})
			if err != nil {
				t.Fatal(err)
			}
			if out.PermissionDecision != c.want {
				t.Fatalf("PermissionDecision = %q, want %q", out.PermissionDecision, c.want)
			}
			if c.reason != "" && !strings.Contains(out.Reason, c.reason) {
				t.Errorf("Reason = %q", out.Reason)
			}
		})
	}
}

// TestAHookRewritesThePayload — the third superpower, and the one that makes
// whoever controls the hooks control the agent.
func TestAHookRewritesThePayload(t *testing.T) {
	out, err := handler(t, "rewrite").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Bash", Input: json.RawMessage(`{"command":"git push --force"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if string(out.UpdatedInput) != `{"command":"git push"}` {
		t.Fatalf("UpdatedInput = %s", out.UpdatedInput)
	}
}

// TestExitTwoBlocksWithStderrAsTheReason. Most hooks people write are shell
// scripts, and `exit 2` is easier to get right than a JSON document.
func TestExitTwoBlocksWithStderrAsTheReason(t *testing.T) {
	out, err := handler(t, "exit-two").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Bash"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Blocked() {
		t.Fatalf("exit 2 did not block: %+v", out)
	}
	if !strings.Contains(out.Reason, "working tree is dirty") {
		t.Errorf("Reason = %q — stderr is the reason", out.Reason)
	}
}

// TestSilenceIsConsent. The common case is a hook that looks and says nothing.
func TestSilenceIsConsent(t *testing.T) {
	out, err := handler(t, "quiet").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Read"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Blocked() || out.PermissionDecision != "" {
		t.Fatalf("a quiet hook decided something: %+v", out)
	}
}

// TestAChattyHookIsStillUnderstood. People log to stdout; the decision is the
// last line that parses.
func TestAChattyHookIsStillUnderstood(t *testing.T) {
	out, err := handler(t, "chatty").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Bash"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Blocked() || !strings.Contains(out.Reason, "main is protected") {
		t.Fatalf("outcome = %+v", out)
	}
}

// TestContinueFalseIsTheOtherWayToStop.
func TestContinueFalseIsTheOtherWayToStop(t *testing.T) {
	out, err := handler(t, "stop", event.Stop).Handle(context.Background(), event.Event{Type: event.Stop})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Blocked() || !strings.Contains(out.Reason, "budget") {
		t.Fatalf("outcome = %+v", out)
	}
}

// TestAHookThatCannotRunIsAnErrorAndNotADenial. The difference matters: a
// missing interpreter is an installation problem, not a policy decision, and
// reporting it as a denial sends the reader looking for a rule that does not
// exist.
func TestAHookThatCannotRunIsAnErrorAndNotADenial(t *testing.T) {
	_, err := handler(t, "broken").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Bash"})
	if err == nil {
		t.Fatal("a hook that exited 127 looked like a decision")
	}
	if !strings.Contains(err.Error(), "cannot find python") {
		t.Errorf("err = %v — stderr should be in it", err)
	}
}

// TestOutputThatIsNotADecisionIsAnError, rather than a silent allow.
func TestOutputThatIsNotADecisionIsAnError(t *testing.T) {
	_, err := handler(t, "garbage").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Bash"})
	if err == nil {
		t.Fatal("malformed output was read as consent")
	}
}

// TestTheHookCannotReadTheDaemonSecrets. A hook is third-party code; it decides
// whether a tool may run, and does not need the token that talks to the API.
func TestTheHookCannotReadTheDaemonSecrets(t *testing.T) {
	t.Setenv("AOS_TOKEN", "secret-value")
	t.Setenv("OPENAI_API_KEY", "sk-secret")
	t.Setenv("AOS_SECRET", "another")

	out, err := handler(t, "no-secrets").Handle(context.Background(),
		event.Event{Type: event.PreToolUse, Tool: "Bash"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Blocked() {
		t.Fatalf("the hook inherited a secret: %s", out.Reason)
	}
}

// TestAHookIsBoundedByItsOwnTimeout.
func TestAHookIsBoundedByItsOwnTimeout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h := hookexec.New(hookexec.Spec{
		ID: "slow", Events: []event.Type{event.Stop},
		Command: exe, Env: []string{helperEnv + "=slow"},
		Timeout: 50 * time.Millisecond,
	}, t.TempDir())

	start := time.Now()
	if _, err := h.Handle(context.Background(), event.Event{Type: event.Stop}); err == nil {
		t.Fatal("the slow hook was allowed to finish")
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("the timeout did not bite: %s", elapsed)
	}
}

// TestDecodeReadsAnEmptyDocumentAsNoOpinion, at the unit level, because the
// process path cannot cover the empty-and-whitespace cases cheaply.
func TestDecodeReadsAnEmptyDocumentAsNoOpinion(t *testing.T) {
	for _, raw := range []string{"", "   \n", "not json at all\n"} {
		out, err := hookexec.Decode([]byte(raw))
		if err != nil {
			t.Fatalf("Decode(%q) = %v", raw, err)
		}
		if out.Blocked() || out.AdditionalContext != "" {
			t.Fatalf("Decode(%q) invented a decision: %+v", raw, out)
		}
	}
}

// TestSpecReportsWhatItWants.
func TestSpecReportsWhatItWants(t *testing.T) {
	h := handler(t, "quiet", event.PreToolUse, event.Stop)
	if got := h.Handles(); len(got) != 2 {
		t.Fatalf("Handles = %v", got)
	}
	if h.ID() != "hook-quiet" {
		t.Fatalf("ID = %q", h.ID())
	}
}
