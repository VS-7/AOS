package toolexec_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/runtime/toolexec"
	"github.com/OWNER/aos/internal/testx"
)

func tool(name string, fn func(context.Context, json.RawMessage) (any, error)) toolexec.Tool {
	return toolexec.Func{Definition: toolexec.Spec{Name: name}, Fn: fn}
}

func returns(name string, v any) toolexec.Tool {
	return tool(name, func(context.Context, json.RawMessage) (any, error) { return v, nil })
}

func wrapped(t *testing.T, inner toolexec.Tool) (toolexec.Tool, string) {
	t.Helper()
	dir := t.TempDir()
	return toolexec.Wrap(inner, toolexec.WithSpill(toolexec.NewSpiller(dir, discardLog()))), dir
}

// TestASmallOutputKeepsItsStructure. A divergence from the original, which
// serializes in order to truncate and pays that cost on every call: a list of
// five tasks should reach the model as an array, not as JSON inside a string.
func TestASmallOutputKeepsItsStructure(t *testing.T) {
	w, _ := wrapped(t, returns("list", []map[string]any{{"id": "t-1"}, {"id": "t-2"}}))

	got, err := w.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	out, ok := got.(toolexec.Output)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if _, isString := out.Data.(string); isString {
		t.Fatalf("a small result was serialized: %#v", out.Data)
	}
	if out.Truncated != nil {
		t.Errorf("a small result reported truncation: %+v", out.Truncated)
	}
}

// TestALargeOutputIsCutAndTheRestIsOnDisk, with the model told exactly how to
// go and read it.
func TestALargeOutputIsCutAndTheRestIsOnDisk(t *testing.T) {
	body := strings.Repeat("x", 500_000)
	w, dir := wrapped(t, returns("grep", body))

	ctx := toolexec.WithCallID(context.Background(), "call-42")
	got, err := w.Invoke(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := got.(toolexec.Output)

	visible, _ := out.Data.(string)
	if len(visible) > toolexec.MaxToolOutputChars+4 {
		t.Fatalf("the model got %d characters", len(visible))
	}
	if out.Truncated == nil {
		t.Fatal("the model was not told the output was cut")
	}
	if out.Truncated.Original != len(body) {
		t.Errorf("Original = %d, want %d", out.Truncated.Original, len(body))
	}

	saved, err := os.ReadFile(filepath.Join(dir, "call-42.txt"))
	if err != nil {
		t.Fatalf("the full output was not saved: %v", err)
	}
	if len(saved) != len(body) {
		t.Fatalf("the saved output is %d bytes, want %d", len(saved), len(body))
	}
	if !strings.Contains(out.Truncated.Instruction, "call-42.txt") {
		t.Errorf("the instruction does not name the file: %s", out.Truncated.Instruction)
	}
}

// TestTheInstructionIsTheOneWeMeantToSend. The text is what turns a truncation
// from a dead end into a next step, so it is compared rather than trusted.
func TestTheInstructionIsTheOneWeMeantToSend(t *testing.T) {
	testx.AssertString(t, "toolexec/instruction",
		toolexec.Instruction("/home/dev/.aos/tmp/outputs/call-42.txt"))
}

// TestCuttingDoesNotSplitACharacter. Go strings are UTF-8, so the original's
// surrogate-pair concern becomes a rune-boundary concern.
func TestCuttingDoesNotSplitACharacter(t *testing.T) {
	for _, filler := range []string{"日", "🙂", "é"} {
		t.Run(filler, func(t *testing.T) {
			body := strings.Repeat(filler, toolexec.MaxToolOutputChars)
			w, _ := wrapped(t, returns("read", body))
			got, err := w.Invoke(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			visible := got.(toolexec.Output).Data.(string)
			if !utf8.ValidString(visible) {
				t.Fatalf("the truncated output is not valid UTF-8")
			}
		})
	}
}

// TestSpilloverIsBestEffort. A read-only filesystem must produce a bounded
// result, not an error about a directory the model cannot do anything about.
func TestSpilloverIsBestEffort(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly", "outputs")
	if err := os.MkdirAll(filepath.Dir(dir), 0o555); err != nil {
		t.Fatal(err)
	}
	w := toolexec.Wrap(returns("grep", strings.Repeat("x", 50_000)),
		toolexec.WithSpill(toolexec.NewSpiller(dir, discardLog())))

	got, err := w.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("a failed spill became a failed tool call: %v", err)
	}
	out := got.(toolexec.Output)
	if len(out.Data.(string)) > toolexec.MaxToolOutputChars+4 {
		t.Error("the model got the whole thing anyway")
	}
	if out.Output != "" {
		t.Errorf("the model was pointed at a file that does not exist: %s", out.Output)
	}
}

// TestACallIdIsNotAPath. The id comes from the model's tool call.
func TestACallIdIsNotAPath(t *testing.T) {
	w, dir := wrapped(t, returns("grep", strings.Repeat("x", 50_000)))
	ctx := toolexec.WithCallID(context.Background(), "../../escaped")

	got, err := w.Invoke(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := got.(toolexec.Output).Output
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("the spilled file landed at %s, outside %s", path, dir)
	}
}

// TestRotationRemovesWhatExpiredAndKeepsWhatDidNot.
func TestRotationRemovesWhatExpiredAndKeepsWhatDidNot(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)

	write := func(name string, age time.Duration) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		when := now.Add(-age)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	write("fresh.txt", 23*time.Hour)
	write("stale.txt", 25*time.Hour)

	removed, err := toolexec.Rotate(context.Background(), dir, toolexec.OutputTTL, now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d files, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "fresh.txt")); err != nil {
		t.Error("a file inside the window was removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.txt")); err == nil {
		t.Error("the expired file survived")
	}
}

// TestRotatingADirectoryThatIsNotThereIsNotAnError.
func TestRotatingADirectoryThatIsNotThereIsNotAnError(t *testing.T) {
	n, err := toolexec.Rotate(context.Background(), filepath.Join(t.TempDir(), "nope"),
		toolexec.OutputTTL, time.Now())
	if n != 0 || err != nil {
		t.Fatalf("got %d, %v", n, err)
	}
}

// TestMultimodalContentPassesThrough, because the provider is going to render
// it and truncating it would produce a broken image instead of a smaller one.
func TestMultimodalContentPassesThrough(t *testing.T) {
	part := toolexec.FilePart{
		MediaType: "image/png",
		Data:      strings.Repeat("A", 40_000),
	}
	w, _ := wrapped(t, returns("Imagine", part))

	got, err := w.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := got.(toolexec.Output)
	if out.Truncated != nil {
		t.Fatal("declared multimodal content was truncated")
	}
	if _, ok := out.Data.(toolexec.FilePart); !ok {
		t.Fatalf("Data = %T", out.Data)
	}
}

// TestAnUndeclaredBlobIsTrimmed. The other half: base64 that arrived inside an
// ordinary result is not something the model needs to read.
func TestAnUndeclaredBlobIsTrimmed(t *testing.T) {
	blob := "data:image/png;base64," + strings.Repeat("A", 4000)
	got := toolexec.TrimInlineBlob(blob)
	if len(got) >= len(blob) {
		t.Fatalf("the blob was not trimmed: %d characters", len(got))
	}
	if !strings.Contains(got, "omitted") {
		t.Errorf("the trim is silent: %q", got[len(got)-60:])
	}
	// Prose of the same length is left alone.
	prose := strings.Repeat("the quick brown fox jumps. ", 200)
	if toolexec.TrimInlineBlob(prose) != prose {
		t.Error("ordinary text was mistaken for a blob")
	}
}

// TestAValueThatCannotBeSerializedIsDescribed, not dropped. A tool that returns
// a cycle is a bug; a silent empty result makes it a bug nobody can find.
func TestAValueThatCannotBeSerializedIsDescribed(t *testing.T) {
	type node struct {
		Name string
		Next *node
	}
	a := &node{Name: "a"}
	a.Next = a

	w, _ := wrapped(t, returns("cycle", a))
	got, err := w.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := got.(toolexec.Output).Data.(string)
	if !strings.Contains(text, "could not be serialized") {
		t.Fatalf("Data = %#v", got.(toolexec.Output).Data)
	}
}

// TestAPanicInOneToolCostsThatCall. The third of the three panic boundaries.
func TestAPanicInOneToolCostsThatCall(t *testing.T) {
	w, _ := wrapped(t, tool("boom", func(context.Context, json.RawMessage) (any, error) {
		panic("index out of range")
	}))
	_, err := w.Invoke(context.Background(), nil)
	if err == nil {
		t.Fatal("the panic was swallowed")
	}
	var app *apperr.Error
	if !errors.As(err, &app) || !strings.Contains(app.Code, "PANIC") {
		t.Fatalf("err = %v", err)
	}
}

// TestTheRecorderIsToldWhatEveryCallCost, including the ones that failed.
func TestTheRecorderIsToldWhatEveryCallCost(t *testing.T) {
	rec := &recorder{}
	clock := &stepping{at: time.Unix(0, 0), step: 250 * time.Millisecond}

	ok := toolexec.Wrap(returns("fine", "x"), toolexec.WithRecorder(rec), toolexec.WithClock(clock.now))
	bad := toolexec.Wrap(tool("bad", func(context.Context, json.RawMessage) (any, error) {
		return nil, errors.New("nope")
	}), toolexec.WithRecorder(rec), toolexec.WithClock(clock.now))

	if _, err := ok.Invoke(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := bad.Invoke(context.Background(), nil); err == nil {
		t.Fatal("the failing tool succeeded")
	}
	if len(rec.calls) != 2 {
		t.Fatalf("recorded %d calls", len(rec.calls))
	}
	if rec.calls[0].elapsed != 250*time.Millisecond {
		t.Errorf("elapsed = %s", rec.calls[0].elapsed)
	}
	if rec.calls[1].err == nil {
		t.Error("the failure was recorded as a success")
	}
}

// TestTheRegistryRefusesANameItDoesNotKnowAndSaysWhatItDoes.
func TestTheRegistryRefusesANameItDoesNotKnowAndSaysWhatItDoes(t *testing.T) {
	r := toolexec.NewRegistry().Add(returns("Read", "x"), returns("Write", "y"), nil)
	if r.Len() != 2 {
		t.Fatalf("Len = %d", r.Len())
	}
	_, err := r.Invoke(context.Background(), "Reed", nil)
	if err == nil {
		t.Fatal("an unknown tool was invoked")
	}
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err = %v", err)
	}
	available, _ := app.Issues["available"].([]string)
	if len(available) != 2 {
		t.Fatalf("the error does not list what is available: %+v", app.Issues)
	}
}

// TestTheRegistryPublishesAStableOrder, because a tool list that reorders
// between runs invalidates every provider-side prompt cache.
func TestTheRegistryPublishesAStableOrder(t *testing.T) {
	r := toolexec.NewRegistry().Add(returns("Write", "a"), returns("Bash", "b"), returns("Read", "c"))
	var names []string
	for _, s := range r.Specs() {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "Bash,Read,Write" {
		t.Fatalf("order = %v", names)
	}
}

// TestADomainCommandBecomesAToolThatDemandsItsReasoning. The obligation is the
// Command Layer's and is not re-decided here; this checks the surface used is
// the one that carries it.
func TestADomainCommandBecomesAToolThatDemandsItsReasoning(t *testing.T) {
	type in struct {
		Title string `json:"title" jsonschema:"Headline of the note." validate:"required,notblank"`
		command.Reasoning
	}
	reg := command.NewRegistry()
	if err := command.Register(reg, command.Command[in, string]{
		Group: "notes", Name: "add", Summary: "Add a note.", Registry: true,
		Handler: func(_ context.Context, i in) (string, error) { return i.Title, nil },
	}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := reg.Lookup("notes_add")

	tl := toolexec.FromCommand(d)
	if tl.Name() != "notes_add" {
		t.Fatalf("Name = %q", tl.Name())
	}
	if _, err := tl.Invoke(context.Background(), json.RawMessage(`{"title":"x"}`)); err == nil {
		t.Fatal("a tool call without _reasoning was accepted")
	}
	got, err := tl.Invoke(context.Background(),
		json.RawMessage(`{"title":"x","_reasoning":"recording the decision we just made"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "x" {
		t.Fatalf("got %v", got)
	}
}

type recorder struct {
	calls []struct {
		name    string
		elapsed time.Duration
		err     error
	}
}

func (r *recorder) ToolCall(_ context.Context, name string, elapsed time.Duration, err error) {
	r.calls = append(r.calls, struct {
		name    string
		elapsed time.Duration
		err     error
	}{name, elapsed, err})
}

type stepping struct {
	at   time.Time
	step time.Duration
}

func (s *stepping) now() time.Time {
	out := s.at
	s.at = s.at.Add(s.step)
	return out
}
