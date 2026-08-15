package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/runtime/sandbox"
	"github.com/OWNER/aos/internal/runtime/toolexec"
	"github.com/OWNER/aos/internal/runtime/toolexec/tools"
)

func workspace(t *testing.T) (*toolexec.Registry, string) {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), "# Title\nsecond line\nthird line\n")
	write(t, filepath.Join(root, "src", "main.go"), "package main\n\nfunc main() {}\n")
	write(t, filepath.Join(root, "src", "util.go"), "package main\n\n// TODO: rename\nfunc util() {}\n")

	s, err := sandbox.New(sandbox.Options{
		WorkspacePath: root,
		Permissions:   sandbox.Permissions{Read: true, Write: true, Delete: true, Execute: true},
		Exec:          sandbox.ExecPolicy{Policy: sandbox.PolicyAllowlist, Allow: []string{"echo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return toolexec.NewRegistry().Add(tools.FS(s)...), s.Root()
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func call(t *testing.T, r *toolexec.Registry, name string, payload map[string]any) map[string]any {
	t.Helper()
	got, err := invoke(r, name, payload)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	out, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("%s returned %T", name, got)
	}
	return out
}

func invoke(r *toolexec.Registry, name string, payload map[string]any) (any, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["_reasoning"] = "exercising the tool from a test"
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return r.Invoke(context.Background(), name, raw)
}

// TestTheToolsetCarriesTheNamesAModelAlreadyKnows.
func TestTheToolsetCarriesTheNamesAModelAlreadyKnows(t *testing.T) {
	r, _ := workspace(t)
	want := []string{"Bash", "Edit", "Glob", "Grep", "Read", "Write"}
	var got []string
	for _, s := range r.Specs() {
		got = append(got, s.Name)
		if s.Description == "" {
			t.Errorf("%s has no description; the model reads it to decide", s.Name)
		}
		if s.InputSchema == nil {
			t.Errorf("%s has no input schema", s.Name)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("tools = %v, want %v", got, want)
	}
}

// TestReadReturnsNumberedLinesAndASlice, which is what makes the offset and
// limit parameters worth having.
func TestReadReturnsNumberedLinesAndASlice(t *testing.T) {
	r, _ := workspace(t)

	whole := call(t, r, "Read", map[string]any{"file_path": "README.md"})
	content := whole["content"].(string)
	if !strings.Contains(content, "     1\t# Title") {
		t.Fatalf("content = %q", content)
	}

	slice := call(t, r, "Read", map[string]any{"file_path": "README.md", "offset": 2, "limit": 1})
	body := slice["content"].(string)
	if strings.Contains(body, "# Title") || !strings.Contains(body, "second line") {
		t.Fatalf("the slice is wrong: %q", body)
	}
	if slice["truncated"] != true {
		t.Error("a partial read did not say it was partial")
	}
}

// TestWriteAndEdit, including the guard that makes an ambiguous edit an error.
func TestWriteAndEdit(t *testing.T) {
	r, root := workspace(t)

	call(t, r, "Write", map[string]any{"file_path": "notes/plan.md", "content": "one\ntwo\none\n"})
	if _, err := os.Stat(filepath.Join(root, "notes", "plan.md")); err != nil {
		t.Fatalf("the file was not created: %v", err)
	}

	if _, err := invoke(r, "Edit", map[string]any{
		"file_path": "notes/plan.md", "old_string": "one", "new_string": "1",
	}); err == nil {
		t.Fatal("an edit matching two places was applied")
	} else if !strings.Contains(err.Error(), "appears 2 times") {
		t.Fatalf("err = %v", err)
	}

	call(t, r, "Edit", map[string]any{
		"file_path": "notes/plan.md", "old_string": "one", "new_string": "1", "replace_all": true,
	})
	body, _ := os.ReadFile(filepath.Join(root, "notes", "plan.md"))
	if string(body) != "1\ntwo\n1\n" {
		t.Fatalf("body = %q", body)
	}

	if _, err := invoke(r, "Edit", map[string]any{
		"file_path": "notes/plan.md", "old_string": "absent", "new_string": "x",
	}); err == nil {
		t.Fatal("an edit with no match succeeded")
	}
	if _, err := invoke(r, "Edit", map[string]any{
		"file_path": "notes/plan.md", "old_string": "", "new_string": "x",
	}); err == nil {
		t.Fatal("an empty match succeeded")
	}
}

// TestGlobAndGrepFindThingsWithoutRunningAnything, which matters because the
// default policy allows no execution at all.
func TestGlobAndGrepFindThingsWithoutRunningAnything(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), "package a\n// TODO: fix\n")
	write(t, filepath.Join(root, "b.txt"), "todo in prose\n")

	s, err := sandbox.New(sandbox.Options{
		WorkspacePath: root,
		Permissions:   sandbox.DefaultPermissions(), // read only, no execution
	})
	if err != nil {
		t.Fatal(err)
	}
	r := toolexec.NewRegistry().Add(tools.FS(s)...)

	found := call(t, r, "Glob", map[string]any{"pattern": "**/*.go"})
	if got := found["total"]; got != 1 {
		t.Fatalf("Glob total = %v", got)
	}

	hits := call(t, r, "Grep", map[string]any{"pattern": "todo"})
	if got := hits["total"]; got != 2 {
		t.Fatalf("a case-insensitive search found %v, want 2", got)
	}
	sensitive := call(t, r, "Grep", map[string]any{"pattern": "TODO", "case_sensitive": true})
	if got := sensitive["total"]; got != 1 {
		t.Fatalf("a case-sensitive search found %v, want 1", got)
	}
	scoped := call(t, r, "Grep", map[string]any{"pattern": "todo", "glob": "**/*.go"})
	if got := scoped["total"]; got != 1 {
		t.Fatalf("a scoped search found %v", got)
	}
}

// TestAnInvalidExpressionSaysSoRatherThanFindingNothing.
func TestAnInvalidExpressionSaysSoRatherThanFindingNothing(t *testing.T) {
	r, _ := workspace(t)
	if _, err := invoke(r, "Grep", map[string]any{"pattern": "([unclosed"}); err == nil {
		t.Fatal("an invalid expression returned an empty result")
	}
}

// TestBashRunsWhatTheAllowlistAllows, and nothing else.
func TestBashRunsWhatTheAllowlistAllows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX binaries")
	}
	r, _ := workspace(t)

	got, err := invoke(r, "Bash", map[string]any{"command": "echo", "args": []string{"hello"}})
	if err != nil {
		t.Skipf("echo is not available as a binary here: %v", err)
	}
	res, ok := got.(sandbox.Result)
	if !ok {
		t.Fatalf("Bash returned %T", got)
	}
	if !strings.Contains(res.Stdout.Content, "hello") {
		t.Fatalf("stdout = %q", res.Stdout.Content)
	}

	if _, err := invoke(r, "Bash", map[string]any{"command": "curl", "args": []string{"https://example.test"}}); err == nil {
		t.Fatal("a binary outside the allowlist ran")
	} else if !strings.Contains(err.Error(), "allowlist") && !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err = %v", err)
	}
}

// TestAMalformedPayloadIsAnErrorTheModelCanActon.
func TestAMalformedPayloadIsAnErrorTheModelCanActOn(t *testing.T) {
	r, _ := workspace(t)
	_, err := r.Invoke(context.Background(), "Read", json.RawMessage(`{"file_path": 42}`))
	if err == nil {
		t.Fatal("a payload with the wrong type was accepted")
	}
	if !strings.Contains(err.Error(), "TOOL_INPUT_INVALID") {
		t.Fatalf("err = %v", err)
	}
}

// TestAToolHoldsOnlyTheInterfaceItNeeds. The claim is about the types rather
// than about a check somewhere: if FileReader ever gains a method that changes
// the filesystem, the tools that hold it silently gain that power too.
func TestAToolHoldsOnlyTheInterfaceItNeeds(t *testing.T) {
	mutating := map[string]bool{"WriteFile": true, "Mkdir": true, "Remove": true, "Run": true, "Start": true}

	reader := reflect.TypeOf((*sandbox.FileReader)(nil)).Elem()
	for i := range reader.NumMethod() {
		if name := reader.Method(i).Name; mutating[name] {
			t.Errorf("sandbox.FileReader has %s; every tool that only reads can now use it", name)
		}
	}

	globber := reflect.TypeOf((*sandbox.Globber)(nil)).Elem()
	for i := range globber.NumMethod() {
		if name := globber.Method(i).Name; mutating[name] {
			t.Errorf("sandbox.Globber has %s", name)
		}
	}

	writer := reflect.TypeOf((*sandbox.FileWriter)(nil)).Elem()
	for i := range writer.NumMethod() {
		if name := writer.Method(i).Name; name == "ReadFile" {
			t.Errorf("sandbox.FileWriter can read; Write was supposed to be unable to")
		}
	}
}
