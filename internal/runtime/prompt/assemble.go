package prompt

import (
	"context"
	"log/slog"
	"runtime"
	"strconv"
	"time"

	"github.com/OWNER/aos/internal/core/build"
)

// AgentRef is the identity the assembled document is built around.
type AgentRef struct {
	ID           string
	Name         string
	Role         string
	Instructions string // the Markdown body of the agent's file
	Orchestrator bool
}

// AssembleInput is one turn's worth of context.
type AssembleInput struct {
	Agent     AgentRef
	Workspace string

	// SessionStartedAt and LastUserMessageAt are omitted from the document
	// when they are zero. The agent must never be handed invented time.
	SessionStartedAt  time.Time
	LastUserMessageAt time.Time

	// ExternalContent is anything that came from outside the machine: a page
	// the agent fetched, a resource a skill shipped. It gets its own block, at
	// the only trust level that fits.
	ExternalContent []External
}

// External is one piece of untrusted content.
type External struct {
	Title  string
	Origin string
	Body   string
}

// Reader supplies the two things the document needs from the rest of the
// system. It is one port with two methods rather than two ports because a
// caller that has one always has the other, and both fail the same way.
type Reader interface {
	Inventory(ctx context.Context, workspace string) (Inventory, error)
	MemoryCounts(ctx context.Context, agent string) (MemoryCounts, error)
}

// Assembler builds the context document for one turn.
type Assembler struct {
	base     string
	clock    Clock
	reader   Reader
	renderer Renderer
	log      *slog.Logger
}

// Clock reports the current time and the zone it is in. The zone is part of the
// port because the agent derives every timezone conversion from the offset it
// is given, and a clock that reports an instant without one leaves it guessing.
type Clock interface {
	Now() time.Time
	Location() *time.Location
}

// Deps is what the assembler is built from.
type Deps struct {
	Base     string // empty means the embedded master prompt
	Clock    Clock
	Reader   Reader
	Renderer Renderer
	Log      *slog.Logger
}

// NewAssembler wires the assembler.
func NewAssembler(d Deps) *Assembler {
	base := d.Base
	if base == "" {
		base = Base
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	renderer := d.Renderer
	if renderer == nil {
		renderer = NewLiquid()
	}
	return &Assembler{base: base, clock: d.Clock, reader: d.Reader, renderer: renderer, log: log}
}

// Assemble produces the document.
//
// A failure to read the inventory does not fail the turn. The original wraps
// two of the ten queries in try/catch for exactly this reason: a workspace
// whose artifacts directory is broken should produce an agent that cannot see
// artifacts, not an agent that cannot start.
func (a *Assembler) Assemble(ctx context.Context, in AssembleInput) (string, error) {
	b := New().
		WithRenderer(a.renderer).
		WithSystemInstructions(a.base).
		WithIdentity(Identity{
			ID:           in.Agent.ID,
			Name:         in.Agent.Name,
			Role:         roleOf(in.Agent),
			Instructions: in.Agent.Instructions,
		})

	b.Append(Section{
		Title: "time_context", Description: timeContextDescription,
		Kind: KindData, Source: SourceRuntime, Trust: TrustObserved,
		Content: a.timeContext(in),
	})
	b.Append(Section{
		Title: "role_directive",
		Kind:  KindPolicy, Source: SourceAgent, Trust: TrustTrusted,
		Content: DirectiveFor(in.Agent.Orchestrator),
	})
	b.Append(Section{
		Title: "activation_modes",
		Kind:  KindPolicy, Source: SourceWorkspace, Trust: TrustTrusted,
		Content: ActivationModes,
	})
	b.Append(Section{
		Title: "environment", Description: environmentDescription,
		Kind: KindData, Source: SourceRuntime, Trust: TrustObserved,
		Content: environment(),
	})

	inventory := a.inventory(ctx, in)
	b.Append(Section{
		Title: "workspace",
		Kind:  KindData, Source: SourceWorkspace, Trust: TrustObserved,
		Content: inventory.node(),
	})

	b.Append(Section{
		Title: "memories", Description: memoriesDescription,
		Kind: KindMemory, Source: SourceAgent, Trust: TrustObserved,
		Content: a.memories(ctx, in).node(),
	})

	for _, ext := range in.ExternalContent {
		b.Append(Section{
			Title: "external_content",
			Kind:  KindEvidence, Source: SourceExternal, Trust: TrustUnverified,
			Content: Object{
				Attr("title", ext.Title),
				Attr("origin", ext.Origin),
				Body(ext.Body),
			},
		})
	}

	return b.Build(a.vars(in))
}

// vars is the allowlist handed to the template engine.
//
// Built field by field, and that is the point (ADR-0014). Handing the engine a
// configuration object would make `{{ config.security.secret }}` a working
// expression, and the only thing standing between that expression and a leak
// would be the opt-in gate.
func (a *Assembler) vars(in AssembleInput) map[string]any {
	return map[string]any{
		"product": map[string]any{
			"name":    build.Name,
			"display": build.DisplayName,
		},
		"agent": map[string]any{
			"id":   in.Agent.ID,
			"name": in.Agent.Name,
			"role": roleOf(in.Agent),
		},
		"workspace": map[string]any{"id": in.Workspace},
	}
}

// timeContext omits every field it cannot prove.
//
// The master prompt tells the agent it has no internal clock and that its
// context says what time it is. A field invented to fill a gap would make that
// sentence a lie in the one place where the agent has no way to check.
func (a *Assembler) timeContext(in AssembleInput) Object {
	now := a.now()
	out := Object{
		Field{Key: "now", Value: Text(now.Format(time.RFC3339))},
		Field{Key: "local", Value: Text(now.Format("15:04, Monday"))},
		Field{Key: "timezone", Value: Text(a.zone())},
	}
	if !in.SessionStartedAt.IsZero() {
		out = append(out,
			Field{Key: "session_started_at", Value: Text(in.SessionStartedAt.Format(time.RFC3339))},
			Field{Key: "minutes_since_session_start", Value: Text(itoa(minutesSince(now, in.SessionStartedAt)))},
		)
	}
	if !in.LastUserMessageAt.IsZero() {
		out = append(out,
			Field{Key: "minutes_since_last_user_message", Value: Text(itoa(minutesSince(now, in.LastUserMessageAt)))},
		)
	}
	return out
}

func (a *Assembler) inventory(ctx context.Context, in AssembleInput) Inventory {
	if a.reader == nil {
		return Inventory{Workspace: WorkspaceRef{ID: in.Workspace}}
	}
	inv, err := a.reader.Inventory(ctx, in.Workspace)
	if err != nil {
		a.log.Warn("the workspace inventory could not be read; the prompt was assembled without it",
			"workspace", in.Workspace, "err", err)
		return Inventory{Workspace: WorkspaceRef{ID: in.Workspace}}
	}
	if inv.Workspace.ID == "" {
		inv.Workspace.ID = in.Workspace
	}
	return inv
}

func (a *Assembler) memories(ctx context.Context, in AssembleInput) MemoryCounts {
	if a.reader == nil {
		return MemoryCounts{}
	}
	counts, err := a.reader.MemoryCounts(ctx, in.Agent.ID)
	if err != nil {
		a.log.Warn("the memory counts could not be read; the prompt was assembled without them",
			"agent", in.Agent.ID, "err", err)
		return MemoryCounts{}
	}
	return counts
}

func (a *Assembler) now() time.Time {
	if a.clock == nil {
		return time.Time{}
	}
	return a.clock.Now()
}

func (a *Assembler) zone() string {
	if a.clock == nil || a.clock.Location() == nil {
		return "UTC"
	}
	return a.clock.Location().String()
}

// environment is what the original reports about the machine, in Go's terms.
func environment() Object {
	return Object{
		Field{Key: "platform", Value: Text(runtime.GOOS)},
		Field{Key: "arch", Value: Text(runtime.GOARCH)},
		Field{Key: "runtime", Value: Text(runtime.Version())},
		Field{Key: "version", Value: Text(build.Version)},
	}
}

// roleOf falls back the way the original does: role, then description, then a
// generic label. An agent with no role is still an agent.
func roleOf(a AgentRef) string {
	if a.Role != "" {
		return a.Role
	}
	return "Assistant"
}

func minutesSince(now, then time.Time) int {
	d := now.Sub(then)
	if d < 0 {
		return 0
	}
	return int(d.Minutes())
}

func itoa(n int) string { return strconv.Itoa(n) }
