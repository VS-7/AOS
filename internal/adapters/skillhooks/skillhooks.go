// Package skillhooks is the filesystem-and-bus implementation of
// skill.Hooks: it turns a skill's declared hooks into live handlers on the
// event bus — internal/adapters/hookexec, spawning the skill's own script as
// an external process — and takes them down again on uninstall.
//
// It lives under internal/adapters, like every other real implementation of
// a domain port, because it spawns an OS process — internal/architecture
// forbids os/exec anywhere under internal/domain.
package skillhooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/adapters/hookexec"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/pathx"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/skill"
)

// scriptMode is what a hook's own script needs to actually run.
// skillfiles.Files writes every file a package brings — the script included
// — at 0o644, the same as every other piece of a skill's content, none of
// which is meant to be executed directly; a hook's script is the one
// exception, and this is where that exception is made, not a broader mode
// change to a writer that has to stay right for everything else a package
// brings too.
const scriptMode = 0o755

// Bus is the slice of event.Service this adapter needs: attach a handler,
// remove one later by the id it was attached under. *event.Service already
// satisfies this with no wrapper.
type Bus interface {
	Register(h event.Handler)
	Deregister(ids ...string)
}

// Hooks is the skill.Hooks implementation.
//
// It tracks which handler ids belong to which skill itself, in memory,
// because the bus's own Deregister works by id and skill.Hooks.Deregister is
// called with only a skillID — the same "boundary this closes is whether
// anything of this skill's is still reachable" shape Toolsets.Close already
// documents. That tracking does not survive a restart, which is exactly why
// Reconcile exists: see its own doc comment.
type Hooks struct {
	bus  Bus
	root string // workspace root; hookexec.Handler's own fallback Dir

	mu    sync.Mutex
	owned map[string][]string // skillID -> handler ids Register attached
}

// New builds the adapter over bus and the workspace root every installed
// skill's own directory is resolved against.
func New(bus Bus, root string) *Hooks {
	return &Hooks{bus: bus, root: filepath.Clean(root), owned: map[string][]string{}}
}

// Register turns decls into live handlers on the bus, on skillID's behalf.
//
// Every handler id is namespaced skillID+"/"+decl.ID, so two skills each
// declaring a hook named "guard" cannot collide in the bus's own, otherwise
// global, id space — and so Deregister can find exactly this skill's own
// handlers back out of that space using nothing but the id it tracked here.
//
// A Command containing a path separator is a script the package itself
// ships — resolved to an absolute path inside the skill's own installed
// directory, confined there the same way every other path this system
// relocates on a skill's behalf is (pathx.ResolveInside). A bare name (no
// separator) is left alone, the same as a toolset's own Command: found on
// PATH when the handler actually runs, exactly like mcp-server::stdio's own
// Command already is.
//
// Nothing is registered on the bus until every decl has resolved cleanly:
// Apply's own rollback only calls Deregister when Register has already
// returned success (see internal/domain/skill's own comment on why), so a
// partial failure here must leave nothing behind for that rollback to miss.
func (h *Hooks) Register(_ context.Context, skillID string, decls []skill.HookDecl) error {
	if len(decls) == 0 {
		return nil
	}
	skillDir, err := pathx.Root(filepath.Join(h.root, collections.Root, "skills", skillID))
	if err != nil {
		return errResolveFailed(skillID, err)
	}

	type built struct {
		id      string
		handler *hookexec.Handler
	}
	prepared := make([]built, 0, len(decls))
	for _, d := range decls {
		command := d.Command
		if strings.ContainsAny(command, `/\`) {
			resolved, rerr := pathx.ResolveInside(skillDir, command)
			if rerr != nil {
				return errCommandOutside(skillID, d.ID, command, rerr)
			}
			if cerr := os.Chmod(resolved, scriptMode); cerr != nil {
				return errScriptNotExecutable(skillID, d.ID, command, cerr)
			}
			command = resolved
		}
		id := skillID + "/" + d.ID
		prepared = append(prepared, built{
			id: id,
			handler: hookexec.New(hookexec.Spec{
				ID: id, Events: d.Events, Command: command, Args: d.Args,
			}, h.root),
		})
	}

	ids := make([]string, 0, len(prepared))
	for _, b := range prepared {
		h.bus.Register(b.handler)
		ids = append(ids, b.id)
	}

	h.mu.Lock()
	h.owned[skillID] = append(h.owned[skillID], ids...)
	h.mu.Unlock()
	return nil
}

// Deregister removes every hook Register added for skillID. It is
// idempotent: a skillID Register was never called for is a no-op, the same
// as Uninstall's own idempotence on a skill already gone.
func (h *Hooks) Deregister(_ context.Context, skillID string) error {
	h.mu.Lock()
	ids := h.owned[skillID]
	delete(h.owned, skillID)
	h.mu.Unlock()
	if len(ids) == 0 {
		return nil
	}
	h.bus.Deregister(ids...)
	return nil
}
