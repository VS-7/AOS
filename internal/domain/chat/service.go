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
		clock: d.Clock, ids: d.IDs, log: log,
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
	msg := Message{
		ID:        s.ids.New(),
		Role:      RoleUser,
		Parts:     []Part{{Type: PartText, Text: in.Text}},
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

	// Failure records a turn that did not answer. A turn that failed silently
	// is a conversation where somebody is still waiting.
	Failure *RunError

	// StartedAt is when the turn began, so the record shows how long it took
	// rather than only when it ended.
	StartedAt time.Time
}

// ReplyOutput is the stored answer.
type ReplyOutput struct {
	Message *Message `json:"message,omitempty"`
	Run     Run      `json:"run"`
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

	var out ReplyOutput
	if len(in.Parts) > 0 {
		msg := Message{
			ID:        s.ids.New(),
			Role:      RoleAssistant,
			Author:    &Author{Type: ActorAgent, ID: in.AgentID},
			Parts:     in.Parts,
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
