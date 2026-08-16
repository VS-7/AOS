package subconscious

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/safe"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/prompt"
)

// Prompt is the subconscious's own instructions, ported from the original.
//
//go:embed subconscious.md
var Prompt string

// recallCandidates is how many recent memories of a category are compared
// against a draft before it is stored. It is bounded because this runs on every
// draft of every turn, and newest-first because a near-duplicate an observer
// formed is far likelier to be recent than ancient.
const recallCandidates = 50

// Draft is a memory the observer proposes.
type Draft struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Category    string   `json:"category"`
	Confidence  float64  `json:"confidence"`
	Tags        []string `json:"tags,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`

	// SupersedeReason is set when the draft contradicts something already
	// known. It is what turns a near-duplicate into a lineage rather than a
	// second copy.
	SupersedeReason string `json:"supersedeReason,omitempty"`
}

// Reply is what the observer's model returns.
type Reply struct {
	Guidance string  `json:"guidance"`
	Drafts   []Draft `json:"drafts"`
}

// Input is one session, as the observer sees it.
type Input struct {
	AgentID   string
	AgentName string
	SessionID string
	Workspace string

	Messages  []agentloop.Message
	Events    []event.Record
	Inventory prompt.Inventory
}

// Output is what one observation produced.
type Output struct {
	// Guidance is surgical advice for the main agent's next turn. It is
	// currently recorded rather than injected: wiring it into the next prompt
	// is a decision about the agent's context budget, and the note records it
	// as pending rather than doing it silently.
	Guidance string

	Stored     []memory.Memory
	Duplicates int
	Skipped    bool
}

// Models resolves the slot this observer runs on.
//
// It is a port rather than a field because the configuration is read afresh
// each turn: a key added while the daemon runs should be used on the next
// observation, not on the next restart.
type Models interface {
	Subconscious(ctx context.Context, agentID string) (agentloop.LLMProvider, agentloop.ModelRef, error)
}

// Memories is the slice of the memory aggregate the observer needs.
type Memories interface {
	Recall(ctx context.Context, in memory.RecallInput) (memory.RecallOutput, error)
	Store(ctx context.Context, in memory.StoreInput) (memory.StoreOutput, error)
}

// Deps is what the observer is built from.
type Deps struct {
	Models     Models
	Memories   Memories
	Signatures Signatures
	Config     Config
	Clock      func() time.Time
	Log        *slog.Logger
}

// Observer runs the background cognitive pass.
type Observer struct {
	models Models
	mems   Memories
	sigs   Signatures
	cfg    Config
	clock  func() time.Time
	log    *slog.Logger

	// last records when each session was last observed, for the coalescing.
	mu   sync.Mutex
	last map[string]time.Time
}

// New builds an observer.
func New(d Deps) *Observer {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	clock := d.Clock
	if clock == nil {
		clock = time.Now //nolint:forbidigo // the fallback when no clock is injected
	}
	sigs := d.Signatures
	if sigs == nil {
		sigs = NewMemorySignatures(clock)
	}
	return &Observer{
		models: d.Models, mems: d.Memories, sigs: sigs,
		cfg: d.Config.withDefaults(), clock: clock, log: log,
		last: map[string]time.Time{},
	}
}

// Schedule fires an observation without waiting for it.
//
// The context is detached and given the observer's own timeout, so a slow or
// failing observation never delays the answer the user is waiting for, and the
// end of the turn never cancels the memory forming from it.
func (o *Observer) Schedule(ctx context.Context, in Input) {
	if !o.due(in.SessionID) {
		return
	}
	detached := context.WithoutCancel(ctx)
	safe.Go(detached, "subconscious.observe", func(ctx context.Context) error {
		runCtx, cancel := context.WithTimeout(ctx, o.cfg.Timeout)
		defer cancel()
		if _, err := o.Observe(runCtx, in); err != nil {
			// Losing an observation must never surface anywhere near the turn.
			o.log.Warn("the background observation failed",
				"agent", in.AgentID, "session", in.SessionID, "err", err)
		}
		return nil
	})
}

// due reports whether this session may be observed now, and records the attempt.
//
// Coalescing is ours: a conversation with ten short turns in a minute should
// produce one observation, not ten. The original has no floor at all.
func (o *Observer) due(sessionID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := o.clock()
	if last, ok := o.last[sessionID]; ok && now.Sub(last) < o.cfg.MinInterval {
		return false
	}
	o.last[sessionID] = now
	return true
}

// Observe runs one pass: read the session, ask the small model, persist what is
// durable.
func (o *Observer) Observe(ctx context.Context, in Input) (Output, error) {
	if o.models == nil || o.mems == nil {
		return Output{Skipped: true}, nil
	}
	agentID := strings.ToLower(strings.TrimSpace(in.AgentID))
	if agentID == "" {
		return Output{Skipped: true}, nil
	}

	provider, ref, err := o.models.Subconscious(ctx, agentID)
	if err != nil {
		return Output{}, err
	}

	reply, err := o.ask(ctx, provider, ref, in)
	if err != nil {
		return Output{}, err
	}

	// The observation is attributed to the agent it supports, because the
	// memory it forms is that agent's. The subconscious has no memories of
	// its own — it is a layer, not a second agent.
	storeCtx := identity.With(ctx, identity.Identity{
		AgentID:     agentID,
		WorkspaceID: in.Workspace,
		RequestID:   in.SessionID,
	})

	out := Output{Guidance: strings.TrimSpace(reply.Guidance)}
	drafts := reply.Drafts
	if len(drafts) > o.cfg.MaxDrafts {
		o.log.Info("the observer proposed more memories than the cap allows",
			"agent", agentID, "proposed", len(drafts), "cap", o.cfg.MaxDrafts)
		drafts = drafts[:o.cfg.MaxDrafts]
	}
	for _, d := range drafts {
		stored, duplicate, err := o.persist(storeCtx, agentID, d)
		if err != nil {
			o.log.Warn("a memory draft could not be stored",
				"agent", agentID, "title", d.Title, "err", err)
			continue
		}
		if duplicate {
			out.Duplicates++
			continue
		}
		if stored != nil {
			out.Stored = append(out.Stored, *stored)
		}
	}
	return out, nil
}

// ask runs the small model and parses its answer.
func (o *Observer) ask(ctx context.Context, provider agentloop.LLMProvider, ref agentloop.ModelRef, in Input) (Reply, error) {
	res, err := provider.Generate(ctx, agentloop.Request{
		Model:        ref.Model,
		Instructions: o.system(in),
		Messages:     []agentloop.Message{{Role: agentloop.RoleUser, Text: o.context(in)}},
		Reasoning:    ref.Reasoning,
	})
	if err != nil {
		return Reply{}, err
	}
	return parseReply(res.Message.Text)
}

// system builds the observer's instructions, with the derived identity.
//
// The subconscious is not a separate agent with its own file: it is derived
// from the one it supports, exactly as in the original.
func (o *Observer) system(in Input) string {
	name := in.AgentName
	if name == "" {
		name = in.AgentID
	}
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(Prompt, "{{ product.display }}", build.DisplayName))
	b.WriteString("\n\n## Who You Support\n\n")
	fmt.Fprintf(&b, "- id: %s-subconscious\n", in.AgentID)
	fmt.Fprintf(&b, "- name: %s Subconscious\n", name)
	fmt.Fprintf(&b, "- role: Background cognitive layer for %s\n", name)
	return b.String()
}

// context formats the session for the observer, within the character cap.
//
// The order is deliberate: the inventory first because it is the cheapest to
// truncate, the messages last because they are what the observation is about.
// A cap that cut the conversation instead of the catalogue would produce an
// observer that knows what exists and not what happened.
func (o *Observer) context(in Input) string {
	var b strings.Builder

	b.WriteString("# Workspace\n\n")
	writeList(&b, "collections", in.Inventory.Collections)
	writeList(&b, "agents", in.Inventory.Agents)
	writeList(&b, "skills", in.Inventory.Skills)
	writeList(&b, "instructions", in.Inventory.Instructions)
	writeList(&b, "goals", in.Inventory.Goals)
	writeList(&b, "routines", in.Inventory.Routines)
	writeList(&b, "projects", in.Inventory.Projects)

	events := in.Events
	if len(events) > MaxRecentEvents {
		events = events[len(events)-MaxRecentEvents:]
	}
	if len(events) > 0 {
		b.WriteString("\n# Recent session events\n\n")
		for _, e := range events {
			fmt.Fprintf(&b, "- %s", e.Type)
			if e.Hook != "" {
				fmt.Fprintf(&b, " (hook %s)", e.Hook)
			}
			// The decision, not the payload: what the observer needs is that a
			// call was refused, not the arguments it was refused with.
			if d := e.Outcome.Decision; d != "" {
				fmt.Fprintf(&b, " → %s", d)
			}
			if d := e.Outcome.PermissionDecision; d != "" {
				fmt.Fprintf(&b, " → %s", d)
			}
			b.WriteString("\n")
		}
	}

	messages := in.Messages
	if len(messages) > MaxRecentMessages {
		messages = messages[len(messages)-MaxRecentMessages:]
	}
	b.WriteString("\n# Recent messages\n\n")
	for _, m := range messages {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", m.Role, describe(m))
	}

	return truncate(b.String(), InputCharLimit)
}

// persist applies the same discipline the master prompt demands of the main
// agent: recall before store, link or supersede rather than duplicate.
func (o *Observer) persist(ctx context.Context, agentID string, d Draft) (*memory.Memory, bool, error) {
	sig := Signature(d)
	if seen, err := o.sigs.Seen(ctx, agentID, sig); err == nil && seen {
		return nil, true, nil
	}

	category := memory.Category(strings.ToLower(strings.TrimSpace(d.Category)))
	if !category.Valid() {
		// A category the model invented is not a reason to lose the memory. It
		// becomes an observation, which is the honest label for "something the
		// system noticed and could not classify".
		o.log.Info("the observer proposed a category that is not one",
			"agent", agentID, "category", d.Category)
		category = memory.CatObservation
	}

	confidence := d.Confidence
	if confidence <= 0 || confidence > 1 {
		// A model that omits the number is guessing; recording that as certainty
		// is how future selves get misled.
		confidence = 0.6
	}

	in := memory.StoreInput{
		Title:       strings.TrimSpace(d.Title),
		Description: strings.TrimSpace(d.Description),
		Content:     d.Content,
		Category:    category,
		Tags:        d.Tags,
		Scopes:      d.Scopes,
		Confidence:  &confidence,
	}
	if in.Title == "" {
		return nil, false, fmt.Errorf("subconscious: a draft with no title cannot be stored")
	}
	if in.Description == "" {
		in.Description = in.Title
	}

	// Recall before storing. A near-duplicate becomes a link; one that
	// contradicts becomes a supersede. Duplicates dilute the graph.
	//
	// The title is deliberately not used as the query. Recall matches when
	// every word is present, so "denial patterns need a spanning wildcard
	// everywhere" would not find "denial patterns need a spanning wildcard" —
	// which is precisely the near-duplicate this is looking for. The category
	// narrows instead, and the overlap below decides.
	similar, err := o.mems.Recall(ctx, memory.RecallInput{
		Agent: agentID, Category: category, Limit: recallCandidates,
		OrderBy: "createdAt", Desc: true,
	})
	if err == nil {
		if hit, ok := bestMatch(similar.Memories, in.Title); ok {
			if reason := strings.TrimSpace(d.SupersedeReason); reason != "" {
				in.Supersedes = append(in.Supersedes, memory.Super{ID: hit.ID, Reason: reason})
			} else {
				in.Links = append(in.Links, hit.ID)
			}
		}
	}

	stored, err := o.mems.Store(ctx, in)
	if err != nil {
		return nil, false, err
	}
	if err := o.sigs.Mark(ctx, agentID, sig, o.cfg.SignatureTTL); err != nil {
		o.log.Warn("a memory was stored and its signature was not recorded, so it may be formed again",
			"agent", agentID, "memory", stored.Memory.ID, "err", err)
	}
	return &stored.Memory, false, nil
}

// bestMatch finds the closest existing memory to a title.
//
// The comparison is word overlap, not similarity scoring: recall has already
// narrowed the set with the shared tokeniser, and what is wanted here is only
// "is one of these the same thing".
func bestMatch(candidates []memory.Memory, title string) (memory.Memory, bool) {
	want := words(title)
	if len(want) == 0 {
		return memory.Memory{}, false
	}
	var best memory.Memory
	bestScore := 0.0
	for _, m := range candidates {
		score := overlap(want, words(m.Title))
		if score > bestScore {
			best, bestScore = m, score
		}
	}
	// Half the words in common is a low bar on purpose: linking two memories
	// that turn out to be unrelated costs an edge, and failing to link two that
	// are the same costs a duplicate that dilutes every future recall.
	if bestScore < 0.5 {
		return memory.Memory{}, false
	}
	return best, true
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?\"'()[]")
		if len(w) > 2 {
			out[w] = true
		}
	}
	return out
}

func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	var shared int
	for w := range a {
		if b[w] {
			shared++
		}
	}
	return float64(shared) / float64(len(a))
}

// parseReply reads the model's answer.
//
// A model asked for JSON will sometimes wrap it in a fence or preface it with a
// sentence. Both are recovered from rather than treated as a failure: losing an
// observation because a small model was chatty is the wrong trade.
func parseReply(raw string) (Reply, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return Reply{}, nil
	}
	if fenced := stripFence(text); fenced != "" {
		text = fenced
	}
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return Reply{}, fmt.Errorf("subconscious: the observer returned no JSON object")
	}

	var reply Reply
	if err := json.Unmarshal([]byte(text[start:end+1]), &reply); err != nil {
		return Reply{}, fmt.Errorf("subconscious: could not read the observer's answer: %w", err)
	}
	return reply, nil
}

func stripFence(text string) string {
	const fence = "```"
	start := strings.Index(text, fence)
	if start < 0 {
		return ""
	}
	rest := text[start+len(fence):]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	}
	end := strings.Index(rest, fence)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// describe renders one message for the observer: what was said, and what was
// done. Tool calls are named rather than expanded — the observer needs to know
// that a file was read, not what was in it.
func describe(m agentloop.Message) string {
	var parts []string
	if text := strings.TrimSpace(m.Text); text != "" {
		parts = append(parts, text)
	}
	for _, call := range m.ToolCalls {
		parts = append(parts, "[called "+call.Name+"]")
	}
	if m.Role == agentloop.RoleTool {
		// The result itself is not shown. A tool result can be a whole file,
		// and this observer has twelve thousand characters for the entire
		// session; what it needs is that the call happened.
		parts = append(parts, "["+m.Name+" returned]")
	}
	if len(parts) == 0 {
		return "(nothing)"
	}
	return strings.Join(parts, "\n")
}

func writeList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, strings.Join(items, ", "))
}

// truncate cuts at a rune boundary and says that it did, so the observer knows
// it is reading a fragment rather than the whole session.
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	const notice = "\n\n[truncated: the session is longer than this observer reads]"
	cut := limit - len(notice)
	if cut < 0 {
		cut = 0
	}
	runes := []rune(s)
	if cut > len(runes) {
		cut = len(runes)
	}
	return string(runes[:cut]) + notice
}
