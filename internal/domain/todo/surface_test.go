package todo

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
	svc, _ := newService(t)
	reg := command.NewRegistry()
	Register(reg, svc)

	want := []string{
		"todos_create", "todos_delete", "todos_get",
		"todos_list", "todos_set-status", "todos_update",
	}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for _, d := range reg.Sorted() {
		if d.Key() == "todos_delete" && !d.Annotations().DestructiveHint {
			t.Error("removing a step must be announced destructive")
		}
		if !d.InRegistry() {
			t.Errorf("%s is not reachable by an agent", d.Key())
		}
	}
}

// TestUpdateChangesEveryFieldItAdvertises.
func TestUpdateChangesEveryFieldItAdvertises(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "reproduce")

	got, err := svc.Update(ctx(), UpdateInput{
		Task: "t-1", ID: step.ID,
		Title:    ptr("Reproduce the failure in a test"),
		Order:    ptr(7),
		Evidence: ptr("the new test fails before the fix"),
		Content:  ptr("\n\nNotes."),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Reproduce the failure in a test" || got.Order != 7 {
		t.Fatalf("got %+v", got)
	}
	if got.Evidence == "" {
		t.Fatal("the evidence was not recorded")
	}
	if got.Content != "Notes." {
		t.Fatalf("content = %q — the leading blank lines were not trimmed", got.Content)
	}
}

// TestAnOrderGivenExplicitlyIsKept, so a step can be inserted rather than only
// appended.
func TestAnOrderGivenExplicitlyIsKept(t *testing.T) {
	svc, _ := newService(t)
	mustCreate(t, svc, "second")
	if _, err := svc.Create(ctx(), CreateInput{Task: "t-1", Title: "first", Order: -1}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.List(ctx(), ListInput{Task: "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Todos[0].Title != "first" {
		t.Fatalf("plan = %+v", out.Todos)
	}
}

// TestTheInputsThatCannotBeAccepted.
func TestTheInputsThatCannotBeAccepted(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "reproduce")

	cases := []struct {
		name string
		call func() error
		code string
	}{
		{"a status that is not one", func() error {
			_, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: step.ID, Status: "doing"})
			return err
		}, "TODO_INVALID_STATUS"},
		{"a step that is not there", func() error {
			_, err := svc.Get(ctx(), GetInput{Task: "t-1", ID: "s-missing"})
			return err
		}, "TODO_NOT_FOUND"},
		{"a task that is not there", func() error {
			_, err := svc.Create(ctx(), CreateInput{Task: "t-missing", Title: "step"})
			return err
		}, "TODO_TASK_NOT_FOUND"},
		{"no parent at all", func() error {
			_, err := svc.Create(ctx(), CreateInput{Title: "step"})
			return err
		}, "TODO_TASK_REQUIRED"},
		{"writing status through update", func() error {
			_, err := svc.Update(ctx(), UpdateInput{Task: "t-1", ID: step.ID, Status: Finished})
			return err
		}, "TODO_STATUS_NOT_WRITABLE"},
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

// TestAPlanThatCannotBeReadIsReportedRatherThanCountedAsEmpty. Counting an
// unreadable plan as finished is how a review guard fails open.
func TestAPlanThatCannotBeReadIsReportedRatherThanCountedAsEmpty(t *testing.T) {
	svc := NewService(Deps{Repo: brokenRepo{}, Parent: existingParent{"t-1": true}})

	if _, err := svc.CountPending(ctx(), "t-1"); err == nil {
		t.Fatal("an unreadable plan counted as having nothing pending")
	}
	if _, err := svc.Progress(ctx(), "t-1"); err == nil {
		t.Fatal("an unreadable plan reported progress")
	}
	if _, err := svc.PendingIDs(ctx(), "t-1"); err == nil {
		t.Fatal("an unreadable plan reported no open steps")
	}
}

// brokenRepo fails everything, which is what an unreadable directory looks like
// from here.
type brokenRepo struct{}

func (brokenRepo) Get(context.Context, collections.Key) (*Todo, error) {
	return nil, errors.New("no such file")
}
func (brokenRepo) List(context.Context, collections.Query) ([]Todo, error) {
	return nil, errors.New("no such directory")
}
func (brokenRepo) Create(context.Context, *Todo) error { return errors.New("no space left") }
func (brokenRepo) Update(context.Context, *Todo, collections.Version) error {
	return errors.New("no space left")
}
func (brokenRepo) Delete(context.Context, collections.Key) error { return errors.New("read-only") }
