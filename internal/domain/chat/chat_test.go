package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/chat"
	"github.com/OWNER/aos/internal/domain/fakes"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

// fakeDirectory answers the two routing questions from a fixed roster.
type fakeDirectory struct {
	agents       map[string]bool
	orchestrator string
	err          error
}

func (d fakeDirectory) IsAgent(_ context.Context, id string) bool { return d.agents[id] }

func (d fakeDirectory) Orchestrator(context.Context) (string, error) {
	if d.err != nil {
		return "", d.err
	}
	return d.orchestrator, nil
}

// fakeDispatcher records the turns it was handed.
type fakeDispatcher struct {
	turns []chat.Turn
	err   error
}

func (d *fakeDispatcher) Dispatch(_ context.Context, in chat.Turn) (string, error) {
	if d.err != nil {
		return "", d.err
	}
	d.turns = append(d.turns, in)
	return "job-1", nil
}

type harness struct {
	svc        *chat.Service
	repo       *fakes.Repo[chat.Chat]
	dispatcher *fakeDispatcher
}

func newHarness(t *testing.T, opts ...func(*chat.Deps)) *harness {
	t.Helper()
	repo := fakes.NewRepo[chat.Chat]("chats")
	dispatcher := &fakeDispatcher{}
	deps := chat.Deps{
		Repo:       repo,
		Directory:  fakeDirectory{agents: map[string]bool{"atlas": true, "reviewer": true}, orchestrator: "atlas"},
		Dispatcher: dispatcher,
		Clock:      &clockx.Stepping{At: refTime, Step: time.Minute},
		IDs:        &ids.Sequence{Prefix: "c"},
	}
	for _, o := range opts {
		o(&deps)
	}
	return &harness{svc: chat.NewService(deps), repo: repo, dispatcher: dispatcher}
}

func userCtx() context.Context {
	return identity.With(context.Background(), identity.Identity{UserID: "vitor"})
}

func (h *harness) create(t *testing.T, in chat.CreateInput) *chat.Chat {
	t.Helper()
	if in.Title == "" {
		in.Title = "A conversation"
	}
	got, err := h.svc.Create(userCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCreateStampsTheDefaults(t *testing.T) {
	h := newHarness(t)
	got := h.create(t, chat.CreateInput{Title: "Planning"})

	if got.Kind != chat.KindChannel || got.Visibility != chat.VisibilityWorkspace {
		t.Fatalf("defaults = %q / %q", got.Kind, got.Visibility)
	}
	if got.ID != "c-1" || got.Messages == nil {
		t.Fatalf("chat = %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("no creation timestamp")
	}
}

// TestTheCreatorIsAParticipant: a private conversation its own creator cannot
// read is not something anyone means to make.
func TestTheCreatorIsAParticipant(t *testing.T) {
	h := newHarness(t)
	got := h.create(t, chat.CreateInput{Title: "Private", Visibility: chat.VisibilityPrivate})

	if !got.HasParticipant(chat.ActorUser, "vitor") {
		t.Fatalf("participants = %+v", got.Participants)
	}
	if got.Participants[0].Role != "admin" {
		t.Errorf("the creator joined as %q", got.Participants[0].Role)
	}
}

func TestAnAgentCreatorJoinsAsAnAgent(t *testing.T) {
	h := newHarness(t)
	agentCtx := identity.With(context.Background(), identity.Identity{AgentID: "atlas"})
	got, err := h.svc.Create(agentCtx, chat.CreateInput{Title: "Started by an agent"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasParticipant(chat.ActorAgent, "atlas") {
		t.Fatalf("participants = %+v", got.Participants)
	}
}

func TestCreateRejectsAnUnknownKindOrVisibility(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Create(userCtx(), chat.CreateInput{Title: "x", Kind: "thread"}); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("kind: error = %v", err)
	}
	if _, err := h.svc.Create(userCtx(), chat.CreateInput{Title: "x", Visibility: "secret"}); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("visibility: error = %v", err)
	}
}

func TestAnExternalChatGetsAStableLookupKey(t *testing.T) {
	h := newHarness(t)
	got := h.create(t, chat.CreateInput{
		Title: "Telegram", Kind: chat.KindExternal,
		Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "123456789"},
	})
	if got.Key != "ext:telegram:123456789" {
		t.Fatalf("key = %q", got.Key)
	}
}

func TestGetByChannelFindsTheConversationBoundToThatChannel(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{
		Title: "Telegram", Kind: chat.KindExternal,
		Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "123456789"},
	})

	got, err := h.svc.GetByChannel(userCtx(), "telegram", "123456789")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetByChannel found %q, want %q", got.ID, created.ID)
	}
}

func TestGetByChannelOfAnUnboundChatIsNotFound(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.GetByChannel(userCtx(), "telegram", "does-not-exist")
	var app *apperr.Error
	if !errors.As(err, &app) || app.Code != "AOS_CHAT_CHANNEL_NOT_FOUND" {
		t.Fatalf("code = %v, want AOS_CHAT_CHANNEL_NOT_FOUND", err)
	}
}

func TestGetByChannelDistinguishesProviders(t *testing.T) {
	h := newHarness(t)
	h.create(t, chat.CreateInput{
		Title: "Telegram", Kind: chat.KindExternal,
		Channel: &chat.ChannelMeta{Provider: "telegram", ChatID: "1"},
	})
	// Same chat id, different provider: must not match.
	_, err := h.svc.GetByChannel(userCtx(), "whatsapp", "1")
	if err == nil {
		t.Fatal("GetByChannel matched across providers on the same chat id")
	}
}

// TestSendPersistsBeforeItDispatches is the ordering the whole method exists
// to guarantee: a runtime that dies in between loses the answer, not the
// question.
func TestSendPersistsBeforeItDispatches(t *testing.T) {
	h := newHarness(t, func(d *chat.Deps) {
		d.Dispatcher = &failingDispatcher{}
	})
	c := h.create(t, chat.CreateInput{Title: "Planning"})

	out, err := h.svc.Send(userCtx(), chat.SendInput{Chat: c.ID, Text: "What changed?"})
	if err != nil {
		t.Fatalf("a dispatch failure must not lose the message: %v", err)
	}
	if out.Dispatched {
		t.Error("nothing was dispatched")
	}

	got, err := h.svc.Get(userCtx(), chat.GetInput{Chat: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Text() != "What changed?" {
		t.Fatalf("messages = %+v", got.Messages)
	}
}

type failingDispatcher struct{}

func (failingDispatcher) Dispatch(context.Context, chat.Turn) (string, error) {
	return "", errors.New("the runtime is down")
}

func TestSendDispatchesTheTurn(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "Planning"})

	out, err := h.svc.Send(userCtx(), chat.SendInput{Chat: c.ID, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Dispatched || out.JobID != "job-1" {
		t.Fatalf("out = %+v", out)
	}
	if len(h.dispatcher.turns) != 1 {
		t.Fatalf("turns = %+v", h.dispatcher.turns)
	}
	turn := h.dispatcher.turns[0]
	if turn.ChatID != c.ID || turn.MessageID != out.Message.ID || turn.AgentID != "atlas" {
		t.Fatalf("turn = %+v", turn)
	}
}

// TestRoutingPrefersAnExplicitMention covers the three ways a recipient is
// chosen, each of which is a different kind of evidence.
func TestRoutingPrefersAnExplicitMention(t *testing.T) {
	cases := []struct {
		name         string
		participants []chat.Participant
		text         string
		wantAgent    string
		wantReason   string
	}{
		{
			name:       "an explicit mention is an instruction",
			text:       "@reviewer please look at this",
			wantAgent:  "reviewer",
			wantReason: chat.ByMention,
		},
		{
			name:       "the bracketed form too",
			text:       "@[reviewer] please look",
			wantAgent:  "reviewer",
			wantReason: chat.ByMention,
		},
		{
			name:       "and the composer's markup",
			text:       `hello <mention id="reviewer">reviewer</mention>`,
			wantAgent:  "reviewer",
			wantReason: chat.ByMention,
		},
		{
			name:         "one agent in the room is an inference",
			participants: []chat.Participant{{Type: chat.ActorAgent, ID: "reviewer"}},
			text:         "what do you think",
			wantAgent:    "reviewer",
			wantReason:   chat.ByParticipant,
		},
		{
			name:       "and otherwise the orchestrator answers",
			text:       "what do you think",
			wantAgent:  "atlas",
			wantReason: chat.ByOrchestrator,
		},
		{
			name:         "a mention beats the only agent in the room",
			participants: []chat.Participant{{Type: chat.ActorAgent, ID: "reviewer"}},
			text:         "@atlas take this one",
			wantAgent:    "atlas",
			wantReason:   chat.ByMention,
		},
		{
			name:       "a mention of something that is not an agent is ignored",
			text:       "see #issue-42 for context",
			wantAgent:  "atlas",
			wantReason: chat.ByOrchestrator,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			created := h.create(t, chat.CreateInput{Title: "t", Participants: c.participants})

			out, err := h.svc.Send(userCtx(), chat.SendInput{Chat: created.ID, Text: c.text})
			if err != nil {
				t.Fatal(err)
			}
			if out.Target.AgentID != c.wantAgent || out.Target.Reason != c.wantReason {
				t.Fatalf("target = %+v, want %s by %s", out.Target, c.wantAgent, c.wantReason)
			}
		})
	}
}

func TestAnExplicitAgentOverridesTheText(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "t"})

	out, err := h.svc.Send(userCtx(), chat.SendInput{
		Chat: c.ID, Text: "@reviewer look at this", Agent: "atlas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Target.AgentID != "atlas" {
		t.Fatalf("target = %+v", out.Target)
	}
}

// TestAMessageWithNobodyToAnswerIsStillStored: the caller has to be able to
// tell the difference between "queued" and "nothing is coming".
func TestAMessageWithNobodyToAnswerIsStillStored(t *testing.T) {
	h := newHarness(t, func(d *chat.Deps) {
		d.Directory = fakeDirectory{agents: map[string]bool{}}
	})
	c := h.create(t, chat.CreateInput{Title: "t"})

	out, err := h.svc.Send(userCtx(), chat.SendInput{Chat: c.ID, Text: "anyone?"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Dispatched || out.Target.AgentID != "" {
		t.Fatalf("out = %+v", out)
	}
	got, err := h.svc.Get(userCtx(), chat.GetInput{Chat: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("the message was lost: %+v", got.Messages)
	}
}

func TestSendStampsTheAuthorAndTheRole(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "t"})

	fromHuman, err := h.svc.Send(userCtx(), chat.SendInput{Chat: c.ID, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if fromHuman.Message.Role != chat.RoleUser || fromHuman.Message.Author.ID != "vitor" {
		t.Fatalf("message = %+v", fromHuman.Message)
	}

	agentCtx := identity.With(context.Background(), identity.Identity{AgentID: "reviewer"})
	fromAgent, err := h.svc.Send(agentCtx, chat.SendInput{Chat: c.ID, Text: "on it"})
	if err != nil {
		t.Fatal(err)
	}
	if fromAgent.Message.Role != chat.RoleAssistant || fromAgent.Message.Author.Type != chat.ActorAgent {
		t.Fatalf("message = %+v", fromAgent.Message)
	}
}

func TestSendToAMissingConversationIsNotFound(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Send(userCtx(), chat.SendInput{Chat: "ghost", Text: "hello"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

// TestListLeavesTheTranscriptsOut: a listing answers what conversations exist,
// and returning every message to answer it would be the most expensive call in
// the system.
func TestListLeavesTheTranscriptsOut(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "Planning"})
	if _, err := h.svc.Send(userCtx(), chat.SendInput{Chat: c.ID, Text: "hello"}); err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.List(userCtx(), chat.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 {
		t.Fatalf("total = %d", out.Total)
	}
	if len(out.Chats[0].Messages) != 0 {
		t.Fatalf("the listing carried %d messages", len(out.Chats[0].Messages))
	}
	// And reading one does return them.
	full, err := h.svc.Get(userCtx(), chat.GetInput{Chat: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Messages) != 1 {
		t.Fatalf("messages = %+v", full.Messages)
	}
}

func TestListIsOrderedByRecency(t *testing.T) {
	h := newHarness(t)
	first := h.create(t, chat.CreateInput{Title: "First"})
	h.create(t, chat.CreateInput{Title: "Second"})
	// Writing to the first makes it the most recent.
	if _, err := h.svc.Send(userCtx(), chat.SendInput{Chat: first.ID, Text: "bump"}); err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.List(userCtx(), chat.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Chats[0].Title != "First" {
		t.Fatalf("order = %v", titlesOf(out.Chats))
	}
}

func TestListFiltersAndPages(t *testing.T) {
	h := newHarness(t)
	h.create(t, chat.CreateInput{Title: "Planning", Kind: chat.KindChannel})
	h.create(t, chat.CreateInput{Title: "Review", Kind: chat.KindDM})
	h.create(t, chat.CreateInput{Title: "Task thread", Kind: chat.KindTask, Task: "t-1"})

	byKind, err := h.svc.List(userCtx(), chat.ListInput{Kind: chat.KindDM})
	if err != nil {
		t.Fatal(err)
	}
	if byKind.Total != 1 || byKind.Chats[0].Title != "Review" {
		t.Fatalf("byKind = %v", titlesOf(byKind.Chats))
	}

	byTask, err := h.svc.List(userCtx(), chat.ListInput{Task: "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	if byTask.Total != 1 {
		t.Fatalf("byTask = %v", titlesOf(byTask.Chats))
	}

	byQuery, err := h.svc.List(userCtx(), chat.ListInput{Query: "plan"})
	if err != nil {
		t.Fatal(err)
	}
	if byQuery.Total != 1 || byQuery.Chats[0].Title != "Planning" {
		t.Fatalf("byQuery = %v", titlesOf(byQuery.Chats))
	}

	page, err := h.svc.List(userCtx(), chat.ListInput{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Chats) != 2 || page.Total != 3 {
		t.Fatalf("page = %d of %d", len(page.Chats), page.Total)
	}
}

// TestEveryPartTypeRoundTrips: the original relaxes the persisted schema to
// `any` because its SDK types cannot be expressed in JSON Schema. That
// constraint does not exist here, and the type is kept on both sides.
func TestEveryPartTypeRoundTrips(t *testing.T) {
	original := chat.Message{
		ID: "m-1", Role: chat.RoleAssistant, CreatedAt: refTime,
		Parts: []chat.Part{
			{Type: chat.PartText, Text: "here is what I found"},
			{Type: chat.PartReasoning, Text: "checked the index first"},
			{
				Type: chat.PartToolCall, ToolName: "memories_recall", ToolCallID: "call-1",
				Input: json.RawMessage(`{"query":"gateway"}`),
			},
			{
				Type: chat.PartToolResult, ToolName: "memories_recall", ToolCallID: "call-1",
				Output: json.RawMessage(`{"total":2}`),
			},
			{Type: chat.PartFile, MediaType: "image/png", URI: "file:///tmp/shot.png"},
		},
		Runs: []chat.Run{{
			AgentID: "atlas", Status: chat.StatusCompleted, StartedAt: refTime,
			Usage: chat.TokenUsage{Input: 412, Output: 98, Total: 510, CostUSD: 0.0031},
		}},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var back chat.Message
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}

	if len(back.Parts) != len(original.Parts) {
		t.Fatalf("parts = %d, want %d", len(back.Parts), len(original.Parts))
	}
	for i := range original.Parts {
		if back.Parts[i].Type != original.Parts[i].Type {
			t.Errorf("part %d type = %q", i, back.Parts[i].Type)
		}
	}
	if string(back.Parts[2].Input) != `{"query":"gateway"}` {
		t.Errorf("tool input = %s", back.Parts[2].Input)
	}
	if back.Runs[0].Usage.CostUSD != 0.0031 {
		t.Errorf("cost = %v", back.Runs[0].Usage.CostUSD)
	}
}

// TestTotalUsageSumsEveryAttempt: cost is audited per message because that is
// the granularity at which a person asks why something got expensive.
func TestTotalUsageSumsEveryAttempt(t *testing.T) {
	m := chat.Message{Runs: []chat.Run{
		{Usage: chat.TokenUsage{Input: 100, Output: 20, Total: 120, CostUSD: 0.001}},
		{Usage: chat.TokenUsage{Input: 150, Output: 30, Total: 180, CostUSD: 0.002}},
	}}
	got := m.TotalUsage()
	if got.Total != 300 || got.Input != 250 || got.Output != 50 {
		t.Fatalf("usage = %+v", got)
	}
	if got.CostUSD < 0.0029 || got.CostUSD > 0.0031 {
		t.Fatalf("cost = %v", got.CostUSD)
	}
}

func TestMentionsAreDeduplicatedAndLowercased(t *testing.T) {
	got := chat.Mentions("@Atlas and @atlas and #Atlas, plus @[reviewer]")
	if len(got) != 2 || got[0] != "atlas" || got[1] != "reviewer" {
		t.Fatalf("mentions = %v", got)
	}
	if len(chat.Mentions("  ")) != 0 {
		t.Error("blank text mentions nobody")
	}
	// An address in the middle of a word is not a mention.
	if len(chat.Mentions("write to me@example.com")) != 0 {
		t.Errorf("mentions = %v", chat.Mentions("write to me@example.com"))
	}
}

func TestRegisterPublishesTheGroup(t *testing.T) {
	h := newHarness(t)
	reg := command.NewRegistry()
	chat.Register(reg, h.svc)

	want := []string{"chats_create", "chats_get", "chats_list", "chats_send"}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commands = %v, want %v", got, want)
		}
	}
	for _, d := range reg.Sorted() {
		if d.Key() == "chats_send" && d.Annotations().ReadOnlyHint {
			t.Error("sending a message is not a read")
		}
	}
}

func titlesOf(cs []chat.Chat) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Title
	}
	return out
}
