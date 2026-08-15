// Package eventlog is the append-only audit trail of the hook system.
//
// Append-only by construction, not by discipline: the file is opened with
// O_APPEND, there is no Update and no Delete in the port, and retention removes
// whole day files rather than editing one. A log a program can rewrite line by
// line is a log somebody can rewrite line by line.
package eventlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/domain/event"
)

// Writer appends records under {root}/.aos/agents/{agent}/events/{date}.jsonl.
type Writer struct {
	dir func(agent string) string

	mu sync.Mutex
}

// New builds a writer rooted at a workspace directory.
func New(root string) *Writer {
	return &Writer{dir: func(agent string) string {
		return filepath.Join(root, ".aos", "agents", event.SanitizeAgent(agent), "events")
	}}
}

// Append writes one record as a line of JSON.
//
// The mutex serialises writers inside this process. Across processes the
// O_APPEND flag is what keeps two daemons from interleaving half a line, which
// holds for writes below the pipe buffer — and a record that does not fit in
// one is a record with a payload that should have been spilled.
func (w *Writer) Append(_ context.Context, r event.Record) error {
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("eventlog: a record without a timestamp cannot be filed")
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}

	dir := w.dir(r.Agent)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, event.LogDay(r.CreatedAt)+".jsonl")

	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // an audit log is readable by the person being audited
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// Read returns the records of one agent for one day, in the order they were
// written. It exists for the audit surface and for the tests that assert on
// what was recorded.
//
// A line that does not parse is skipped rather than fatal: one corrupt record —
// a machine that lost power mid-write — must not make the rest unreadable.
func (w *Writer) Read(_ context.Context, agent string, day time.Time) ([]event.Record, error) {
	path := filepath.Join(w.dir(agent), event.LogDay(day)+".jsonl")
	raw, err := os.ReadFile(path) //nolint:gosec // the path is derived from a sanitized agent id and a formatted date
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []event.Record
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r event.Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// Prune removes day files older than the retention window and reports how many
// went. Whole files: a retention job that rewrites a file to drop lines from it
// is a retention job that can drop the wrong lines.
func (w *Writer) Prune(_ context.Context, agent string, retention time.Duration, now time.Time) (int, error) {
	dir := w.dir(agent)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-retention)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var removed int
	for _, name := range names {
		day, err := time.Parse("2006-01-02", strings.TrimSuffix(name, ".jsonl"))
		if err != nil {
			continue
		}
		// A day file is expired once the day after it has also passed the
		// cutoff: the file named for a day holds records up to its last second.
		if day.AddDate(0, 0, 1).After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
