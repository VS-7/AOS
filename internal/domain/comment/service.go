package comment

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
)

// Service is the comment aggregate.
type Service struct {
	repo      Repository
	parent    Parent
	moderator Moderator
	clock     Clock
	ids       IDs
	log       *slog.Logger
}

// Deps is what the service is built from.
type Deps struct {
	Repo   Repository
	Parent Parent
	Clock  Clock
	IDs    IDs

	// Moderator is optional. Without it nobody can edit another's comment,
	// which is the strict reading and the safe default.
	Moderator Moderator

	Log *slog.Logger
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		repo: d.Repo, parent: d.Parent, moderator: d.Moderator,
		clock: d.Clock, ids: d.IDs, log: log,
	}
}

// List returns a task's discussion, oldest first, grouped into threads.
func (s *Service) List(ctx context.Context, in ListInput) (ListOutput, error) {
	taskID := strings.TrimSpace(in.Task)
	if taskID == "" {
		return ListOutput{}, errTaskRequired("List")
	}
	all, err := s.all(ctx, taskID)
	if err != nil {
		return ListOutput{}, err
	}

	byParent := map[string][]Comment{}
	var tops []Comment
	for _, c := range all {
		if c.IsReply() {
			byParent[c.ParentID] = append(byParent[c.ParentID], c)
			continue
		}
		tops = append(tops, c)
	}

	threads := make([]Thread, 0, len(tops))
	for _, top := range tops {
		threads = append(threads, Thread{Comment: top, Replies: byParent[top.ID]})
	}
	return ListOutput{Comments: all, Threads: threads, Total: len(all)}, nil
}

// Get reads one comment.
func (s *Service) Get(ctx context.Context, in GetInput) (*Comment, error) {
	taskID := strings.TrimSpace(in.Task)
	if taskID == "" {
		return nil, errTaskRequired("Get")
	}
	found, err := s.repo.Get(ctx, collections.Key{"taskId": taskID, "id": strings.TrimSpace(in.ID)})
	if err != nil {
		return nil, errNotFound(taskID, in.ID)
	}
	return found, nil
}

// Create writes a comment, attributing it to whoever is acting.
//
// A reply to a reply attaches to the top-level comment rather than nesting
// deeper. Arbitrary depth adds nothing to a discussion and complicates every
// surface that has to render it.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Comment, error) {
	taskID := strings.TrimSpace(in.Task)
	if taskID == "" {
		return nil, errTaskRequired("Create")
	}
	actor, kind := identity.Actor(ctx)
	if actor == "" {
		return nil, errNoActor()
	}
	if err := s.requireParent(ctx, taskID); err != nil {
		return nil, err
	}

	parentID := strings.TrimSpace(in.Parent)
	if parentID != "" {
		parent, err := s.repo.Get(ctx, collections.Key{"taskId": taskID, "id": parentID})
		if err != nil {
			return nil, errParentCommentMissing(taskID, parentID)
		}
		if parent.IsReply() {
			parentID = parent.ParentID
		}
	}

	now := s.clock.Now()
	c := &Comment{
		TaskID:     taskID,
		ID:         s.ids.New(),
		Author:     actor,
		AuthorType: string(kind),
		ParentID:   parentID,
		CreatedAt:  now,
		UpdatedAt:  now,
		Content:    strings.TrimLeft(in.Body, " \t\n\r"),
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, errWriteFailed("Create", err)
	}
	return c, nil
}

// Update rewrites a comment's body, if the actor wrote it.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Comment, error) {
	current, err := s.Get(ctx, GetInput{Task: in.Task, ID: in.ID})
	if err != nil {
		return nil, err
	}
	if err := s.guardOwnership(ctx, current); err != nil {
		return nil, err
	}

	current.Content = strings.TrimLeft(in.Body, " \t\n\r")
	current.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, current, collections.Version{}); err != nil {
		return nil, errWriteFailed("Update", err)
	}
	return current, nil
}

// Delete removes a comment, if the actor wrote it.
//
// Deleting a top-level comment leaves its replies in place, promoted to the top
// level. Cascading would let one participant erase another's words by removing
// the message they were answering.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
	current, err := s.Get(ctx, GetInput{Task: in.Task, ID: in.ID})
	if err != nil {
		return DeleteOutput{}, err
	}
	if err := s.guardOwnership(ctx, current); err != nil {
		return DeleteOutput{}, err
	}

	promoted, err := s.promoteReplies(ctx, current)
	if err != nil {
		return DeleteOutput{}, err
	}

	key := collections.Key{"taskId": current.TaskID, "id": current.ID}
	if err := s.repo.Delete(ctx, key); err != nil {
		return DeleteOutput{}, errWriteFailed("Delete", err)
	}
	return DeleteOutput{ID: current.ID, Task: current.TaskID, Promoted: promoted}, nil
}

// promoteReplies detaches the replies of a comment about to be removed.
func (s *Service) promoteReplies(ctx context.Context, parent *Comment) ([]string, error) {
	if parent.IsReply() {
		return nil, nil
	}
	all, err := s.all(ctx, parent.TaskID)
	if err != nil {
		return nil, err
	}
	var promoted []string
	for i := range all {
		if all[i].ParentID != parent.ID {
			continue
		}
		reply := all[i]
		reply.ParentID = ""
		reply.UpdatedAt = s.clock.Now()
		if err := s.repo.Update(ctx, &reply, collections.Version{}); err != nil {
			return promoted, errWriteFailed("Delete", err)
		}
		promoted = append(promoted, reply.ID)
	}
	return promoted, nil
}

// guardOwnership enforces that an actor only edits what it wrote.
func (s *Service) guardOwnership(ctx context.Context, c *Comment) error {
	actor, kind := identity.Actor(ctx)
	if actor == "" {
		return errNoActor()
	}
	if c.Author == actor && c.AuthorType == string(kind) {
		return nil
	}
	if s.moderator != nil && s.moderator.MayModerate(ctx) {
		s.log.Info("a moderator edited another actor's comment",
			"comment", c.ID, "author", c.Author, "actor", actor)
		return nil
	}
	return errForbidden(c.ID, c.Author, actor)
}

func (s *Service) all(ctx context.Context, taskID string) ([]Comment, error) {
	found, err := s.repo.List(ctx, collections.Query{
		Key:            collections.Key{"taskId": strings.TrimSpace(taskID)},
		IncludeContent: true,
	})
	if err != nil {
		return nil, errReadFailed("List", err)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].CreatedAt.Equal(found[j].CreatedAt) {
			return found[i].ID < found[j].ID
		}
		return found[i].CreatedAt.Before(found[j].CreatedAt)
	})
	return found, nil
}

func (s *Service) requireParent(ctx context.Context, taskID string) error {
	if s.parent == nil {
		return nil
	}
	exists, err := s.parent.Exists(ctx, taskID)
	if err != nil {
		return errReadFailed("Create", err)
	}
	if !exists {
		return errParentMissing(taskID)
	}
	return nil
}
