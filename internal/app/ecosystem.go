package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"

	"github.com/OWNER/aos/internal/adapters/marketplacegit"
	"github.com/OWNER/aos/internal/adapters/marketplacehttp"
	"github.com/OWNER/aos/internal/adapters/skillhooks"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/bot"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/collection"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/marketplace"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/tunnel"
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

// reconcileHooks re-registers every active, already-installed skill's hooks
// once, at boot.
//
// skillhooks.Hooks starts every run with nothing in memory — the event bus
// is fresh, and so is this adapter's own bookkeeping of which handler ids
// belong to which skill — but a skill's hooks are still sitting on disk from
// whenever it was installed, in an earlier run of this same daemon. Without
// this, a skill's hooks silently stop intercepting anything the moment the
// process restarts, until someone reinstalls the skill to get them back —
// the same problem reconcileCollections (watch.go) solves for a dynamic
// collection's schema.json, and the same fix: read what a prior run already
// wrote and make it live again before a turn can reach it.
//
// A skill whose reconciliation fails — a hook script since deleted by hand,
// say — is logged and skipped rather than aborting the rest: one broken
// skill must not silently take every other installed skill's hooks down
// with it.
func reconcileHooks(ctx context.Context, skills *skill.Installer, hooks *skillhooks.Hooks, fetcher skill.Fetcher, root string, log *slog.Logger) {
	out, err := skills.List(ctx)
	if err != nil {
		log.Warn("could not reconcile skill hooks declared on disk", "err", err)
		return
	}
	for _, s := range out.Skills {
		if !s.Active || len(s.Metadata.Hooks) == 0 {
			continue
		}
		pkg, err := fetcher.Fetch(ctx, filepath.Join(root, collections.Root, "skills", s.ID), "")
		if err != nil {
			log.Warn("could not re-read an installed skill's hooks", "skill", s.ID, "err", err)
			continue
		}
		if err := hooks.Register(ctx, s.ID, pkg.Hooks); err != nil {
			log.Warn("could not re-register an installed skill's hooks", "skill", s.ID, "err", err)
		}
	}
}

// noopSkillToolsets is the Close a skill's toolset connections need on
// Uninstall.
//
// It also has nothing to do: toolset.Service.Call opens a connection, uses
// it, and closes it within the one call (see toolset/service.go) — nothing
// in this build holds a toolset connection open across calls for Uninstall
// to close on a skill's behalf.
type noopSkillToolsets struct{}

func (noopSkillToolsets) Close(context.Context, string) error { return nil }

// skillNetwork adapts skill.Installer to toolset.SkillNetwork: the hosts a
// skill's manifest declared under permissions.network, read fresh on every
// call rather than cached at install, so a skill update that narrows its
// network permissions takes effect on the very next toolsets_call.
type skillNetwork struct{ svc *skill.Installer }

func (n skillNetwork) NetworkHosts(ctx context.Context, skillID string) ([]string, error) {
	s, err := n.svc.Get(ctx, skillID)
	if err != nil {
		return nil, err
	}
	return s.Permissions.Network, nil
}

// goalTasksAdapter is the goal.Tasks a Goal's Delete needs: clearing the Goal
// field off every task that referenced it, without touching the tasks
// themselves. task.Service already has everything this takes — List already
// filters by Goal, Update already accepts a *string for it — so no change to
// that package was needed.
type goalTasksAdapter struct{ tasks *task.Service }

func (a goalTasksAdapter) ClearGoal(ctx context.Context, goalID string) error {
	found, err := a.tasks.List(ctx, task.ListInput{
		Goal:      goalID,
		Reasoning: command.Reasoning{Reasoning: "clearing this goal off every task that referenced it, before the goal itself is removed"},
	})
	if err != nil {
		return err
	}
	empty := ""
	for _, t := range found.Tasks {
		if _, err := a.tasks.Update(ctx, task.UpdateInput{
			ID: t.ID, Goal: &empty,
			Reasoning: command.Reasoning{Reasoning: "unlinking a task from a goal that is being deleted"},
		}); err != nil {
			return err
		}
	}
	return nil
}

// taskProjectUnlinker is one of project.Service.Delete's Unlinkers: it clears
// Project off every task that referenced the deleted project, the same
// clear-not-cascade rule goalTasksAdapter applies for goals.
type taskProjectUnlinker struct{ tasks *task.Service }

func (u taskProjectUnlinker) UnlinkProject(ctx context.Context, projectID string) error {
	found, err := u.tasks.List(ctx, task.ListInput{
		Project:   projectID,
		Reasoning: command.Reasoning{Reasoning: "clearing this project off every task that referenced it, before the project itself is removed"},
	})
	if err != nil {
		return err
	}
	empty := ""
	for _, t := range found.Tasks {
		if _, err := u.tasks.Update(ctx, task.UpdateInput{
			ID: t.ID, Project: &empty,
			Reasoning: command.Reasoning{Reasoning: "unlinking a task from a project that is being deleted"},
		}); err != nil {
			return err
		}
	}
	return nil
}

// goalProjectUnlinker is project.Service.Delete's other Unlinker: it clears
// Project off every goal that referenced the deleted project.
type goalProjectUnlinker struct{ goals *goal.Service }

func (u goalProjectUnlinker) UnlinkProject(ctx context.Context, projectID string) error {
	found, err := u.goals.List(ctx, goal.ListInput{
		Query:     goal.Query{Project: projectID},
		Reasoning: command.Reasoning{Reasoning: "clearing this project off every goal that referenced it, before the project itself is removed"},
	})
	if err != nil {
		return err
	}
	empty := ""
	for _, g := range found {
		if _, err := u.goals.Update(ctx, goal.UpdateInput{
			ID: g.ID, Project: &empty,
			Reasoning: command.Reasoning{Reasoning: "unlinking a goal from a project that is being deleted"},
		}); err != nil {
			return err
		}
	}
	return nil
}

// tunnelConfig adapts config.Service to tunnel.Config: the two substructures
// Start's guard reads, kept separate so tunnel does not import config's full
// surface just to name a type.
type tunnelConfig struct{ svc config.Service }

func (c tunnelConfig) Raw(ctx context.Context) (tunnel.RawConfig, error) {
	cfg, err := c.svc.Raw(ctx)
	if err != nil {
		return tunnel.RawConfig{}, err
	}
	return tunnel.RawConfig{
		SecurityEnabled: cfg.Security.Enabled,
		APIToken:        cfg.Security.APIToken,
		Hostname:        cfg.Tunnel.Hostname,
		Token:           cfg.Tunnel.Token,
	}, nil
}

// chatsForBot adapts chat.Service to bot.Chats: finding the conversation an
// inbound message belongs to, opening one when there is none yet, and
// relaying the message in as if a person sent it — which is what makes the
// agent actually answer it, through the same Send/Dispatcher path every
// other message in this system already takes.
type chatsForBot struct{ svc *chat.Service }

func (c chatsForBot) GetByChannel(ctx context.Context, provider, chatID string) (bot.ChatRef, error) {
	got, err := c.svc.GetByChannel(ctx, provider, chatID)
	if err != nil {
		return bot.ChatRef{}, err
	}
	return bot.ChatRef{ID: got.ID}, nil
}

func (c chatsForBot) CreateForChannel(ctx context.Context, provider, chatID, agentID, title string) (bot.ChatRef, error) {
	got, err := c.svc.Create(ctx, chat.CreateInput{
		Title: title, Kind: chat.KindExternal, Agent: agentID,
		Channel: &chat.ChannelMeta{Provider: provider, ChatID: chatID},
	})
	if err != nil {
		return bot.ChatRef{}, err
	}
	return bot.ChatRef{ID: got.ID}, nil
}

func (c chatsForBot) Send(ctx context.Context, chatID, text, agentID string) error {
	_, err := c.svc.Send(ctx, chat.SendInput{Chat: chatID, Text: text, Agent: agentID})
	return err
}

// tunnelPublicURL adapts tunnel.Service to bot.PublicURL: the one fact a
// webhook registration needs from it, and the boot-order enforcement point —
// see the design doc's "tunnel -> bots" and RegisterAll's own doc comment.
type tunnelPublicURL struct{ svc tunnel.Service }

func (t tunnelPublicURL) URL(ctx context.Context) (string, bool) {
	state, err := t.svc.Status(ctx)
	if err != nil || state.Status != tunnel.Running || state.URL == "" {
		return "", false
	}
	return state.URL, true
}

// marketplaceRegistries builds one marketplace.Registry per configured
// entry. A type this package does not recognise is skipped rather than
// refused — a config file written by a newer version naming a registry type
// this build does not know yet should not fail boot over it.
func marketplaceRegistries(entries []config.MarketplaceRegistry) (map[string]marketplace.Registry, []string) {
	regs := make(map[string]marketplace.Registry, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		switch e.Type {
		case "git":
			regs[e.ID] = marketplacegit.New(e.URL)
		case "http":
			regs[e.ID] = marketplacehttp.New(e.URL)
		default:
			continue
		}
		order = append(order, e.ID)
	}
	return regs, order
}
