package skillhooks_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/skillhooks"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/skill"
)

// discardLog is event.Log with nowhere to write — this test cares about
// Emit's own return value, not the audit trail Emit also produces.
type discardLog struct{}

func (discardLog) Append(context.Context, event.Record) error { return nil }

func newRealBus() *event.Service {
	return event.NewService(event.Deps{
		Log:   discardLog{},
		Clock: clockx.Fixed{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		IDs:   &ids.Sequence{Prefix: "e"},
	})
}

// TestARegisteredHookActuallyInterceptsAnEmit is the proof this whole slice
// of work exists for: a skill's own hook, once Register has run, changes
// what a real event.Service.Emit returns — not merely what Handlers(t)
// lists. It is the same distinction RunAdapterContract's own "did the
// connection survive" subtest draws for a different port: a listing can be
// right while dispatch is still broken.
func TestARegisteredHookActuallyInterceptsAnEmit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test spawns a POSIX shell script")
	}
	root := t.TempDir()
	dir := skillDir(t, root, "crm")
	script := filepath.Join(dir, "hooks", "guard.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\necho '{\"decision\":\"block\",\"reason\":\"blocked by guard\"}'\n"
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	bus := newRealBus()
	h := skillhooks.New(bus, root)
	if err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "hooks/guard.sh"},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := bus.Emit(context.Background(), event.Event{Type: event.PreToolUse, Tool: "toolsets_call"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Decision != event.DecisionBlock {
		t.Fatalf("Decision = %q, want %q", out.Decision, event.DecisionBlock)
	}
	if out.HookID != "crm/guard" {
		t.Fatalf("HookID = %q, want the namespaced id crm/guard", out.HookID)
	}
	if out.Reason != "blocked by guard" {
		t.Fatalf("Reason = %q", out.Reason)
	}

	// And Deregister actually takes it back out of dispatch — Handlers(t)
	// already proves the listing empties in the unit tests; this proves
	// Emit itself stops seeing it.
	if err := h.Deregister(context.Background(), "crm"); err != nil {
		t.Fatal(err)
	}
	out2, err := bus.Emit(context.Background(), event.Event{Type: event.PreToolUse, Tool: "toolsets_call"})
	if err != nil {
		t.Fatal(err)
	}
	if out2.Decision == event.DecisionBlock {
		t.Fatal("the deregistered hook still blocked the event")
	}
}
