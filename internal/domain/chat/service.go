package chat

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
)

// defaultLimit is the page size of a listing. The sidebar shows a window, not
// the whole history.
const defaultLimit = 50

// Service is the conversation aggregate.
type Service struct {
	repo       Repository
	directory  Directory
	dispatcher Dispatcher
	canceller  Canceller
	clock      Clock
	ids        IDs
	log        *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo      Repository
	Directory Directory
	Clock     Clock
	IDs       IDs

	// Dispatcher is optional. Without one a message is still persisted and the
	// recipient still resolved; only the answer does not start. That is the
	// honest shape while the agent runtime is being built, and it is also what
	// happens in production when the runtime is down.
	Dispatcher Dispatcher

	// Canceller ends a turn already running. Optional for the same reason
	// Dispatcher is: a build with no runtime has nothing to cancel.
	Canceller Canceller

	Log *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		repo: d.Repo, directory: d.Directory, dispatcher: d.Dispatcher,
		canceller: d.Canceller,
		clock:     d.Clock, ids: d.IDs, log: log,
	}
}

// List returns the conversations, most recently updated first, without their
// messages.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, errReadFailed("List", err)
	}

	needle := strings.ToLower(strings.TrimSpace(in.Query))
	matched := make([]Chat, 0, len(found))
	for _, c := range found {
		if in.Kind != "" && c.Kind != in.Kind {
			continue
		}
		if in.Task != "" && c.Task != in.Task {
			continue
		}
		if in.Routine != "" && c.Routine != in.Routine {
			continue
		}
		if needle != "" && !matchesQuery(c, needle) {
			continue
		}
		c.Messages = nil
		matched = append(matched, c)
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
			return matched[i].ID < matched[j].ID // ordering is always total
		}
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})

	total := len(matched)
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit < len(matched) {
		matched = matched[:limit]
	}
	return ListOutput{Chats: matched, Total: total}, nil
}

func matchesQuery(c Chat, needle string) bool {
	for _, field := range []string{c.Title, c.ID, c.Task, c.Routine, string(c.Kind)} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

// Get reads one conversation with its whole transcript.
func (s *Service) Get(ctx context.Context, in GetInput) (*Chat, error) {
	id := strings.TrimSpace(in.Chat)
	got, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	return got, nil
}

// Update renames a conversation, or changes who can read it.
//
// Only what the caller named changes. The alternative — taking a whole Chat
// back and writing it — would let a rename drop the transcript, reopen a
// private conversation or move it to another task, none of which the screen
// offering the rename is asking for.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Chat, error) {
	id := strings.TrimSpace(in.Chat)
	current, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}

	if title := strings.TrimSpace(in.Title); title != "" {
		current.Title = title
	}
	if in.Visibility != "" {
		if in.Visibility != VisibilityPrivate && in.Visibility != VisibilityWorkspace {
			return nil, errInvalidVisibility(string(in.Visibility))
		}
		current.Visibility = in.Visibility
	}
	current.UpdatedAt = s.clock.Now()

	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// Clear empties the transcript, keeping the conversation.
//
// Clearing one that is already empty is not an error: the caller asked for a
// state, and that state holds.
// Stop ends the turn running on a conversation.
//
// Answering "there was nothing to stop" is not a refusal: a person presses
// the button when they see an agent working, and by the time the call lands
// the turn may have finished on its own. What *is* a refusal is a build with
// no runtime at all, because then the button can never work and saying so is
// the only honest answer.
func (s *Service) Stop(ctx context.Context, in StopInput) (StopOutput, error) {
	id := strings.TrimSpace(in.Chat)
	if _, err := s.repo.Get(ctx, collections.Key{"id": id}); err != nil {
		return StopOutput{}, errNotFound(id)
	}
	if s.canceller == nil {
		return StopOutput{}, errNoRuntime(id)
	}

	stopped, err := s.canceller.Stop(ctx, id)
	if err != nil {
		return StopOutput{}, err
	}
	return StopOutput{Chat: id, Stopped: stopped}, nil
}

// React toggles one reaction on one message.
//
// Toggling rather than adding, because that is what a person means by
// clicking the same emoji twice — and because two entries for the same actor
// and value would render as two, which is a bug nobody would report as one.
//
// The actor comes from the ambient identity, never from the payload: a caller
// that could name the actor could react as somebody else.
func (s *Service) React(ctx context.Context, in ReactInput) (*Chat, error) {
	id := strings.TrimSpace(in.Chat)
	current, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}

	actor, _ := identity.Actor(ctx)
	if actor == "" {
		return nil, errNoActor(id)
	}
	value := strings.TrimSpace(in.Value)
	if value == "" {
		return nil, errReactionEmpty(id)
	}

	messageID := strings.TrimSpace(in.Message)
	at := -1
	for i := range current.Messages {
		if current.Messages[i].ID == messageID {
			at = i
			break
		}
	}
	if at < 0 {
		return nil, errMessageNotFound(id, messageID)
	}

	message := &current.Messages[at]
	kept := make([]Reaction, 0, len(message.Reactions))
	removed := false
	for _, r := range message.Reactions {
		if r.Actor == actor && r.Value == value {
			removed = true
			continue
		}
		kept = append(kept, r)
	}
	if !removed {
		kept = append(kept, Reaction{Actor: actor, Value: value, Timestamp: s.clock.Now()})
	}
	message.Reactions = kept

	current.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, errWriteFailed("React", err)
	}
	return current, nil
}

func (s *Service) Clear(ctx context.Context, in ClearInput) (ClearOutput, error) {
	id := strings.TrimSpace(in.Chat)
	current, err := s.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return ClearOutput{Chat: id}, errNotFound(id)
	}

	removed := len(current.Messages)
	if removed == 0 {
		return ClearOutput{Chat: id, Removed: 0}, nil
	}

	// An empty slice rather than nil: the field is not omitempty, and a
	// conversation that answers `"messages": null` is one every reader has to
	// guard before iterating.
	current.Messages = []Message{}
	current.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return ClearOutput{Chat: id}, errWriteFailed("Clear", err)
	}
	return ClearOutput{Chat: id, Removed: removed}, nil
}

// Delete removes a conversation and the transcript with it.
//
// The conversation has to exist. Deleting something that is not there could be
// answered as a state that already holds — and is, for a workspace record
// nobody references — but a chat is what a screen is looking at: a delete that
// reports success for an identifier this workspace has never had is a caller
// deleting from the wrong workspace and not being told.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	id := strings.TrimSpace(in.Chat)
	if _, err := s.repo.Get(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{Chat: id}, errNotFound(id)
	}
	if err := s.repo.Delete(ctx, collections.Key{"id": id}); err != nil {
		return DeleteOutput{Chat: id}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{Chat: id, Deleted: true}, nil
}

// GetByChannel finds the conversation bound to an external messenger's own
// chat, by the same Key Create derives from Channel — so a second inbound
// message from the same Telegram chat lands in the same conversation as the
// first, rather than opening a new one every time. Added for
// internal/domain/bot, which needs to resolve an inbound webhook to a Chat
// before Send can write to it; nothing else in this package needed a lookup
// by anything other than ID until now.
func (s *Service) GetByChannel(ctx context.Context, provider, chatID string) (*Chat, error) {
	key := externalKey(provider, chatID)
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return nil, errReadFailed("GetByChannel", err)
	}
	for i := range found {
		if found[i].Key == key {
			return &found[i], nil
		}
	}
	return nil, errChannelNotFound(provider, chatID)
}

// Create opens a conversation.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Chat, error) {
	kind := in.Kind
	if kind == "" {
		kind = KindChannel
	}
	if !kind.Valid() {
		return nil, errInvalidKind(string(kind))
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = VisibilityWorkspace
	}
	if visibility != VisibilityPrivate && visibility != VisibilityWorkspace {
		return nil, errInvalidVisibility(string(visibility))
	}

	now := s.clock.Now()
	participants := make([]Participant, 0, len(in.Participants)+1)
	for _, p := range in.Participants {
		if p.JoinedAt.IsZero() {
			p.JoinedAt = now
		}
		if p.Role == "" {
			p.Role = "member"
		}
		p.ID = strings.TrimSpace(p.ID)
		if p.Type == ActorAgent {
			p.ID = strings.ToLower(p.ID)
		}
		participants = append(participants, p)
	}
	// Whoever opened the conversation is in it. A private chat its own creator
	// cannot read is not a thing anyone means to make.
	if actor, kind := identity.Actor(ctx); actor != "" {
		as := ActorUser
		if kind == identity.ActorAgent {
			as = ActorAgent
		}
		if !hasParticipant(participants, as, actor) {
			participants = append(participants, Participant{
				Type: as, ID: actor, Role: "admin", JoinedAt: now,
			})
		}
	}

	c := &Chat{
		ID:           s.ids.New(),
		Title:        strings.TrimSpace(in.Title),
		Kind:         kind,
		Visibility:   visibility,
		Participants: participants,
		Task:         in.Task,
		Routine:      in.Routine,
		Agent:        strings.ToLower(strings.TrimSpace(in.Agent)),
		Channel:      in.Channel,
		Messages:     []Message{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if c.Channel != nil {
		c.Key = externalKey(c.Channel.Provider, c.Channel.ChatID)
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return c, nil
}

// Send appends a message and hands the turn to the agent runtime.
//
// The order matters and is the one property of this method worth stating: the
// message is persisted before anything is dispatched. A worker that dies
// between the two loses the answer, not the question — and the question is the
// part the person typed.
func (s *Service) Send(ctx context.Context, in SendInput) (SendOutput, error) {
	c, err := s.Get(ctx, GetInput{Chat: in.Chat})
	if err != nil {
		return SendOutput{}, err
	}

	now := s.clock.Now()
	// The person's own words first, then whatever was attached. The order is
	// the point: a renderer that hides attached context still finds the
	// message's own text, and a reader — human or model — sees what was asked
	// before what was supplied.
	parts := make([]Part, 0, 1+len(in.Context))
	parts = append(parts, Part{Type: PartText, Text: in.Text})
	for _, attached := range in.Context {
		if strings.TrimSpace(attached) == "" {
			continue
		}
		parts = append(parts, Part{Type: PartText, Text: attached})
	}
	msg := Message{
		ID:        s.ids.New(),
		Role:      RoleUser,
		Parts:     parts,
		CreatedAt: now,
	}
	if actor, kind := identity.Actor(ctx); actor != "" {
		as := ActorUser
		role := RoleUser
		if kind == identity.ActorAgent {
			as, role = ActorAgent, RoleAssistant
		}
		msg.Author = &Author{Type: as, ID: actor}
		msg.Role = role
	}

	target, routed := s.route(ctx, c, in)

	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = now
	if err := s.repo.Update(ctx, c, collections.Version{}); err != nil {
		return SendOutput{}, errWriteFailed("Send", err)
	}

	out := SendOutput{Message: msg, Target: target}
	if !routed || s.dispatcher == nil {
		if !routed {
			// Not an error: the message is stored and readable. But the caller
			// has to know that nothing is coming, or it waits forever.
			s.log.Warn("message stored with nobody to answer it", "chat", c.ID, "message", msg.ID)
		}
		return out, nil
	}

	jobID, err := s.dispatcher.Dispatch(ctx, Turn{
		ChatID: c.ID, MessageID: msg.ID, AgentID: target.AgentID,
		Task: c.Task, Routine: c.Routine,
	})
	if err != nil {
		// The message survives. Reporting the failure of the dispatch rather
		// than of the whole call is the difference between "your message was
		// lost" and "your message is there and nobody has answered yet".
		s.log.Error("the turn could not be dispatched", "chat", c.ID, "message", msg.ID, "err", err)
		return out, nil
	}
	out.JobID, out.Dispatched = jobID, true
	return out, nil
}

// route decides who answers: the explicit override, then a mention, then the
// only agent in the conversation, then the orchestrator.
func (s *Service) route(ctx context.Context, c *Chat, in SendInput) (Target, bool) {
	if named := strings.ToLower(strings.TrimSpace(in.Agent)); named != "" {
		return Target{AgentID: named, Reason: ByMention}, true
	}
	orchestrator := ""
	if s.directory != nil {
		if found, err := s.directory.Orchestrator(ctx); err == nil {
			orchestrator = found
		}
	}
	isAgent := func(id string) bool {
		return s.directory != nil && s.directory.IsAgent(ctx, id)
	}
	return resolveTarget(c, in.Text, isAgent, orchestrator)
}

func hasParticipant(list []Participant, kind ActorType, id string) bool {
	for _, p := range list {
		if p.Type == kind && p.ID == id {
			return true
		}
	}
	return false
}

// externalKey is the stable lookup key of a conversation bound to a messenger,
// so that an inbound message finds its thread without scanning.
func externalKey(provider, chatID string) string { return "ext:" + provider + ":" + chatID }

// ReplyInput is the runtime handing back what an agent produced.
//
// It is not a command and it is not on the registry, deliberately: an agent's
// answer is written by the runtime that ran the turn, and a surface that could
// forge one would make the transcript worth nothing as a record of what
// happened.
type ReplyInput struct {
	Chat    string
	ReplyTo string
	AgentID string

	Parts []Part
	Usage TokenUsage

	// MessageID names the answer being stored, when the caller already
	// announced it. A turn that streams publishes snapshots of the answer
	// while it is being written, and those snapshots have to carry an id —
	// the interface keys on it to replace the in-progress message rather
	// than append another. Storing the finished answer under a fresh id
	// would leave that in-progress copy on screen forever, beside its own
	// completed twin. Empty means "mint one", which is every non-streaming
	// caller.
	MessageID string

	// Failure records a turn that did not answer. A turn that failed silently
	// is a conversation where somebody is still waiting.
	Failure *RunError

	// Interrupted marks a turn a person stopped, which is not the same thing
	// as one that failed. StatusInterrupted has existed on Run since this
	// aggregate was written and nothing ever assigned it, because nothing
	// could stop a turn.
	Interrupted bool

	// StartedAt is when the turn began, so the record shows how long it took
	// rather than only when it ended.
	StartedAt time.Time
}

// ReplyOutput is the stored answer.
type ReplyOutput struct {
	Message *Message `json:"message,omitempty"`
	Run     Run      `json:"run"`
}

// Post writes a message into a conversation without dispatching a turn for it.
//
// Like Reply, it is deliberately not a command. Send is the surface a person or
// an agent uses, and it dispatches — a message nobody answers is not what
// anybody means by sending one. This is the other case: a caller that is about
// to run the turn itself and needs the message on the record first. A routine
// is exactly that, because its run has to finish before the run record can say
// how it went, so it cannot hand the turn to a detached dispatcher and return.
//
// Sending through Send and then running the turn as well would be two turns for
// one message. That is a real defect, and it is the reason this exists.
func (s *Service) Post(ctx context.Context, chatID, text string) (*Message, error) {
	c, err := s.Get(ctx, GetInput{Chat: chatID})
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	msg := Message{
		ID:        s.ids.New(),
		Role:      RoleUser,
		Parts:     []Part{{Type: PartText, Text: text}},
		CreatedAt: now,
	}
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = now
	if err := s.repo.Update(ctx, c, collections.Version{}); err != nil {
		return nil, errWriteFailed("Post", err)
	}
	return &msg, nil
}

// Reply appends an agent's answer and records the attempt on the message that
// asked for it.
func (s *Service) Reply(ctx context.Context, in ReplyInput) (ReplyOutput, error) {
	c, err := s.Get(ctx, GetInput{Chat: in.Chat})
	if err != nil {
		return ReplyOutput{}, err
	}
	now := s.clock.Now()

	run := Run{
		AgentID:     in.AgentID,
		Status:      StatusCompleted,
		Usage:       in.Usage,
		StartedAt:   in.StartedAt,
		CompletedAt: &now,
	}
	if in.StartedAt.IsZero() {
		run.StartedAt = now
	}
	if in.Failure != nil {
		run.Status, run.Error = StatusError, in.Failure
	}
	if in.Interrupted {
		run.Status = StatusInterrupted
	}

	var out ReplyOutput
	if len(in.Parts) > 0 {
		id := strings.TrimSpace(in.MessageID)
		if id == "" {
			id = s.ids.New()
		}
		msg := Message{
			ID:        id,
			Role:      RoleAssistant,
			Author:    &Author{Type: ActorAgent, ID: in.AgentID},
			Parts:     in.Parts,
			ReplyTo:   in.ReplyTo,
			CreatedAt: now,
		}
		c.Messages = append(c.Messages, msg)
		out.Message = &msg
	}

	// The attempt is recorded on the message that triggered it, which is the
	// granularity at which somebody asks why a particular answer was expensive.
	for i := range c.Messages {
		if c.Messages[i].ID == in.ReplyTo {
			c.Messages[i].Runs = append(c.Messages[i].Runs, run)
			break
		}
	}

	c.UpdatedAt = now
	if err := s.repo.Update(ctx, c, collections.Version{}); err != nil {
		return ReplyOutput{}, errWriteFailed("Reply", err)
	}
	out.Run = run
	return out, nil
}
