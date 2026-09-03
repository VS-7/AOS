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

	want := []string{"chats_clear", "chats_create", "chats_delete", "chats_get", "chats_list", "chats_react", "chats_send", "chats_stop", "chats_update"}
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
		if d.Key() == "chats_delete" && !d.Annotations().DestructiveHint {
			t.Error("deleting a conversation takes its transcript with it, and says so")
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

// Post and Reply are the pair a caller that runs its own turn uses: put the
// question on the record, run it, then store the answer against it. Neither is
// a command — see their own doc comments — so nothing else in this suite
// reaches them, and they went untested when the streaming id was threaded
// through Reply.

func TestPostWritesTheMessageWithoutDispatchingATurn(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "A routine's own run"})

	msg, err := h.svc.Post(userCtx(), c.ID, "what changed today?")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if msg.Role != chat.RoleUser || len(msg.Parts) != 1 || msg.Parts[0].Text != "what changed today?" {
		t.Fatalf("message = %+v", msg)
	}
	// The whole reason this exists rather than Send: the caller is about to
	// run the turn itself, and dispatching here would run it twice.
	if len(h.dispatcher.turns) != 0 {
		t.Fatalf("Post dispatched a turn: %+v", h.dispatcher.turns)
	}

	stored, err := h.svc.Get(userCtx(), chat.GetInput{Chat: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Messages) != 1 || stored.Messages[0].ID != msg.ID {
		t.Fatalf("messages = %+v", stored.Messages)
	}
}

func TestReplyRecordsTheAttemptOnTheMessageThatAskedForIt(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "A question"})
	asked, err := h.svc.Post(userCtx(), c.ID, "why is it slow?")
	if err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.Reply(userCtx(), chat.ReplyInput{
		Chat:    c.ID,
		ReplyTo: asked.ID,
		AgentID: "atlas",
		Parts:   []chat.Part{{Type: chat.PartText, Text: "the index is cold"}},
		Usage:   chat.TokenUsage{Input: 12, Output: 34, Total: 46},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if out.Run.Status != chat.StatusCompleted || out.Run.AgentID != "atlas" {
		t.Fatalf("run = %+v", out.Run)
	}
	if out.Run.CompletedAt == nil || out.Run.StartedAt.IsZero() {
		t.Fatalf("an attempt with no span: %+v", out.Run)
	}

	stored, err := h.svc.Get(userCtx(), chat.GetInput{Chat: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Messages) != 2 || stored.Messages[1].Role != chat.RoleAssistant {
		t.Fatalf("messages = %+v", stored.Messages)
	}
	// The attempt belongs to the question, not to the answer: that is the
	// granularity at which somebody asks why a particular reply was expensive.
	if len(stored.Messages[0].Runs) != 1 || stored.Messages[0].Runs[0].Usage.Total != 46 {
		t.Fatalf("the run was not recorded on the question: %+v", stored.Messages[0])
	}
	if len(stored.Messages[1].Runs) != 0 {
		t.Fatalf("the run was recorded on the answer too: %+v", stored.Messages[1])
	}
}

// A streaming turn announces the id of the answer it is writing before it
// finishes writing it. Storing the finished answer under a fresh id leaves the
// in-progress copy on screen forever, beside its own completed twin.
func TestReplyStoresTheAnswerUnderTheIdTheTurnAlreadyAnnounced(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "A streamed answer"})
	asked, err := h.svc.Post(userCtx(), c.ID, "explain it")
	if err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.Reply(userCtx(), chat.ReplyInput{
		Chat:      c.ID,
		ReplyTo:   asked.ID,
		AgentID:   "atlas",
		MessageID: "announced-while-streaming",
		Parts:     []chat.Part{{Type: chat.PartText, Text: "here it is"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if out.Message == nil || out.Message.ID != "announced-while-streaming" {
		t.Fatalf("the announced id was not kept: %+v", out.Message)
	}
}

func TestAFailedTurnIsRecordedAsAnAttemptThatFailed(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, chat.CreateInput{Title: "A turn that could not answer"})
	asked, err := h.svc.Post(userCtx(), c.ID, "do the thing")
	if err != nil {
		t.Fatal(err)
	}

	out, err := h.svc.Reply(userCtx(), chat.ReplyInput{
		Chat:    c.ID,
		ReplyTo: asked.ID,
		AgentID: "atlas",
		Failure: &chat.RunError{Code: "AOS_AGENT_PROVIDER_FAILED", Message: "the provider answered 401"},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if out.Run.Status != chat.StatusError || out.Run.Error == nil {
		t.Fatalf("run = %+v", out.Run)
	}
	// A turn that failed silently is a conversation where somebody is still
	// waiting: the attempt has to be on the record even though there is no
	// message to show for it.
	if out.Message != nil {
		t.Fatalf("a failed turn stored an answer: %+v", out.Message)
	}
	stored, err := h.svc.Get(userCtx(), chat.GetInput{Chat: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Messages) != 1 {
		t.Fatalf("messages = %+v", stored.Messages)
	}
	if len(stored.Messages[0].Runs) != 1 || stored.Messages[0].Runs[0].Error.Code != "AOS_AGENT_PROVIDER_FAILED" {
		t.Fatalf("the failure was not recorded: %+v", stored.Messages[0].Runs)
	}
}

// TestDeleteRemovesTheConversation.
//
// The interface has had a delete on every conversation row since it was
// ported, and nothing was behind it: `chat.delete` was `null` in the frontend's
// command map because this group published four commands — list, get, create,
// send — and no way to remove one. A workspace accumulated every thread anyone
// had ever opened, including the throwaway ones, with no way to clear them.
func TestDeleteRemovesTheConversation(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{Title: "Throwaway"})

	out, err := h.svc.Delete(userCtx(), chat.DeleteInput{Chat: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if out.Chat != created.ID || !out.Deleted {
		t.Fatalf("output = %+v", out)
	}

	if _, err := h.svc.Get(userCtx(), chat.GetInput{Chat: created.ID}); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("the conversation survived: %v", err)
	}
}

func TestDeletingAConversationThatIsNotThereIsRefused(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Delete(userCtx(), chat.DeleteInput{Chat: "never-existed"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v, want not-found", err)
	}
}

// TestUpdateRenamesAConversation: the other half of what the row's menu offers.
func TestUpdateRenamesAConversation(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{Title: "Untitled"})

	renamed, err := h.svc.Update(userCtx(), chat.UpdateInput{
		Chat:  created.ID,
		Title: "Release planning",
	})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Title != "Release planning" {
		t.Fatalf("title = %q", renamed.Title)
	}
	if !renamed.UpdatedAt.After(created.UpdatedAt) {
		t.Error("the audit trail did not move")
	}

	read, err := h.svc.Get(userCtx(), chat.GetInput{Chat: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if read.Title != "Release planning" {
		t.Fatalf("the rename was not persisted: %q", read.Title)
	}
}

// TestUpdateLeavesUnnamedFieldsAlone. A rename must not silently reopen a
// private conversation or drop its transcript.
func TestUpdateLeavesUnnamedFieldsAlone(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{
		Title:      "Private",
		Visibility: chat.VisibilityPrivate,
		Kind:       chat.KindDM,
	})
	if _, err := h.svc.Send(userCtx(), chat.SendInput{Chat: created.ID, Text: "hello"}); err != nil {
		t.Fatal(err)
	}

	updated, err := h.svc.Update(userCtx(), chat.UpdateInput{Chat: created.ID, Title: "Still private"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Visibility != chat.VisibilityPrivate {
		t.Errorf("visibility = %q, want it untouched", updated.Visibility)
	}
	if updated.Kind != chat.KindDM {
		t.Errorf("kind = %q, want it untouched", updated.Kind)
	}
	if len(updated.Messages) == 0 {
		t.Error("the transcript was dropped by a rename")
	}
}

// TestUpdateCanOpenAPrivateConversationToTheWorkspace — the second field the
// row's menu offers, and the one with a consequence: it changes who can read
// the transcript.
func TestUpdateChangesVisibility(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{Title: "Draft", Visibility: chat.VisibilityPrivate})

	updated, err := h.svc.Update(userCtx(), chat.UpdateInput{
		Chat:       created.ID,
		Visibility: chat.VisibilityWorkspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Visibility != chat.VisibilityWorkspace {
		t.Fatalf("visibility = %q", updated.Visibility)
	}
	if updated.Title != "Draft" {
		t.Errorf("title = %q, want it untouched", updated.Title)
	}
}

func TestUpdatingAConversationThatIsNotThereIsRefused(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Update(userCtx(), chat.UpdateInput{Chat: "never-existed", Title: "x"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v, want not-found", err)
	}
}

// TestClearEmptiesTheTranscriptWithoutRemovingTheConversation.
//
// The composer has a "clear context" action, and it was calling `chat.update`
// with `{messages: []}` — a field Go's update does not have and must not have,
// since a rename that can silently drop a transcript is a rename nobody can
// trust. The action is real and separate, so it is its own command.
func TestClearEmptiesTheTranscriptWithoutRemovingTheConversation(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{Title: "Long thread"})
	if _, err := h.svc.Send(userCtx(), chat.SendInput{Chat: created.ID, Text: "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Send(userCtx(), chat.SendInput{Chat: created.ID, Text: "two"}); err != nil {
		t.Fatal(err)
	}

	cleared, err := h.svc.Clear(userCtx(), chat.ClearInput{Chat: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Removed != 2 {
		t.Fatalf("removed = %d, want the two messages", cleared.Removed)
	}

	read, err := h.svc.Get(userCtx(), chat.GetInput{Chat: created.ID})
	if err != nil {
		t.Fatalf("the conversation went with its messages: %v", err)
	}
	if len(read.Messages) != 0 {
		t.Fatalf("%d messages survived", len(read.Messages))
	}
	if read.Title != "Long thread" {
		t.Errorf("title = %q, want it untouched", read.Title)
	}
}

func TestClearingAnEmptyConversationIsNotAnError(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{Title: "Empty"})

	out, err := h.svc.Clear(userCtx(), chat.ClearInput{Chat: created.ID})
	if err != nil {
		t.Fatalf("clearing what is already clear is the state the caller asked for: %v", err)
	}
	if out.Removed != 0 {
		t.Fatalf("removed = %d", out.Removed)
	}
}

// codeOf is the apperr code of err, failing the test when there is none —
// every refusal in this package carries one.
func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not *apperr.Error: %v", err, err)
	}
	return app.Code
}

// fakeCanceller stands in for the agent runtime's own cancel registry.
type fakeCanceller struct {
	asked   []string
	running bool
	err     error
}

func (c *fakeCanceller) Stop(_ context.Context, chatID string) (bool, error) {
	c.asked = append(c.asked, chatID)
	if c.err != nil {
		return false, c.err
	}
	return c.running, nil
}

// The composer has always drawn a Stop button, and it called a command that
// did not exist: the facade answered its dormant envelope as a *success*, the
// screen said "No active run was found to stop", and the agent kept working.
// There was no way to end a turn at all.
func TestStopEndsTheTurnRunningOnAConversation(t *testing.T) {
	canceller := &fakeCanceller{running: true}
	h := newHarness(t, func(d *chat.Deps) { d.Canceller = canceller })
	created := h.create(t, chat.CreateInput{Title: "Going the wrong way"})

	out, err := h.svc.Stop(userCtx(), chat.StopInput{Chat: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Stopped {
		t.Error("the running turn was not stopped")
	}
	if len(canceller.asked) != 1 || canceller.asked[0] != created.ID {
		t.Errorf("the runtime was asked about %v, want the conversation", canceller.asked)
	}
}

// Pressing the button a moment late is not an error — the turn finished on
// its own, which is the outcome the person wanted.
func TestStoppingAConversationWithNothingRunningIsNotAnError(t *testing.T) {
	h := newHarness(t, func(d *chat.Deps) { d.Canceller = &fakeCanceller{running: false} })
	created := h.create(t, chat.CreateInput{})

	out, err := h.svc.Stop(userCtx(), chat.StopInput{Chat: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped {
		t.Error("a conversation with no turn reported one stopped")
	}
}

// A build with no runtime is the one case worth refusing: the button can
// never work here, and reporting "nothing was running" forever would hide
// that.
func TestStoppingWithoutARuntimeSaysSo(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{})

	_, err := h.svc.Stop(userCtx(), chat.StopInput{Chat: created.ID})
	if err == nil {
		t.Fatal("a daemon with no runtime claimed it could stop a turn")
	}
	if code := codeOf(t, err); code != "AOS_CHAT_NO_RUNTIME" {
		t.Errorf("code = %q, want AOS_CHAT_NO_RUNTIME", code)
	}
}

func TestStoppingAConversationThatDoesNotExist(t *testing.T) {
	h := newHarness(t, func(d *chat.Deps) { d.Canceller = &fakeCanceller{} })
	if _, err := h.svc.Stop(userCtx(), chat.StopInput{Chat: "nope"}); err == nil {
		t.Fatal("a conversation that does not exist was stopped")
	}
}

// `Message.Reactions` has been persisted since this aggregate was written and
// no command ever touched it, so the emoji picker the message list draws had
// nothing behind it.
func TestReactingTogglesTheMarkOffAndOn(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{})
	sent, err := h.svc.Send(userCtx(), chat.SendInput{Chat: created.ID, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	after, err := h.svc.React(userCtx(), chat.ReactInput{
		Chat: created.ID, Message: sent.Message.ID, Value: "👍",
	})
	if err != nil {
		t.Fatal(err)
	}
	reactions := after.Messages[len(after.Messages)-1].Reactions
	if len(reactions) != 1 || reactions[0].Value != "👍" || reactions[0].Actor != "vitor" {
		t.Fatalf("reactions = %+v, want one from the caller", reactions)
	}

	// The same one again takes it back, which is what clicking twice means.
	after, err = h.svc.React(userCtx(), chat.ReactInput{
		Chat: created.ID, Message: sent.Message.ID, Value: "👍",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Messages[len(after.Messages)-1].Reactions; len(got) != 0 {
		t.Errorf("reactions = %+v, want none left", got)
	}
}

func TestReactingToAMessageThatIsNotThere(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{})

	_, err := h.svc.React(userCtx(), chat.ReactInput{Chat: created.ID, Message: "m-404", Value: "👍"})
	if err == nil {
		t.Fatal("a reaction landed on a message that does not exist")
	}
	if code := codeOf(t, err); code != "AOS_CHAT_MESSAGE_NOT_FOUND" {
		t.Errorf("code = %q, want AOS_CHAT_MESSAGE_NOT_FOUND", code)
	}
}

// A reaction is somebody's mark. Without an identity there is nothing to
// attribute it to, and nobody could ever take it back.
func TestReactingWithoutAnIdentityIsRefused(t *testing.T) {
	h := newHarness(t)
	created := h.create(t, chat.CreateInput{})
	sent, err := h.svc.Send(userCtx(), chat.SendInput{Chat: created.ID, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.svc.React(context.Background(), chat.ReactInput{
		Chat: created.ID, Message: sent.Message.ID, Value: "👍",
	})
	if err == nil {
		t.Fatal("an anonymous reaction was stored")
	}
	if code := codeOf(t, err); code != "AOS_CHAT_ACTOR_REQUIRED" {
		t.Errorf("code = %q, want AOS_CHAT_ACTOR_REQUIRED", code)
	}
}
