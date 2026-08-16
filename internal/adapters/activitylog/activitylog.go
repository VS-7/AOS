// Package activitylog is the JSONL store behind the workspace activity log.
//
// The original keeps one .fractal/activity.json array and rewrites it whole on
// every notification. That is an O(n) write on a file that only grows, with no
// atomicity, and it works for a week. Here each month is its own append-only
// file under .aos/activity/{yyyy-mm}.jsonl: appends are O(1), retention removes
// a file, and the only rewrite is the one the domain asks for explicitly.
package activitylog

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

	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/domain/activity"
)

// Writer stores activities under {root}/.aos/activity/.
type Writer struct {
	dir string
	mu  sync.Mutex
}

// New builds a writer rooted at a workspace directory.
func New(root string) *Writer {
	return &Writer{dir: filepath.Join(root, ".aos", "activity")}
}

// Append writes one entry as a line of JSON in its month's partition.
func (w *Writer) Append(_ context.Context, a activity.Activity) error {
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("activitylog: an entry without a timestamp cannot be filed")
	}
	line, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(w.pathOf(a.Month()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // the workspace log is readable by the person whose workspace it is
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// Load reads every partition that can hold an entry at or after since, oldest
// first.
//
// The partition name is the filter: a month whose last day precedes the cutoff
// cannot contain a match and is never opened. That is what keeps "what happened
// this week" from parsing a year of history.
func (w *Writer) Load(_ context.Context, since time.Time) ([]activity.Activity, error) {
	months, err := w.months()
	if err != nil {
		return nil, err
	}

	var out []activity.Activity
	for _, month := range months {
		if !since.IsZero() && endOf(month).Before(since) {
			continue
		}
		entries, err := w.read(month)
		if err != nil {
			return nil, err
		}
		for _, a := range entries {
			if since.IsZero() || !a.CreatedAt.Before(since) {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

// Months lists the partitions present, oldest first.
func (w *Writer) Months(_ context.Context) ([]string, error) { return w.months() }

// Rewrite replaces one partition with exactly these entries, atomically. No
// entries removes the file.
func (w *Writer) Rewrite(_ context.Context, month string, entries []activity.Activity) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	path := w.pathOf(month)
	if len(entries) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	var buf strings.Builder
	for _, a := range entries {
		line, err := json.Marshal(a)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	return atomicfs.WriteFile(path, []byte(buf.String()), 0o644)
}

func (w *Writer) pathOf(month string) string { return filepath.Join(w.dir, month+".jsonl") }

func (w *Writer) months() ([]string, error) {
	entries, err := os.ReadDir(w.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		month := strings.TrimSuffix(name, ".jsonl")
		if _, err := time.Parse("2006-01", month); err != nil {
			continue
		}
		out = append(out, month)
	}
	sort.Strings(out) // yyyy-mm sorts chronologically as text
	return out, nil
}

// read parses one partition. A line that does not parse is skipped rather than
// fatal: one torn record from a machine that lost power must not make the
// month unreadable.
func (w *Writer) read(month string) ([]activity.Activity, error) {
	raw, err := os.ReadFile(w.pathOf(month)) //nolint:gosec // the path is a formatted month under the workspace log directory
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []activity.Activity
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var a activity.Activity
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// endOf returns the last instant a partition can hold.
func endOf(month string) time.Time {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		// An unparseable name was already filtered out by months(); treating it
		// as unbounded here means a partition is read rather than skipped, which
		// is the safe direction for a log.
		return time.Time{}
	}
	return start.AddDate(0, 1, 0).Add(-time.Nanosecond)
}

// ReadStore keeps the per-actor read overlay in one small JSON file.
//
// It is deliberately not in the log: marking a notification read must not
// rewrite the record of what happened.
type ReadStore struct {
	path string
	mu   sync.Mutex
}

// NewReadStore builds the overlay store for a workspace.
func NewReadStore(root string) *ReadStore {
	return &ReadStore{path: filepath.Join(root, ".aos", "activity", "read.json")}
}

// Load reads the overlay. A missing file is an empty overlay, not an error.
func (s *ReadStore) Load(context.Context) (activity.ReadState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path) //nolint:gosec // a fixed path under the workspace log directory
	if os.IsNotExist(err) {
		return activity.ReadState{}, nil
	}
	if err != nil {
		return activity.ReadState{}, err
	}
	var state activity.ReadState
	if err := json.Unmarshal(raw, &state); err != nil {
		return activity.ReadState{}, err
	}
	return state, nil
}

// Save writes the overlay atomically.
func (s *ReadStore) Save(_ context.Context, state activity.ReadState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return atomicfs.WriteFile(s.path, raw, 0o644)
}
