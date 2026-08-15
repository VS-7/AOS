package eventlog_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/eventlog"
	"github.com/OWNER/aos/internal/domain/event"
)

var day = time.Date(2026, 3, 1, 9, 30, 0, 0, time.UTC)

func record(id string, at time.Time) event.Record {
	return event.Record{
		ID: id, Agent: "atlas", SessionID: "s-1",
		Type: event.PreToolUse, Hook: "guard",
		Outcome:   event.Outcome{PermissionDecision: event.PermissionAllow, HookID: "guard"},
		CreatedAt: at,
	}
}

// TestTheLogAppendsAndReadsBack is the shape of the thing: lines in, lines out,
// in the order they were written.
func TestTheLogAppendsAndReadsBack(t *testing.T) {
	root := t.TempDir()
	w := eventlog.New(root)
	ctx := context.Background()

	for i, id := range []string{"a", "b", "c"} {
		if err := w.Append(ctx, record(id, day.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatal(err)
		}
	}

	got, err := w.Read(ctx, "atlas", day)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != "a" || got[2].ID != "c" {
		t.Fatalf("read back %d records in the wrong order: %+v", len(got), got)
	}

	path := filepath.Join(root, ".aos", "agents", "atlas", "events", "2026-03-01.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the log is not where the specification says it is: %v", err)
	}
}

// TestASecondWriteAppendsRatherThanReplaces. The property the whole package
// exists for, checked against the file rather than against the API: opening
// without O_APPEND would pass every other test in here.
func TestASecondWriteAppendsRatherThanReplaces(t *testing.T) {
	root := t.TempDir()
	w := eventlog.New(root)
	ctx := context.Background()

	if err := w.Append(ctx, record("first", day)); err != nil {
		t.Fatal(err)
	}
	// A second writer over the same directory is the two-process case: a CLI
	// running a hook while the daemon is up.
	if err := eventlog.New(root).Append(ctx, record("second", day.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".aos", "agents", "atlas", "events", "2026-03-01.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"first"`) || !strings.Contains(string(raw), `"second"`) {
		t.Fatalf("the second write replaced the first:\n%s", raw)
	}
	if lines := strings.Count(strings.TrimSpace(string(raw)), "\n") + 1; lines != 2 {
		t.Fatalf("%d lines, want 2", lines)
	}
}

// TestOneCorruptLineDoesNotHideTheRest. A machine that lost power mid-write
// must not cost the whole day's audit trail.
func TestOneCorruptLineDoesNotHideTheRest(t *testing.T) {
	root := t.TempDir()
	w := eventlog.New(root)
	ctx := context.Background()

	if err := w.Append(ctx, record("good", day)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".aos", "agents", "atlas", "events", "2026-03-01.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"id\":\"trunc\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := w.Append(ctx, record("after", day.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}

	got, err := w.Read(ctx, "atlas", day)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d records, want the two that parse: %+v", len(got), got)
	}
}

// TestRetentionRemovesWholeFiles. Never lines: a job that edits a log is a job
// that can edit the wrong line.
func TestRetentionRemovesWholeFiles(t *testing.T) {
	root := t.TempDir()
	w := eventlog.New(root)
	ctx := context.Background()

	now := day.AddDate(0, 0, 40)
	for _, offset := range []int{0, -3, -35} {
		if err := w.Append(ctx, record("r", now.AddDate(0, 0, offset))); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := w.Prune(ctx, "atlas", 30*24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d files, want the one outside the window", removed)
	}
	if got, _ := w.Read(ctx, "atlas", now); len(got) != 1 {
		t.Error("today's file went with it")
	}
	if got, _ := w.Read(ctx, "atlas", now.AddDate(0, 0, -3)); len(got) != 1 {
		t.Error("a file inside the window was removed")
	}
}

// TestAnAgentIdIsNotAPath. The record arrives from a payload, and the payload
// comes from a hook.
func TestAnAgentIdIsNotAPath(t *testing.T) {
	root := t.TempDir()
	w := eventlog.New(root)
	r := record("x", day)
	r.Agent = "../../escape"

	if err := w.Append(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".aos", "agents", "escape", "events")); err != nil {
		t.Fatalf("the id was not sanitized into the agents directory: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "escape" {
			t.Fatal("the log escaped the workspace")
		}
	}
}

// TestARecordWithoutATimeIsRefused: it has no file to go in, and inventing one
// would file it under a day it did not happen.
func TestARecordWithoutATimeIsRefused(t *testing.T) {
	w := eventlog.New(t.TempDir())
	r := record("x", day)
	r.CreatedAt = time.Time{}
	if err := w.Append(context.Background(), r); err == nil {
		t.Fatal("a record with no timestamp was filed")
	}
}

// TestReadingADayThatWasNeverWrittenIsEmpty, not an error.
func TestReadingADayThatWasNeverWrittenIsEmpty(t *testing.T) {
	w := eventlog.New(t.TempDir())
	got, err := w.Read(context.Background(), "atlas", day)
	if err != nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
	if n, err := w.Prune(context.Background(), "atlas", time.Hour, day); n != 0 || err != nil {
		t.Fatalf("prune of nothing = %d, %v", n, err)
	}
}
