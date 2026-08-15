package toolexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Spiller writes what does not fit in the context window.
type Spiller struct {
	dir string
	log *slog.Logger
}

// NewSpiller writes under the given directory, which is the same one the
// sandbox exposes read-only.
func NewSpiller(dir string, log *slog.Logger) *Spiller {
	if log == nil {
		log = slog.Default()
	}
	return &Spiller{dir: dir, log: log}
}

// Dir is where spilled outputs live.
func (s *Spiller) Dir() string { return s.dir }

// Process turns a tool's return value into what the model sees.
func (s *Spiller) Process(_ context.Context, callID string, v any) Output {
	if passed, ok := passthrough(v); ok {
		return Output{Data: passed}
	}

	text, structured := serialize(v)
	if len(text) <= MaxToolOutputChars {
		return Output{Data: structured}
	}

	visible := truncateRunes(text, MaxToolOutputChars)

	path, err := s.persist(callID, text)
	if err != nil {
		// Best effort, and deliberately so: on a read-only filesystem the
		// model still gets something bounded rather than an error about a
		// directory it cannot do anything about.
		s.log.Warn("could not spill a tool output; the model got the truncated prefix",
			"call", callID, "err", err)
		return Output{
			Data: visible,
			Truncated: &Truncated{
				Original: len(text), Visible: len(visible),
				Instruction: "The output was truncated to fit the context and could not be saved to disk. Narrow the call and try again.",
			},
		}
	}

	return Output{
		Data:   visible,
		Output: path,
		Truncated: &Truncated{
			Original:    len(text),
			Visible:     len(visible),
			Output:      path,
			Instruction: Instruction(path),
		},
	}
}

// Instruction is what the model is told when an output was cut. It names the
// tool and the parameters that read a slice, because "the output was truncated"
// on its own leaves the model with no move.
func Instruction(path string) string {
	return fmt.Sprintf("The full output of the previous tool was truncated to fit the agent context. "+
		"The complete output was saved to %s. Use the Read tool with offset and limit parameters to inspect "+
		"the relevant slice (e.g. Read({ file_path: %q, offset: 1, limit: 200 })), or move the file to a more "+
		"convenient location with Bash if you need to keep it for later.", path, path)
}

// persist writes the whole output under a name derived from the call id.
func (s *Spiller) persist(callID, content string) (string, error) {
	if s.dir == "" {
		return "", fmt.Errorf("toolexec: no spillover directory is configured")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.dir, sanitizeCallID(callID)+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // the agent reads this back through the sandbox
		return "", err
	}
	return path, nil
}

// sanitizeCallID keeps a malformed identifier from becoming a path. The id
// comes from the model's tool call, which means it comes from outside.
func sanitizeCallID(id string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, id)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "output"
	}
	if len(clean) > 128 {
		clean = clean[:128]
	}
	return clean
}

// Rotate removes spilled outputs older than the window.
//
// It runs at boot and on a timer, which is a divergence: the original rotates
// when the executor is constructed, and a daemon that runs for three weeks
// constructs it once.
func Rotate(_ context.Context, dir string, ttl time.Duration, now time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-ttl)
	var removed int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// serialize renders a value as text and returns the form the model should get
// when it fits: a string stays a string, everything else keeps its structure.
func serialize(v any) (string, any) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, t
	case []byte:
		return string(t), string(t)
	case error:
		return t.Error(), t.Error()
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		// A structure that will not serialize — a cycle, a channel, a function
		// — is described rather than dropped, so the model learns something
		// went wrong rather than seeing an empty result.
		text := fmt.Sprintf("<a value of type %T that could not be serialized: %v>", v, err)
		return text, text
	}
	text := strings.TrimRight(buf.String(), "\n")
	if len(text) > SafeJSONLimit {
		text = text[:SafeJSONLimit]
	}
	return text, v
}

// truncateRunes cuts on a rune boundary.
//
// Go strings are UTF-8, so the original's concern with UTF-16 surrogate pairs
// becomes a concern with rune boundaries: the same intent, simpler mechanics,
// and the same test — an output of emoji cut at the limit must still be valid.
func truncateRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}
