package comment

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/domain/fakes"
)

type countingIDs struct{ n int }

func (g *countingIDs) New() string { g.n++; return "c" + strconv.Itoa(g.n) }

type existingParent map[string]bool

func (p existingParent) Exists(_ context.Context, taskID string) (bool, error) {
	return p[taskID], nil
}

// moderatorFor lets exactly one actor edit somebody else's comment. Who counts
// as a moderator is an installation's policy, which is why it is a port.
type moderatorFor string

func (m moderatorFor) MayModerate(ctx context.Context) bool {
	actor, _ := identity.Actor(ctx)
	return actor == string(m)
}

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newService(t *testing.T, mod Moderator) *Service {
	t.Helper()
	repo := fakes.NewRepo[Comment]("comments").WithKeyFunc(func(v *Comment) collections.Key {
		return collections.Key{"taskId": v.TaskID, "id": v.ID}
	})
	return NewService(Deps{
		Repo:      repo,
		Parent:    existingParent{"t-1": true},
		Moderator: mod,
		Clock:     &clockx.Stepping{At: start, Step: time.Minute},
		IDs:       &countingIDs{},
	})
}

func asAgent(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: id})
}

func asUser(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{UserID: id})
}

func mustCreate(t *testing.T, svc *Service, ctx context.Context, in CreateInput) *Comment {
	t.Helper()
	got, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestAuthorshipComesFromTheRequestAndNotThePayload. There is no author field
// on CreateInput, and that absence is the rule: an agent that could set one
// would make the discussion worthless as a record of who said what.
func TestAuthorshipComesFromTheRequestAndNotThePayload(t *testing.T) {
	svc := newService(t, nil)

	got := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "reproduced it"})
	if got.Author != "atlas" || got.AuthorType != "agent" {
		t.Fatalf("authorship = %q/%q", got.Author, got.AuthorType)
	}

	byPerson := mustCreate(t, svc, asUser("u-7"), CreateInput{Task: "t-1", Body: "thanks"})
	if byPerson.Author != "u-7" || byPerson.AuthorType != "user" {
		t.Fatalf("authorship = %q/%q", byPerson.Author, byPerson.AuthorType)
	}
}

// TestAnUnattributableCommentIsRefused. A comment nobody wrote is a hole in the
// one audit trail this aggregate exists to keep.
func TestAnUnattributableCommentIsRefused(t *testing.T) {
	svc := newService(t, nil)
	if _, err := svc.Create(context.Background(), CreateInput{Task: "t-1", Body: "who am i"}); err == nil {
		t.Fatal("a comment with no author was accepted")
	}
}

// TestAnAgentOnlyEditsWhatItWrote.
func TestAnAgentOnlyEditsWhatItWrote(t *testing.T) {
	svc := newService(t, nil)
	mine := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "mine"})

	if _, err := svc.Update(asAgent("nova"), UpdateInput{Task: "t-1", ID: mine.ID, Body: "rewritten"}); err == nil {
		t.Fatal("one agent rewrote another's comment")
	}
	if _, err := svc.Delete(asAgent("nova"), DeleteInput{Task: "t-1", ID: mine.ID}); err == nil {
		t.Fatal("one agent deleted another's comment")
	}

	edited, err := svc.Update(asAgent("atlas"), UpdateInput{Task: "t-1", ID: mine.ID, Body: "mine, corrected"})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Content != "mine, corrected" {
		t.Fatalf("body = %q", edited.Content)
	}
}

// TestAnAgentAndAUserWithTheSameNameAreNotTheSameActor. Without comparing the
// kind as well as the identifier, an agent called "vitor" could edit the
// comments of a person called "vitor".
func TestAnAgentAndAUserWithTheSameNameAreNotTheSameActor(t *testing.T) {
	svc := newService(t, nil)
	byPerson := mustCreate(t, svc, asUser("vitor"), CreateInput{Task: "t-1", Body: "written by the person"})

	if _, err := svc.Update(asAgent("vitor"), UpdateInput{
		Task: "t-1", ID: byPerson.ID, Body: "written by the agent",
	}); err == nil {
		t.Fatal("an agent edited a person's comment by sharing its name")
	}
}

// TestAModeratorIsTheExplicitException, and it is logged when it happens.
func TestAModeratorIsTheExplicitException(t *testing.T) {
	svc := newService(t, moderatorFor("super"))
	mine := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "mine"})

	if _, err := svc.Update(asUser("super"), UpdateInput{
		Task: "t-1", ID: mine.ID, Body: "moderated",
	}); err != nil {
		t.Fatalf("the moderator was refused: %v", err)
	}
	if _, err := svc.Update(asUser("someone-else"), UpdateInput{
		Task: "t-1", ID: mine.ID, Body: "not moderated",
	}); err == nil {
		t.Fatal("a non-moderator edited somebody else's comment")
	}
}

// TestAReplyToAReplyJoinsTheSameThread. Arbitrary depth adds nothing to a
// discussion and complicates every surface that renders it.
func TestAReplyToAReplyJoinsTheSameThread(t *testing.T) {
	svc := newService(t, nil)
	top := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "found the cause"})
	first := mustCreate(t, svc, asUser("u-7"), CreateInput{Task: "t-1", Body: "which line?", Parent: top.ID})
	second := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "sandbox/exec.go:61", Parent: first.ID})

	if second.ParentID != top.ID {
		t.Fatalf("a reply to a reply hangs off %q, want the top comment %q", second.ParentID, top.ID)
	}

	out, err := svc.List(asAgent("atlas"), ListInput{Task: "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Threads) != 1 || len(out.Threads[0].Replies) != 2 {
		t.Fatalf("threads = %+v", out.Threads)
	}
	if out.Total != 3 {
		t.Fatalf("total = %d", out.Total)
	}
}

// TestAReplyToNothingIsRefused.
func TestAReplyToNothingIsRefused(t *testing.T) {
	svc := newService(t, nil)
	if _, err := svc.Create(asAgent("atlas"), CreateInput{
		Task: "t-1", Body: "answering", Parent: "c-does-not-exist",
	}); err == nil {
		t.Fatal("a reply to a comment that does not exist was accepted")
	}
}

// TestDeletingAThreadHeadLeavesTheAnswersStanding. Cascading would let one
// participant erase another's words by removing the message they answered.
func TestDeletingAThreadHeadLeavesTheAnswersStanding(t *testing.T) {
	svc := newService(t, nil)
	top := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "found the cause"})
	reply := mustCreate(t, svc, asUser("u-7"), CreateInput{Task: "t-1", Body: "good catch", Parent: top.ID})

	out, err := svc.Delete(asAgent("atlas"), DeleteInput{Task: "t-1", ID: top.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Promoted) != 1 || out.Promoted[0] != reply.ID {
		t.Fatalf("promoted = %v", out.Promoted)
	}

	survivor, err := svc.Get(asUser("u-7"), GetInput{Task: "t-1", ID: reply.ID})
	if err != nil {
		t.Fatalf("the reply went with its parent: %v", err)
	}
	if survivor.ParentID != "" {
		t.Fatalf("the survivor still points at a comment that is gone: %q", survivor.ParentID)
	}
}

// TestACommentCannotHangOffATaskThatIsNotThere.
func TestACommentCannotHangOffATaskThatIsNotThere(t *testing.T) {
	svc := newService(t, nil)
	if _, err := svc.Create(asAgent("atlas"), CreateInput{Task: "t-missing", Body: "hello"}); err == nil {
		t.Fatal("a comment was written on a task that does not exist")
	}
}

// TestTheDiscussionReadsInWriteOrder.
func TestTheDiscussionReadsInWriteOrder(t *testing.T) {
	svc := newService(t, nil)
	for _, body := range []string{"first", "second", "third"} {
		mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: body})
	}
	out, err := svc.List(asAgent("atlas"), ListInput{Task: "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"first", "second", "third"}
	for i, c := range out.Comments {
		if c.Content != want[i] {
			t.Fatalf("position %d is %q, want %q", i, c.Content, want[i])
		}
	}
}
