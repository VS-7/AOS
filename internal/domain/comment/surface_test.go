package comment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
)

// TestRegisterPublishesTheWholeGroup.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	reg := command.NewRegistry()
	Register(reg, newService(t, nil))

	want := []string{
		"comments_create", "comments_delete", "comments_get",
		"comments_list", "comments_update",
	}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for _, d := range reg.Sorted() {
		if d.Key() == "comments_delete" && !d.Annotations().DestructiveHint {
			t.Error("removing a comment must be announced destructive")
		}
		if !d.InRegistry() {
			t.Errorf("%s is not reachable by an agent", d.Key())
		}
	}
}

// TestCreateInputHasNoAuthorField. The absence is the rule, so it is asserted
// rather than assumed: a field added here later would silently make the whole
// aggregate forgeable.
func TestCreateInputHasNoAuthorField(t *testing.T) {
	reg := command.NewRegistry()
	Register(reg, newService(t, nil))

	d, _, ok := reg.Lookup("comments_create")
	if !ok {
		t.Fatal("comments_create is not registered")
	}
	for i := range d.InputType().NumField() {
		name := strings.ToLower(d.InputType().Field(i).Name)
		if name == "author" || name == "authortype" {
			t.Fatalf("CreateInput has an %s field; authorship is server-side", name)
		}
	}
}

// TestTheInputsThatCannotBeAccepted.
func TestTheInputsThatCannotBeAccepted(t *testing.T) {
	svc := newService(t, nil)
	mine := mustCreate(t, svc, asAgent("atlas"), CreateInput{Task: "t-1", Body: "mine"})

	cases := []struct {
		name string
		call func() error
		code string
	}{
		{"no parent at all", func() error {
			_, err := svc.List(asAgent("atlas"), ListInput{})
			return err
		}, "COMMENT_TASK_REQUIRED"},
		{"a read with no parent", func() error {
			_, err := svc.Get(asAgent("atlas"), GetInput{ID: mine.ID})
			return err
		}, "COMMENT_TASK_REQUIRED"},
		{"a comment that is not there", func() error {
			_, err := svc.Get(asAgent("atlas"), GetInput{Task: "t-1", ID: "c-missing"})
			return err
		}, "COMMENT_NOT_FOUND"},
		{"a task that is not there", func() error {
			_, err := svc.Create(asAgent("atlas"), CreateInput{Task: "t-missing", Body: "hi"})
			return err
		}, "COMMENT_TASK_NOT_FOUND"},
		{"a reply to nothing", func() error {
			_, err := svc.Create(asAgent("atlas"), CreateInput{Task: "t-1", Body: "hi", Parent: "c-missing"})
			return err
		}, "COMMENT_PARENT_NOT_FOUND"},
		{"an edit by somebody else", func() error {
			_, err := svc.Update(asAgent("nova"), UpdateInput{Task: "t-1", ID: mine.ID, Body: "no"})
			return err
		}, "COMMENT_FORBIDDEN"},
		{"a write with no identity", func() error {
			_, err := svc.Create(context.Background(), CreateInput{Task: "t-1", Body: "hi"})
			return err
		}, "COMMENT_NO_ACTOR"},
		{"an edit with no identity", func() error {
			_, err := svc.Update(context.Background(), UpdateInput{Task: "t-1", ID: mine.ID, Body: "hi"})
			return err
		}, "COMMENT_NO_ACTOR"},
	}
	for _, tc := range cases {
		err := tc.call()
		if err == nil {
			t.Fatalf("%s was accepted", tc.name)
		}
		got, ok := apperr.As(err)
		if !ok || !strings.HasSuffix(got.Code, tc.code) {
			t.Fatalf("%s: error = %v, want %s", tc.name, err, tc.code)
		}
	}
}

// TestADiscussionThatCannotBeReadIsReported.
func TestADiscussionThatCannotBeReadIsReported(t *testing.T) {
	svc := NewService(Deps{Repo: brokenRepo{}, Parent: existingParent{"t-1": true}})
	if _, err := svc.List(asAgent("atlas"), ListInput{Task: "t-1"}); err == nil {
		t.Fatal("an unreadable discussion listed as empty")
	}
}

// brokenRepo fails everything, which is what an unreadable directory looks like
// from here.
type brokenRepo struct{}

func (brokenRepo) Get(context.Context, collections.Key) (*Comment, error) {
	return nil, errors.New("no such file")
}
func (brokenRepo) List(context.Context, collections.Query) ([]Comment, error) {
	return nil, errors.New("no such directory")
}
func (brokenRepo) Create(context.Context, *Comment) error { return errors.New("no space left") }
func (brokenRepo) Update(context.Context, *Comment, collections.Version) error {
	return errors.New("no space left")
}
func (brokenRepo) Delete(context.Context, collections.Key) error { return errors.New("read-only") }
