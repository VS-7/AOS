package app

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/artifact"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/config"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/instruction"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/project"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/skill"
	"github.com/OWNER/aos/internal/domain/template"
	"github.com/OWNER/aos/internal/domain/view"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/prompt"
	"github.com/OWNER/aos/internal/runtime/providers"
	"github.com/OWNER/aos/internal/transport/realtime"

	// The provider adapters register themselves from init. Importing them here
	// is the whole of adding a provider to a build, which is the point of the
	// registry: the core never learns their names.
	_ "github.com/OWNER/aos/internal/runtime/providers/anthropic"
	_ "github.com/OWNER/aos/internal/runtime/providers/antigravity"
	_ "github.com/OWNER/aos/internal/runtime/providers/compat"
	_ "github.com/OWNER/aos/internal/runtime/providers/google"
	_ "github.com/OWNER/aos/internal/runtime/providers/openai"
)

// promptClock adds the zone to a clock, because the agent derives every
// timezone conversion from the offset it is handed and a bare instant leaves it
// guessing. The zone comes from the installation's configuration, falling back
// to the machine's.
type promptClock struct {
	clock clockx.Clock
	zone  *time.Location
}

func (c promptClock) Now() time.Time           { return c.clock.Now().In(c.location()) }
func (c promptClock) Location() *time.Location { return c.location() }

func (c promptClock) location() *time.Location {
	if c.zone != nil {
		return c.zone
	}
	return time.UTC
}

func zoneFrom(cfg config.Service, log *slog.Logger) *time.Location {
	current, err := cfg.Get(context.Background(), config.GetInput{})
	if err != nil || current.Region.Timezone == "" {
		return time.Local
	}
	zone, err := time.LoadLocation(current.Region.Timezone)
	if err != nil {
		log.Warn("the configured timezone is not one this machine knows; using the machine's",
			"timezone", current.Region.Timezone)
		return time.Local
	}
	return zone
}

// reader answers the two questions the context document asks of the rest of the
// system: what the workspace holds, and how much this agent remembers.
type reader struct {
	workspaces   *workspace.Service
	agents       *agent.Service
	memories     *memory.Service
	skills       *skill.Installer
	templates    *template.Service
	views        *view.Service
	goals        *goal.Service
	routines     *routine.Service
	projects     *project.Service
	artifacts    *artifact.Service
	instructions *instruction.Service
}

// countLimit is high enough that the per-category counts are exact for any
// realistic graph. It is a scan, which is a divergence: the specification asks
// for facets from the search index so the cost does not grow with the graph.
// Recorded as pending rather than pretended.
const countLimit = 100_000

// Inventory reads the names of everything in the workspace.
func (r reader) Inventory(ctx context.Context, id string) (prompt.Inventory, error) {
	out := prompt.Inventory{Workspace: prompt.WorkspaceRef{ID: id}}

	if r.workspaces != nil {
		if w, err := r.workspaces.Get(ctx, workspace.GetInput{Workspace: id}); err == nil && w != nil {
			out.Workspace = prompt.WorkspaceRef{ID: w.ID, Name: w.Name, Path: w.Path}
		}
		if inv, err := r.workspaces.Inventory(ctx, workspace.InventoryInput{Workspace: id}); err == nil {
			for _, c := range inv.Collections {
				out.Collections = append(out.Collections, c.Name)
			}
		}
	}
	if r.agents != nil {
		if list, err := r.agents.List(ctx, agent.ListInput{}); err == nil {
			for _, a := range list.Agents {
				out.Agents = append(out.Agents, a.ID)
			}
		}
	}
	if r.skills != nil {
		if list, err := r.skills.List(ctx); err == nil {
			for _, s := range list.Skills {
				out.Skills = append(out.Skills, s.ID)
			}
		}
	}
	if r.templates != nil {
		if list, err := r.templates.List(ctx, template.ListInput{}); err == nil {
			for _, t := range list.Templates {
				out.Templates = append(out.Templates, t.ID)
			}
		}
	}
	if r.views != nil {
		if list, err := r.views.List(ctx, view.ListInput{}); err == nil {
			for _, v := range list.Views {
				out.Views = append(out.Views, v.ID)
			}
		}
	}
	if r.goals != nil {
		if list, err := r.goals.List(ctx, goal.ListInput{}); err == nil {
			for _, g := range list {
				out.Goals = append(out.Goals, g.ID)
			}
		}
	}
	if r.routines != nil {
		if list, err := r.routines.List(ctx, routine.ListInput{}); err == nil {
			for _, rt := range list.Routines {
				out.Routines = append(out.Routines, rt.ID)
			}
		}
	}
	if r.projects != nil {
		if list, err := r.projects.List(ctx, project.ListInput{}); err == nil {
			for _, p := range list {
				out.Projects = append(out.Projects, p.ID)
			}
		}
	}
	if r.artifacts != nil {
		if list, err := r.artifacts.List(ctx, artifact.ListInput{}); err == nil {
			for _, ar := range list {
				out.Artifacts = append(out.Artifacts, ar.ID)
			}
		}
	}
	if r.instructions != nil {
		if list, err := r.instructions.List(ctx, instruction.ListInput{}); err == nil {
			for _, ins := range list.Instructions {
				out.Instructions = append(out.Instructions, ins.ID)
			}
		}
	}
	sort.Strings(out.Collections)
	sort.Strings(out.Agents)
	sort.Strings(out.Skills)
	sort.Strings(out.Templates)
	sort.Strings(out.Views)
	sort.Strings(out.Goals)
	sort.Strings(out.Routines)
	sort.Strings(out.Projects)
	sort.Strings(out.Artifacts)
	sort.Strings(out.Instructions)
	return out, nil
}

// MemoryCounts reports how many memories the agent holds, per category.
func (r reader) MemoryCounts(ctx context.Context, agentID string) (prompt.MemoryCounts, error) {
	if r.memories == nil {
		return prompt.MemoryCounts{}, nil
	}
	found, err := r.memories.Recall(ctx, memory.RecallInput{
		Agent: agentID, Status: memory.StatusActive, Limit: countLimit,
	})
	if err != nil {
		return prompt.MemoryCounts{}, err
	}
	counts := prompt.MemoryCounts{Total: found.Total, ByCategory: map[string]int{}}
	for _, m := range found.Memories {
		counts.ByCategory[string(m.Category)]++
	}
	return counts, nil
}

// models resolves an agent to a provider, reading the configuration afresh each
// turn so that a key added while the daemon runs is used on the next message.
type models struct {
	config config.Service
	home   string
}

// For resolves the model cascade and builds the provider that owns it.
func (m models) For(ctx context.Context, a *agent.Agent) (agentloop.LLMProvider, agentloop.ModelRef, error) {
	// Raw, not Get: Get redacts every provider key to a fingerprint before it
	// leaves the config package (ADR-0010), and keyFor below needs the real
	// one to hand the provider adapter, or every API-key provider call would
	// authenticate with the fingerprint string instead of the key the user
	// saved. Raw never crosses a transport boundary, which is what makes it
	// safe here — this is the runtime resolving its own model, not answering
	// a caller.
	current, err := m.config.Raw(ctx)
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}
	slot := current.Agents.Models[config.SlotDefault]

	ref, err := agentloop.Resolve(
		agentloop.AgentModel{Provider: a.Provider, Model: a.Model, Reasoning: a.Reasoning},
		agentloop.ConfigModel{Provider: slot.Provider, Model: slot.Model, Reasoning: slot.Reasoning},
	)
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}

	provider, err := providers.Build(ref.Provider, providers.Config{
		APIKey: keyFor(current, ref.Provider),
		Home:   m.home,
	})
	if err != nil {
		return nil, agentloop.ModelRef{}, err
	}
	return provider, ref, nil
}

// keyFor finds the credential for a provider.
//
// The configuration service redacts secrets on the way out to anything an agent
// can reach; this reads the unredacted value through the path reserved for the
// runtime, which is the only consumer that needs it.
func keyFor(c config.Config, provider string) string {
	for _, p := range c.Agents.Providers {
		if p.ID == provider {
			return p.Key
		}
	}
	return ""
}

// publisher pushes what a turn is producing to whoever is watching.
type publisher struct {
	hub *realtime.Hub

	// channel names the workspace to publish under when the turn does not.
	// See channelFor.
	channel func(ctx context.Context, workspaceID string) string
}

// channelFor picks the channel a turn's events go out on.
//
// Every socket subscribes to the channel of the workspace it resolved, so a
// mismatch here is not a degraded experience — it is silence. And the
// mismatch was the normal case: `session.Deps.WorkspaceID` comes from
// `AOS_WORKSPACE_ID` (`wire.go`'s `active`), which is a deliberate pin that
// nothing sets in a desktop installation — the window names its workspace
// per request instead. So the id was empty, every delta and every done
// event was published to `ChannelFor("")`, and the interface never learned
// anything had happened until something else forced it to refetch. That is
// the "I have to reload the page to see the answer" symptom.
//
// The fallback is the same single-installation assumption
// `workspace.Service.AuthorizeWorkspace` already makes: one registered
// workspace is unambiguous. It is resolved per publish rather than at
// wiring because a fresh installation registers its workspace *after* the
// daemon is up (`workspace_introspect`, from the desktop's own start-up),
// so anything computed at wiring time would still be empty.
func (p publisher) channelFor(ctx context.Context, workspaceID string) string {
	if workspaceID == "" && p.channel != nil {
		workspaceID = p.channel(ctx, workspaceID)
	}
	return realtime.ChannelFor(workspaceID)
}

func (p publisher) ChatStarted(ctx context.Context, workspaceID, chatID, agentID string) {
	if p.hub == nil {
		return
	}
	p.hub.Publish(ctx, p.channelFor(ctx, workspaceID), realtime.Event{
		Type: realtime.EventChatStarted,
		Data: map[string]any{"chat": chatID, "agent": agentID},
	})
}

func (p publisher) ChatDelta(ctx context.Context, workspaceID, chatID, text, reasoning string) {
	if p.hub == nil || (text == "" && reasoning == "") {
		return
	}
	p.hub.Publish(ctx, p.channelFor(ctx, workspaceID), realtime.Event{
		Type: realtime.EventChatDelta,
		Data: map[string]any{"chat": chatID, "text": text, "reasoning": reasoning},
	})
}

func (p publisher) ChatMessage(ctx context.Context, workspaceID, chatID string, message chat.Message) {
	if p.hub == nil {
		return
	}
	p.hub.Publish(ctx, p.channelFor(ctx, workspaceID), realtime.Event{
		Type: realtime.EventChatMessage,
		Data: map[string]any{"chat": chatID, "message": message},
	})
}

func (p publisher) ChatDone(ctx context.Context, workspaceID, chatID, agentID string, usage chat.TokenUsage) {
	if p.hub == nil {
		return
	}
	p.hub.Publish(ctx, p.channelFor(ctx, workspaceID), realtime.Event{
		Type: realtime.EventChatDone,
		Data: map[string]any{"chat": chatID, "agent": agentID, "usage": usage},
	})
}

// approvalNotifier turns a waiting approval into an event the desktop can show.
type approvalNotifier struct {
	hub       *realtime.Hub
	workspace string
}

func (n approvalNotifier) ApprovalRequested(ctx context.Context, req event.ApprovalRequest) {
	if n.hub == nil {
		return
	}
	workspaceID := req.Workspace
	if workspaceID == "" {
		workspaceID = n.workspace
	}
	n.hub.Publish(ctx, realtime.ChannelFor(workspaceID), realtime.Event{
		Type: realtime.EventApprovalRequest, Workspace: workspaceID, Data: req,
	})
}

func (n approvalNotifier) ApprovalSettled(ctx context.Context, req event.ApprovalRequest, res event.ApprovalResult) {
	if n.hub == nil {
		return
	}
	workspaceID := req.Workspace
	if workspaceID == "" {
		workspaceID = n.workspace
	}
	n.hub.Publish(ctx, realtime.ChannelFor(workspaceID), realtime.Event{
		Type: realtime.EventApprovalRequest, Workspace: workspaceID,
		Data: map[string]any{"id": req.ID, "settled": true, "approved": res.Approved, "reason": res.Reason},
	})
}

// approvalsGroup registers the two commands that answer a waiting request.
//
// Neither is in the agent's registry, and that is the boundary: an agent that
// could approve its own tool call would make the whole mechanism decoration.
func registerApprovals(reg *command.Registry, broker *event.Broker) {
	reg.DescribeGroup(command.GroupDoc{
		Name:    "approvals",
		Tool:    "Approvals",
		Summary: "See and answer the tool calls waiting on a person.",
		Doc: `Tool calls an agent is waiting on.

A policy hook can answer "ask" instead of allow or deny, and when it does the
call stops here until somebody decides. Deciding is not something an agent can
do: these commands are deliberately outside the agent's own tool registry.

A request that nobody answers becomes a denial when its deadline passes. There
is no path by which waiting turns into approval.`,
	})

	command.MustRegister(reg, command.Command[PendingInput, PendingOutput]{
		Group: "approvals", Name: "list",
		Summary:     "List the tool calls waiting for a decision.",
		Doc:         "List the tool calls waiting for a person to allow or refuse them, oldest first.",
		Annotations: command.Annotations{Title: "Pending approvals", ReadOnlyHint: true},
		Handler: func(_ context.Context, _ PendingInput) (PendingOutput, error) {
			pending := broker.Pending()
			return PendingOutput{Pending: pending, Total: len(pending)}, nil
		},
	})

	command.MustRegister(reg, command.Command[DecideInput, DecideOutput]{
		Group: "approvals", Name: "decide",
		Summary: "Allow or refuse a waiting tool call.",
		Doc: `Answer a waiting tool call.

The payload may be corrected before approving, which is often what somebody
actually wants: not "no", but "not like that". Approving once does not approve
the same call in every future context.`,
		Annotations: command.Annotations{Title: "Decide an approval"},
		Handler: func(_ context.Context, in DecideInput) (DecideOutput, error) {
			landed := broker.Decide(in.ID, event.ApprovalResult{
				Approved: in.Approved, Reason: in.Reason,
				UpdatedInput: in.UpdatedInput, Remember: event.RememberScope(in.Remember),
			})
			return DecideOutput{ID: in.ID, Settled: landed}, nil
		},
	})
}
