package subconscious

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/prompt"
)

// scripted is a provider that answers with whatever the test wrote, and records
// what it was asked. Everything in this suite runs on it: the point is what the
// observer does with an answer, not what a model would produce.
type scripted struct {
	mu       sync.Mutex
	replies  []string
	requests []agentloop.Request
	failWith error
	delay    time.Duration
	n        int
}

func (s *scripted) Name() string { return "scripted" }

func (s *scripted) Generate(ctx context.Context, req agentloop.Request) (agentloop.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	i := s.n
	s.n++
	s.mu.Unlock()

	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return agentloop.Response{}, ctx.Err()
		case <-time.After(s.delay):
		}
	}
	if s.failWith != nil {
		return agentloop.Response{}, s.failWith
	}
	if i >= len(s.replies) {
		return agentloop.Response{Message: agentloop.Message{Text: `{"guidance":"","drafts":[]}`}}, nil
	}
	return agentloop.Response{
		Message: agentloop.Message{Role: agentloop.RoleAssistant, Text: s.replies[i]},
	}, nil
}

func (s *scripted) Stream(context.Context, agentloop.Request) (agentloop.Stream, error) {
	return nil, errors.New("this provider does not stream")
}

func (s *scripted) asked() []agentloop.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agentloop.Request(nil), s.requests...)
}

type models struct {
	provider agentloop.LLMProvider
	ref      agentloop.ModelRef
	failWith error
}

func (m models) Subconscious(context.Context, string) (agentloop.LLMProvider, agentloop.ModelRef, error) {
	if m.failWith != nil {
		return nil, agentloop.ModelRef{}, m.failWith
	}
	return m.provider, m.ref, nil
}

type countingIDs struct {
	mu sync.Mutex
	n  int
}

func (g *countingIDs) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return "m" + strconv.Itoa(g.n)
}

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

type harness struct {
	obs      *Observer
	provider *scripted
	mems     *memory.Service
	repo     *fakes.Repo[memory.Memory]
	sigs     *MemorySignatures
	clock    *clockx.Stepping
}

func newHarness(t *testing.T, replies ...string) *harness {
	t.Helper()
	clock := &clockx.Stepping{At: start, Step: time.Second}
	repo := fakes.NewRepo[memory.Memory]("memories").WithKeyFunc(func(v *memory.Memory) collections.Key {
		return collections.Key{"agent": v.Agent, "id": v.ID}
	})
	mems := memory.NewService(memory.Deps{Repo: repo, Clock: clock, IDs: &countingIDs{}})
	provider := &scripted{replies: replies}
	sigs := NewMemorySignatures(clock.Now)

	return &harness{
		obs: New(Deps{
			Models:     models{provider: provider, ref: agentloop.ModelRef{Provider: "fake", Model: "small"}},
			Memories:   mems,
			Signatures: sigs,
			Clock:      clock.Now,
		}),
		provider: provider, mems: mems, repo: repo, sigs: sigs, clock: clock,
	}
}

func ctx() context.Context { return context.Background() }

func sampleInput() Input {
	return Input{
		AgentID: "atlas", AgentName: "Atlas", SessionID: "s-1", Workspace: "alpha",
		Messages: []agentloop.Message{
			{Role: agentloop.RoleUser, Text: "the denial pattern never matches"},
			{Role: agentloop.RoleAssistant, Text: "found it", ToolCalls: []agentloop.ToolCall{{Name: "Read"}}},
			{Role: agentloop.RoleTool, Name: "Read"},
		},
		Inventory: prompt.Inventory{Collections: []string{"notes"}, Agents: []string{"atlas"}},
	}
}

const oneDraft = `{
  "guidance": "The sandbox matches command lines with a path glob.",
  "drafts": [{
    "title": "Denial patterns need a spanning wildcard",
    "description": "A path glob stops at the separator, so a command with a path in it slips past the denial list.",
    "content": "Use matchLine, not doublestar.Match, for a command line.",
    "category": "learning",
    "confidence": 0.95,
    "tags": ["sandbox"],
    "scopes": ["internal/runtime/sandbox/**"]
  }]
}`

// TestAMemoryFormsWithoutTheMainAgentAskingForIt. This is the whole point: the
// prompt tells the agent to reflect before answering, and under cost pressure
// that is the step that gets skipped. Here the system remembers regardless.
func TestAMemoryFormsWithoutTheMainAgentAskingForIt(t *testing.T) {
	h := newHarness(t, oneDraft)

	out, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Stored) != 1 {
		t.Fatalf("stored %d memories", len(out.Stored))
	}
	stored := out.Stored[0]
	if stored.Agent != "atlas" {
		t.Fatalf("the memory belongs to %q", stored.Agent)
	}
	if stored.Category != memory.CatLearning || stored.Confidence != 0.95 {
		t.Fatalf("got %+v", stored)
	}
	if out.Guidance == "" {
		t.Fatal("the observer returned no guidance")
	}
}

// TestTheSameDraftTwiceIsOneMemory. Without deduplication an observer running
// every turn recreates the same memory forever.
func TestTheSameDraftTwiceIsOneMemory(t *testing.T) {
	h := newHarness(t, oneDraft, oneDraft)

	first, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Stored) != 1 || len(second.Stored) != 0 {
		t.Fatalf("stored %d then %d", len(first.Stored), len(second.Stored))
	}
	if second.Duplicates != 1 {
		t.Fatalf("the repeat was not reported as a duplicate: %+v", second)
	}
}

// TestCosmeticRewordingDeduplicatesAndAGenuineChangeDoesNot.
func TestCosmeticRewordingDeduplicatesAndAGenuineChangeDoesNot(t *testing.T) {
	base := Draft{
		Category: "learning",
		Title:    "Denial patterns need a spanning wildcard",
		Content:  "Use matchLine, not doublestar.Match.",
	}
	reworded := base
	reworded.Title = "  DENIAL   Patterns Need a Spanning   Wildcard  "
	reworded.Content = "USE MATCHLINE, NOT DOUBLESTAR.MATCH."

	if Signature(base) != Signature(reworded) {
		t.Fatal("capitalisation and whitespace produced a second memory")
	}

	changed := base
	changed.Content = "Use doublestar.Match; the glob is fine."
	if Signature(base) == Signature(changed) {
		t.Fatal("a genuine change deduplicated away")
	}

	other := base
	other.Category = "decision"
	if Signature(base) == Signature(other) {
		t.Fatal("the same text in a different category deduplicated")
	}
}

// TestASignatureExpires, so a lesson that genuinely recurs months later can be
// recorded again.
func TestASignatureExpires(t *testing.T) {
	clock := &clockx.Stepping{At: start, Step: 0}
	sigs := NewMemorySignatures(clock.Now)

	if err := sigs.Mark(ctx(), "atlas", "sig", time.Hour); err != nil {
		t.Fatal(err)
	}
	seen, err := sigs.Seen(ctx(), "atlas", "sig")
	if err != nil || !seen {
		t.Fatalf("seen = %v, %v", seen, err)
	}
	if other, _ := sigs.Seen(ctx(), "nova", "sig"); other {
		t.Fatal("one agent's signature suppressed another's memory")
	}

	clock.Set(start.Add(2 * time.Hour))
	seen, err = sigs.Seen(ctx(), "atlas", "sig")
	if err != nil || seen {
		t.Fatalf("an expired signature still suppresses: %v, %v", seen, err)
	}
	if sigs.Len() != 0 {
		t.Fatalf("the expired signature was not dropped: %d left", sigs.Len())
	}
}

// TestANearDuplicateBecomesALinkAndAContradictionBecomesASupersede. It is the
// same discipline the master prompt demands of the main agent, applied to the
// observer: recall before store.
func TestANearDuplicateBecomesALinkAndAContradictionBecomesASupersede(t *testing.T) {
	existing := `{"drafts":[{
		"title":"Denial patterns need a spanning wildcard",
		"description":"first","content":"first version","category":"learning","confidence":0.9
	}]}`
	similar := `{"drafts":[{
		"title":"Denial patterns need a spanning wildcard everywhere",
		"description":"second","content":"a second, related note","category":"learning","confidence":0.9
	}]}`
	contradicting := `{"drafts":[{
		"title":"Denial patterns need a spanning wildcard everywhere too",
		"description":"third","content":"the opposite","category":"learning","confidence":0.9,
		"supersedeReason":"the earlier note was measured before the fix and no longer holds"
	}]}`

	h := newHarness(t, existing, similar, contradicting)

	first, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	original := first.Stored[0]

	second, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Stored) != 1 {
		t.Fatalf("stored %d", len(second.Stored))
	}
	if len(second.Stored[0].Links) != 1 || second.Stored[0].Links[0] != original.ID {
		t.Fatalf("a near-duplicate did not link: %+v", second.Stored[0].Links)
	}

	third, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Stored[0].Supersedes) != 1 {
		t.Fatalf("a contradiction did not supersede: %+v", third.Stored[0])
	}

	// The superseded memory is deprecated rather than deleted.
	replaced, err := h.mems.Reflect(asAgent("atlas"), memory.ReflectInput{
		Memory: third.Stored[0].Supersedes[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Status != memory.StatusDeprecated {
		t.Fatalf("the superseded memory is %s", replaced.Status)
	}
}

// TestTheWindowsAreEnforced: fifty messages become eight, a hundred events
// twelve, and a huge session is cut at twelve thousand characters.
func TestTheWindowsAreEnforced(t *testing.T) {
	h := newHarness(t, `{"drafts":[]}`)

	in := sampleInput()
	in.Messages = nil
	for i := range 50 {
		in.Messages = append(in.Messages, agentloop.Message{
			Role: agentloop.RoleUser, Text: "message " + strconv.Itoa(i),
		})
	}
	for i := range 100 {
		in.Events = append(in.Events, event.Record{Type: event.PreToolUse, Hook: "h" + strconv.Itoa(i)})
	}

	if _, err := h.obs.Observe(ctx(), in); err != nil {
		t.Fatal(err)
	}
	asked := h.provider.asked()
	body := asked[0].Messages[0].Text

	if strings.Contains(body, "message 41") {
		t.Fatal("more than the last eight messages were shown")
	}
	if !strings.Contains(body, "message 49") {
		t.Fatal("the most recent message was cut")
	}
	if strings.Contains(body, "hook h87") {
		t.Fatal("more than the last twelve events were shown")
	}
	if len(body) > InputCharLimit {
		t.Fatalf("the context is %d characters, over the %d cap", len(body), InputCharLimit)
	}
}

// TestAHugeSessionIsCutAndSaysThatItWas, so the observer knows it is reading a
// fragment rather than the whole thing.
func TestAHugeSessionIsCutAndSaysThatItWas(t *testing.T) {
	h := newHarness(t, `{"drafts":[]}`)

	in := sampleInput()
	in.Messages = []agentloop.Message{{
		Role: agentloop.RoleUser, Text: strings.Repeat("x", 40_000),
	}}
	if _, err := h.obs.Observe(ctx(), in); err != nil {
		t.Fatal(err)
	}
	body := h.provider.asked()[0].Messages[0].Text
	if len(body) > InputCharLimit {
		t.Fatalf("the context is %d characters", len(body))
	}
	if !strings.Contains(body, "truncated") {
		t.Fatal("the context was cut silently")
	}
}

// TestTheIdentityIsDerivedFromTheAgentItSupports. The subconscious is a layer,
// not a second agent with a file of its own.
func TestTheIdentityIsDerivedFromTheAgentItSupports(t *testing.T) {
	h := newHarness(t, `{"drafts":[]}`)
	if _, err := h.obs.Observe(ctx(), sampleInput()); err != nil {
		t.Fatal(err)
	}
	system := h.provider.asked()[0].Instructions

	for _, want := range []string{
		"atlas-subconscious", "Atlas Subconscious", "Background cognitive layer for Atlas",
		"Do not answer the user", "Do not roleplay as the main agent",
	} {
		if !strings.Contains(system, want) {
			t.Fatalf("the observer's instructions are missing %q", want)
		}
	}
	if strings.Contains(system, "{{") {
		t.Fatal("the instructions still carry an unrendered placeholder")
	}
}

// TestCoalescingCollapsesABurstOfShortTurns. Ten turns in a minute should
// produce one observation. The original has no floor at all.
func TestCoalescingCollapsesABurstOfShortTurns(t *testing.T) {
	clock := &clockx.Stepping{At: start, Step: time.Second}
	obs := New(Deps{
		Models:   models{provider: &scripted{}},
		Memories: nil, // not reached: this is about the gate, not the pass
		Clock:    clock.Now,
	})

	var fired int
	for range 10 {
		if obs.due("s-1") {
			fired++
		}
	}
	if fired != 1 {
		t.Fatalf("%d observations for ten turns in ten seconds", fired)
	}

	// A different session is not gated by the first one's floor.
	if !obs.due("s-2") {
		t.Fatal("one session's floor suppressed another's observation")
	}

	clock.Set(start.Add(2 * time.Minute))
	if !obs.due("s-1") {
		t.Fatal("the floor never lifted")
	}
}

// TestAnObservationNeverBlocksTheTurn. Schedule returns while the model is
// still thinking, which is what keeps a slow observer out of the answer's path.
func TestAnObservationNeverBlocksTheTurn(t *testing.T) {
	clock := &clockx.Stepping{At: start, Step: time.Second}
	repo := fakes.NewRepo[memory.Memory]("memories").WithKeyFunc(func(v *memory.Memory) collections.Key {
		return collections.Key{"agent": v.Agent, "id": v.ID}
	})
	mems := memory.NewService(memory.Deps{Repo: repo, Clock: clock, IDs: &countingIDs{}})
	provider := &scripted{replies: []string{oneDraft}, delay: 200 * time.Millisecond}

	obs := New(Deps{
		Models:   models{provider: provider, ref: agentloop.ModelRef{Model: "small"}},
		Memories: mems,
		Clock:    clock.Now,
	})

	began := time.Now()
	obs.Schedule(ctx(), sampleInput())
	if elapsed := time.Since(began); elapsed > 50*time.Millisecond {
		t.Fatalf("Schedule blocked for %s", elapsed)
	}

	// The observation still lands, on its own time.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.Len() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the detached observation never stored anything")
}

// TestAnObserverThatFailsCostsNothingButTheObservation.
func TestAnObserverThatFailsCostsNothingButTheObservation(t *testing.T) {
	h := newHarness(t)
	h.provider.failWith = errors.New("the small model is down")

	if _, err := h.obs.Observe(ctx(), sampleInput()); err == nil {
		t.Fatal("a failed observation reported success")
	}
	// Scheduling swallows it: the turn must not learn about this.
	h.clock.Set(start.Add(time.Hour))
	h.obs.Schedule(ctx(), sampleInput())
	time.Sleep(20 * time.Millisecond)
	if h.repo.Len() != 0 {
		t.Fatal("a failed observation stored something")
	}
}

// TestAModelWithNoSlotConfiguredIsReported.
func TestAModelWithNoSlotConfiguredIsReported(t *testing.T) {
	obs := New(Deps{
		Models:   models{failWith: errors.New("no subconscious model is configured")},
		Memories: newHarness(t).mems,
	})
	if _, err := obs.Observe(ctx(), sampleInput()); err == nil {
		t.Fatal("an unconfigured slot observed anyway")
	}
}

// TestAnObserverWithNothingWiredSkipsRatherThanPanics. It is the shape a run
// with no memory service gets, and it must be inert rather than fatal.
func TestAnObserverWithNothingWiredSkipsRatherThanPanics(t *testing.T) {
	obs := New(Deps{})
	out, err := obs.Observe(ctx(), sampleInput())
	if err != nil || !out.Skipped {
		t.Fatalf("out = %+v, err = %v", out, err)
	}

	h := newHarness(t, oneDraft)
	anonymous := sampleInput()
	anonymous.AgentID = "  "
	out, err = h.obs.Observe(ctx(), anonymous)
	if err != nil || !out.Skipped {
		t.Fatalf("an observation with no agent produced %+v (%v)", out, err)
	}
}

// TestTheDraftCapHolds. A model asked for durable learnings will find some if
// it looks hard enough.
func TestTheDraftCapHolds(t *testing.T) {
	var drafts []string
	for i := range 6 {
		n := strconv.Itoa(i)
		drafts = append(drafts, `{"title":"Lesson `+n+`","description":"d`+n+
			`","content":"c`+n+`","category":"learning","confidence":0.9}`)
	}
	reply := `{"drafts":[` + strings.Join(drafts, ",") + `]}`

	h := newHarness(t, reply)
	out, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Stored) != DefaultMaxDrafts {
		t.Fatalf("stored %d, cap is %d", len(out.Stored), DefaultMaxDrafts)
	}
}

// TestAChattyModelIsStillRead. Losing an observation because a small model
// wrapped its JSON in a fence is the wrong trade.
func TestAChattyModelIsStillRead(t *testing.T) {
	cases := map[string]string{
		"bare":            `{"guidance":"g","drafts":[]}`,
		"fenced":          "```json\n{\"guidance\":\"g\",\"drafts\":[]}\n```",
		"prefaced":        "Sure! Here is the object:\n{\"guidance\":\"g\",\"drafts\":[]}",
		"fenced no label": "```\n{\"guidance\":\"g\",\"drafts\":[]}\n```",
	}
	for name, raw := range cases {
		got, err := parseReply(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Guidance != "g" {
			t.Fatalf("%s: guidance = %q", name, got.Guidance)
		}
	}

	if _, err := parseReply("I could not do that."); err == nil {
		t.Fatal("an answer with no object at all parsed")
	}
	if _, err := parseReply("{not json}"); err == nil {
		t.Fatal("an object that is not JSON parsed")
	}
	empty, err := parseReply("   ")
	if err != nil || len(empty.Drafts) != 0 {
		t.Fatalf("an empty answer produced %+v (%v)", empty, err)
	}
}

// TestADraftWithAnInventedCategoryBecomesAnObservation, rather than being lost.
func TestADraftWithAnInventedCategoryBecomesAnObservation(t *testing.T) {
	reply := `{"drafts":[{"title":"Something happened","description":"d","content":"c",
		"category":"epiphany","confidence":0.9}]}`
	h := newHarness(t, reply)

	out, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Stored) != 1 || out.Stored[0].Category != memory.CatObservation {
		t.Fatalf("stored %+v", out.Stored)
	}
}

// TestADraftWithNoConfidenceIsNotRecordedAsCertain. A model that omits the
// number is guessing, and recording that as certainty is how future selves get
// misled.
func TestADraftWithNoConfidenceIsNotRecordedAsCertain(t *testing.T) {
	reply := `{"drafts":[{"title":"Unsure about this","description":"d","content":"c",
		"category":"observation"}]}`
	h := newHarness(t, reply)

	out, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Stored[0].Confidence != 0.6 {
		t.Fatalf("confidence = %v", out.Stored[0].Confidence)
	}
}

// TestADraftWithNoTitleIsDroppedAndTheRestSurvive.
func TestADraftWithNoTitleIsDroppedAndTheRestSurvive(t *testing.T) {
	reply := `{"drafts":[
		{"title":"  ","description":"d","content":"c","category":"fact","confidence":0.9},
		{"title":"A real one","description":"d","content":"c","category":"fact","confidence":0.9}
	]}`
	h := newHarness(t, reply)

	out, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Stored) != 1 || out.Stored[0].Title != "A real one" {
		t.Fatalf("stored %+v", out.Stored)
	}
}

// TestADraftWithNoDescriptionFallsBackToItsTitle, because the description is
// what a later recall searches on and an empty one makes the memory unfindable.
func TestADraftWithNoDescriptionFallsBackToItsTitle(t *testing.T) {
	reply := `{"drafts":[{"title":"Findable","content":"c","category":"fact","confidence":0.9}]}`
	h := newHarness(t, reply)

	out, err := h.obs.Observe(ctx(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Stored[0].Description != "Findable" {
		t.Fatalf("description = %q", out.Stored[0].Description)
	}
}

// TestTheObserverSeesWhatHappenedNotWhatWasInIt. A tool result can be a whole
// file, and this observer has twelve thousand characters for a session.
func TestTheObserverSeesWhatHappenedNotWhatWasInIt(t *testing.T) {
	h := newHarness(t, `{"drafts":[]}`)

	in := sampleInput()
	in.Messages = []agentloop.Message{
		{Role: agentloop.RoleAssistant, ToolCalls: []agentloop.ToolCall{{Name: "Read"}}},
		{Role: agentloop.RoleTool, Name: "Read", Result: []byte(`{"data":"the entire file"}`)},
		{Role: agentloop.RoleAssistant},
	}
	in.Events = []event.Record{{
		Type: event.PreToolUse, Hook: "policy",
		Outcome: event.Outcome{PermissionDecision: event.PermissionDeny},
	}}

	if _, err := h.obs.Observe(ctx(), in); err != nil {
		t.Fatal(err)
	}
	body := h.provider.asked()[0].Messages[0].Text

	if strings.Contains(body, "the entire file") {
		t.Fatal("a tool result was expanded into the observer's context")
	}
	if !strings.Contains(body, "called Read") || !strings.Contains(body, "Read returned") {
		t.Fatalf("the observer cannot see that a tool ran:\n%s", body)
	}
	if !strings.Contains(body, "deny") {
		t.Fatal("the observer cannot see that a call was refused")
	}
	if !strings.Contains(body, "(nothing)") {
		t.Fatal("an empty assistant turn was rendered as a blank rather than named")
	}
}

func asAgent(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: id})
}
