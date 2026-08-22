package skillhooks_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/adapters/skillhooks"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/skill"
)

// fakeBus is skillhooks.Bus in memory — enough to prove Hooks registers and
// deregisters the right ids, without a real event.Service or a real spawned
// process. The end-to-end proof that a registered hook actually intercepts
// an Emit lives in skillhooks_integration_test.go, against the real bus.
type fakeBus struct {
	mu        sync.Mutex
	handlers  map[string]event.Handler
	deregLast []string
}

func newFakeBus() *fakeBus { return &fakeBus{handlers: map[string]event.Handler{}} }

func (b *fakeBus) Register(h event.Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[h.ID()] = h
}

func (b *fakeBus) Deregister(ids ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deregLast = append([]string(nil), ids...)
	for _, id := range ids {
		delete(b.handlers, id)
	}
}

func (b *fakeBus) has(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.handlers[id]
	return ok
}

func (b *fakeBus) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.handlers)
}

// skillDir builds a fake installed skill's own directory under root, the
// shape skillhooks.New(root) expects: root/.aos/skills/{id}/...
func skillDir(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, collections.Root, "skills", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("err is not *apperr.Error: %v", err)
	}
	return e.Code
}

func TestRegisterAttachesANamespacedHandlerPerDecl(t *testing.T) {
	root := t.TempDir()
	skillDir(t, root, "crm")
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "sh"},
		{ID: "watch", Events: []event.Type{event.PostToolUse}, Command: "sh"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bus.has("crm/guard") || !bus.has("crm/watch") {
		t.Fatalf("expected crm/guard and crm/watch on the bus, got %d handlers", bus.count())
	}
}

func TestRegisterWithNoDeclsTouchesNothing(t *testing.T) {
	root := t.TempDir()
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	if err := h.Register(context.Background(), "crm", nil); err != nil {
		t.Fatal(err)
	}
	if bus.count() != 0 {
		t.Fatalf("bus has %d handlers, want 0", bus.count())
	}
}

// A bare command name — no path separator — is left alone: it is resolved on
// PATH when the handler actually runs, the same as an mcp-server::stdio
// toolset's own Command already is. This proves Register does not require
// the skill's own directory to contain anything for that case.
func TestRegisterLeavesABareCommandNameAlone(t *testing.T) {
	root := t.TempDir()
	skillDir(t, root, "crm")
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	if err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "python3"},
	}); err != nil {
		t.Fatal(err)
	}
	if !bus.has("crm/guard") {
		t.Fatal("crm/guard was not registered")
	}
}

// A Command naming a real script inside the skill's own directory is
// resolved to an absolute path and made executable — skillfiles.Files writes
// every file a package brings at 0o644, and a script needs its execute bit
// or the handler's first real invocation fails with "permission denied".
func TestRegisterResolvesAndMakesAPackageScriptExecutable(t *testing.T) {
	root := t.TempDir()
	dir := skillDir(t, root, "crm")
	script := filepath.Join(dir, "hooks", "guard.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	bus := newFakeBus()
	h := skillhooks.New(bus, root)
	if err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "hooks/guard.sh"},
	}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("script mode = %v, want an execute bit set", info.Mode())
	}
}

// The path-traversal boundary every other adapter that relocates a skill's
// content already enforces (artifactfiles, skillfiles): a hook's own Command
// cannot point outside the skill's own installed directory.
func TestRegisterRefusesACommandThatEscapesTheSkillsOwnDirectory(t *testing.T) {
	root := t.TempDir()
	skillDir(t, root, "crm")
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "../../../etc/passwd"},
	})
	if code := codeOf(t, err); code != "AOS_SKILLHOOKS_COMMAND_OUTSIDE" {
		t.Fatalf("code = %q", code)
	}
	if bus.count() != 0 {
		t.Fatal("a refused hook must not reach the bus")
	}
}

// Atomicity: if any decl in the batch is refused, nothing in the batch is
// registered — the same all-or-nothing guarantee skill.defaultApplier's own
// rollback depends on (a partial Register with no way to know which ids
// succeeded would leave orphans Deregister can never find).
func TestRegisterIsAllOrNothingAcrossTheWholeBatch(t *testing.T) {
	root := t.TempDir()
	skillDir(t, root, "crm")
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "good", Events: []event.Type{event.PreToolUse}, Command: "sh"},
		{ID: "bad", Events: []event.Type{event.PreToolUse}, Command: "../../../etc/passwd"},
	})
	if err == nil {
		t.Fatal("a batch with one bad decl reported success")
	}
	if bus.count() != 0 {
		t.Fatalf("bus has %d handlers, want 0 — the good decl must not have registered either", bus.count())
	}
}

func TestDeregisterRemovesExactlyThisSkillsOwnHandlers(t *testing.T) {
	root := t.TempDir()
	skillDir(t, root, "crm")
	skillDir(t, root, "other")
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	if err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "sh"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.Register(context.Background(), "other", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "sh"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := h.Deregister(context.Background(), "crm"); err != nil {
		t.Fatal(err)
	}
	if bus.has("crm/guard") {
		t.Fatal("crm/guard survived its own skill's Deregister")
	}
	if !bus.has("other/guard") {
		t.Fatal("other/guard was removed by crm's Deregister — ids collided across skills")
	}
}

func TestDeregisterOfASkillNeverRegisteredIsANoOp(t *testing.T) {
	root := t.TempDir()
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	if err := h.Deregister(context.Background(), "never-installed"); err != nil {
		t.Fatal(err)
	}
	if bus.deregLast != nil {
		t.Fatalf("bus.Deregister was called with %v, want it never called", bus.deregLast)
	}
}

func TestDeregisterIsIdempotent(t *testing.T) {
	root := t.TempDir()
	skillDir(t, root, "crm")
	bus := newFakeBus()
	h := skillhooks.New(bus, root)
	if err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "sh"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := h.Deregister(context.Background(), "crm"); err != nil {
		t.Fatal(err)
	}
	if err := h.Deregister(context.Background(), "crm"); err != nil {
		t.Fatalf("second Deregister must not error: %v", err)
	}
}

// A workspace root that does not resolve at all — not "the skill directory
// is missing" (pathx.Root tolerates that, see its own doc), but a genuine
// resolution failure — is reported rather than silently treated as "nothing
// to confine against".
func TestRegisterReportsAWorkspaceRootThatCannotBeResolved(t *testing.T) {
	root := t.TempDir()
	// A file where a directory is expected makes EvalSymlinks fail with
	// something other than "not exist" once collections.Root/"skills"/"crm"
	// is appended beneath it.
	blocker := filepath.Join(root, collections.Root)
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	bus := newFakeBus()
	h := skillhooks.New(bus, root)

	err := h.Register(context.Background(), "crm", []skill.HookDecl{
		{ID: "guard", Events: []event.Type{event.PreToolUse}, Command: "sh"},
	})
	if err == nil {
		t.Fatal("Register over an unresolvable root reported success")
	}
	if bus.count() != 0 {
		t.Fatal("nothing should have reached the bus")
	}
}
