package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/transport/realtime"
)

// collectionPublisher adapts the realtime hub to collections.Publisher, so a
// write to a dynamic collection reaches whoever is subscribed the same way
// every other domain event does — realtime.EventCollectionChanged already
// exists for exactly this, reserved ahead of the watcher a later task adds:
// the watcher's own fscollections.WithWatchPublisher takes the identical
// collections.Publisher, so it plugs into this same adapter without a
// second one.
type collectionPublisher struct {
	hub       *realtime.Hub
	workspace string
}

func (p collectionPublisher) Publish(ctx context.Context, ev collections.Changed) {
	if p.hub == nil {
		return
	}
	p.hub.Publish(ctx, realtime.ChannelFor(p.workspace), realtime.Event{
		Type: realtime.EventCollectionChanged, Workspace: p.workspace, Data: ev,
	})
}

// collectionsForViews adapts collection.Service to view.Collections: the
// narrower two-method slice a view needs to validate a bind against and to
// render from, kept a separate port so a change to collection.Service's own
// signature cannot silently break view without either package's tests
// noticing — see view/port.go's own comment on why this indirection exists.
type collectionsForViews struct{ svc *collection.Service }

// Get tries the skill-scoped declaration first when skill is given — a
// skill-scoped view's own source is commonly that same skill's own
// collection, and a skill-scoped collection is unreachable by id alone (see
// collection.GetInput's own doc) — and falls back to the workspace-scoped
// id, so a skill's view sourcing a workspace collection still resolves.
func (c collectionsForViews) Get(ctx context.Context, id, skill string) (*collection.Collection, error) {
	if skill != "" {
		if found, err := c.svc.Get(ctx, collection.GetInput{ID: id, Skill: skill}); err == nil {
			return found, nil
		}
	}
	return c.svc.Get(ctx, collection.GetInput{ID: id})
}

// ListRecords resolves the declaration the same way Get does — required so
// a skill-scoped collection's own field types and hooks are read from the
// right declaration — before listing its rows through RecordService, which
// addresses a collection's records by id alone regardless of scope (see
// collection.RecordService's own doc on that boundary).
func (c collectionsForViews) ListRecords(ctx context.Context, id, skill string, q collection.RecordQuery) ([]collection.Record, error) {
	if _, err := c.Get(ctx, id, skill); err != nil {
		return nil, err
	}
	return c.svc.Records().List(ctx, id, q)
}

// registryCommands adapts the command registry to view.Commands: whether an
// action's named command exists, and running it.
//
// Invoke dispatches through command.SurfaceHTTP, the same surface
// internal/transport/httpapi uses for a live request — a view's button is a
// shortcut to the call a browser would otherwise make directly, not an
// agent's own tool call, so it never demands the `_reasoning` a person
// clicking a button has no way to supply.
type registryCommands struct{ reg *command.Registry }

func (c registryCommands) Has(name string) bool {
	_, _, ok := c.reg.Lookup(name)
	return ok
}

func (c registryCommands) Invoke(ctx context.Context, name string, in json.RawMessage) (json.RawMessage, error) {
	d, _, ok := c.reg.Lookup(name)
	if !ok {
		// Unreachable through a validated view — Service.ExecuteAction
		// already refuses an action naming an unregistered command before
		// this is ever called (view/service.go) — kept as a defensive
		// refusal rather than a panic, for the same reason every boundary
		// in this system refuses instead of assuming its own caller.
		return nil, errActionCommandNotFound(name)
	}
	out, err := d.Invoke(ctx, command.SurfaceHTTP, in)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func errActionCommandNotFound(name string) error {
	return apperr.New("VIEW_ACTION_COMMAND_NOT_FOUND").
		Causer("app.registryCommands.Invoke").
		Msgf("command %q is not registered", name).
		Issue("command", name).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "the view names a command that no longer exists; edit or remove the action"})
}

// noopSkillHooks is the Deregister a skill's hooks need on Uninstall.
//
// It has nothing to do yet: hook registration in this build is static —
// event.Service.Register is called once, at boot, for the handlers wired
// into New — nothing here registers one dynamically per skill for this to
// tear down. A skill that ships hooks still installs; they simply do not
// intercept anything until dynamic hook registration exists, an honest gap
// disclosed here rather than a Deregister that pretends to undo work that
// was never done.
type noopSkillHooks struct{}

func (noopSkillHooks) Deregister(context.Context, string) error { return nil }

// noopSkillToolsets is the Close a skill's toolset connections need on
// Uninstall.
//
// It also has nothing to do: toolset.Service.Call opens a connection, uses
// it, and closes it within the one call (see toolset/service.go) — nothing
// in this build holds a toolset connection open across calls for Uninstall
// to close on a skill's behalf.
type noopSkillToolsets struct{}

func (noopSkillToolsets) Close(context.Context, string) error { return nil }
