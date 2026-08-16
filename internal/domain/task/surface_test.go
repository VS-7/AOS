package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
)

// TestRegisterPublishesTheWholeGroup covers the projection: the domain declares
// the commands, and every surface is derived from that declaration.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	h := newHarness(t)
	reg := command.NewRegistry()
	Register(reg, h.svc)

	want := []string{
		"tasks_branch", "tasks_create", "tasks_delete",
		"tasks_get", "tasks_list", "tasks_set-status", "tasks_update",
	}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", got, want)
	}

	for _, d := range reg.Sorted() {
		switch d.Key() {
		case "tasks_list", "tasks_get":
			if !d.Annotations().ReadOnlyHint {
				t.Errorf("%s must be announced read-only", d.Key())
			}
		case "tasks_delete":
			if !d.Annotations().DestructiveHint {
				t.Errorf("%s must be announced destructive", d.Key())
			}
		}
		if !d.InRegistry() {
			t.Errorf("%s is not reachable by an agent", d.Key())
		}
	}
}

// TestTheListFiltersOnEveryFieldItAdvertises.
func TestTheListFiltersOnEveryFieldItAdvertises(t *testing.T) {
	h := newHarness(t)
	h.create(t, CreateInput{
		Name: "Fix the pattern", Type: "bug", Status: Todo,
		Assigned: "atlas", Project: "p-1", Goal: "g-1",
	})
	newest := h.create(t, CreateInput{Name: "Add the scheduler", Type: "feature", Status: Backlog})

	cases := []struct {
		name string
		in   ListInput
		want int
	}{
		{"everything", ListInput{}, 2},
		{"by status", ListInput{Status: Todo}, 1},
		{"by type", ListInput{Type: "bug"}, 1},
		{"by owner", ListInput{Assigned: "ATLAS"}, 1},
		{"by project", ListInput{Project: "p-1"}, 1},
		{"by goal", ListInput{Goal: "g-1"}, 1},
		{"by nothing that matches", ListInput{Type: "docs"}, 0},
	}
	for _, tc := range cases {
		out, err := h.svc.List(ctx(), tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if out.Total != tc.want {
			t.Fatalf("%s: total = %d, want %d", tc.name, out.Total, tc.want)
		}
	}

	// The page is cut after the count, so paging does not shrink the total.
	out, err := h.svc.List(ctx(), ListInput{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || len(out.Tasks) != 1 {
		t.Fatalf("total %d, page %d", out.Total, len(out.Tasks))
	}
	if out.Tasks[0].ID != newest.ID {
		t.Fatalf("newest first put %q at the top", out.Tasks[0].Name)
	}

	skipped, err := h.svc.List(ctx(), ListInput{Offset: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped.Tasks) != 0 || skipped.Total != 2 {
		t.Fatalf("offset past the end returned %d of %d", len(skipped.Tasks), skipped.Total)
	}
}

// TestUpdateChangesEveryFieldItAdvertises, and the slug follows the name — a
// branch cut later carries the new one.
func TestUpdateChangesEveryFieldItAdvertises(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Old name", Status: Todo})
	due := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)

	got, err := h.svc.Update(ctx(), UpdateInput{
		ID:       task.ID,
		Name:     ptr("Fix the denial pattern"),
		Type:     ptr("bug"),
		Assigned: ptr("atlas"),
		Priority: ptr(Urgent),
		Summary:  ptr("The glob stops at a separator."),
		DueAt:    ptr(due.Format(time.RFC3339)),
		Project:  ptr("p-1"),
		Goal:     ptr("g-1"),
		Content:  ptr("\n\nThe body."),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Fix the denial pattern" || got.Slug != "fix-the-denial-pattern" {
		t.Fatalf("name/slug = %q/%q", got.Name, got.Slug)
	}
	if got.Type != "bug" || got.Assigned != "atlas" || got.Priority != Urgent {
		t.Fatalf("got %+v", got.Task)
	}
	if got.DueAt == nil || !got.DueAt.Equal(due) {
		t.Fatalf("dueAt = %v", got.DueAt)
	}
	if got.Project != "p-1" || got.Goal != "g-1" {
		t.Fatalf("project/goal = %q/%q", got.Project, got.Goal)
	}
	if got.Content != "The body." {
		t.Fatalf("content = %q — the leading blank lines were not trimmed", got.Content)
	}

	cleared, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, DueAt: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.DueAt != nil {
		t.Fatal("an empty due date did not clear it")
	}
}

// TestTheInputsThatCannotBeAccepted, each with the code that says why.
func TestTheInputsThatCannotBeAccepted(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})

	cases := []struct {
		name string
		call func() error
		code string
	}{
		{"a name with nothing in it", func() error {
			_, err := h.svc.Create(ctx(), CreateInput{Name: "   "})
			return err
		}, "TASK_INVALID_NAME"},
		{"a name that slugs to nothing", func() error {
			_, err := h.svc.Create(ctx(), CreateInput{Name: "!!!"})
			return err
		}, "TASK_INVALID_NAME"},
		{"a status that is not one", func() error {
			_, err := h.svc.Create(ctx(), CreateInput{Name: "x", Status: "doing"})
			return err
		}, "TASK_INVALID_STATUS"},
		{"a priority that is not one", func() error {
			_, err := h.svc.Create(ctx(), CreateInput{Name: "x", Priority: "asap"})
			return err
		}, "TASK_INVALID_PRIORITY"},
		{"a due date that is not an instant", func() error {
			_, err := h.svc.Create(ctx(), CreateInput{Name: "x", DueAt: "tomorrow"})
			return err
		}, "TASK_INVALID_TIME"},
		{"a task that is not there", func() error {
			_, err := h.svc.Get(ctx(), GetInput{ID: "t-missing"})
			return err
		}, "TASK_NOT_FOUND"},
		{"a status filter that is not a status", func() error {
			_, err := h.svc.List(ctx(), ListInput{Status: "doing"})
			return err
		}, "TASK_INVALID_STATUS"},
		{"an empty name on update", func() error {
			_, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, Name: ptr("  ")})
			return err
		}, "TASK_INVALID_NAME"},
		{"a priority that is not one, on update", func() error {
			_, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, Priority: ptr(Priority("asap"))})
			return err
		}, "TASK_INVALID_PRIORITY"},
		{"a due date that is not an instant, on update", func() error {
			_, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, DueAt: ptr("soon")})
			return err
		}, "TASK_INVALID_TIME"},
		{"a type outside the taxonomy, on update", func() error {
			_, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, Type: ptr("chore")})
			return err
		}, "TASK_UNKNOWN_TYPE"},
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

// TestWithoutAWorktreePortBranchingSaysSoRatherThanPretending.
func TestWithoutAWorktreePortBranchingSaysSoRatherThanPretending(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Worktrees = nil })
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err == nil {
		t.Fatal("branching succeeded with no way to create a checkout")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "TASK_WORKTREES_UNAVAILABLE") {
		t.Fatalf("error = %v", err)
	}
}

// TestACheckoutThatCannotBeCutIsReported.
func TestACheckoutThatCannotBeCutIsReported(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Worktrees = &worktrees{failWith: errors.New("fatal: invalid reference: main")}
	})
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})

	_, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err == nil {
		t.Fatal("a failed checkout reported success")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "TASK_WORKTREE_FAILED") {
		t.Fatalf("error = %v", err)
	}
	after, err := h.svc.Get(ctx(), GetInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Worktree.Path != "" {
		t.Fatal("the task records a checkout that was never created")
	}
}

// TestABranchNameCanBeGivenRatherThanDerived.
func TestABranchNameCanBeGivenRatherThanDerived(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true, Base: "develop"})

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID, Branch: "hotfix/queue"})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Branch != "hotfix/queue" || tree.Base != "develop" {
		t.Fatalf("tree = %+v", tree)
	}
}

// TestABranchNameFallsBackToTheIdentifier, for a task whose name produced no
// slug of its own.
func TestABranchNameFallsBackToTheIdentifier(t *testing.T) {
	if got := BranchNameFor("", &Task{ID: "t-42"}); got != "aos/t-42" {
		t.Fatalf("branch = %q", got)
	}
	if got := BranchNameFor("feat", &Task{ID: "t-42", Slug: "fix-it"}); got != "feat/fix-it" {
		t.Fatalf("branch = %q", got)
	}
}

// TestAServiceWithNoPlanPortDoesNotBlockReview. The composition root always
// supplies one; this is the shape a test that is about something else gets, and
// it must not pretend the guard ran.
func TestAServiceWithNoPlanPortDoesNotBlockReview(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Plan = nil })
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})
	h.move(t, task.ID, InProgress, InReview)
}

// TestAPlanThatCannotBeReadStopsTheMove, rather than letting the review through
// on a guard that failed.
func TestAPlanThatCannotBeReadStopsTheMove(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})
	h.move(t, task.ID, InProgress)
	h.plan.err = errors.New("the plan directory is unreadable")

	if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: task.ID, Status: InReview}); err == nil {
		t.Fatal("the review guard failed open")
	}
}

// TestAWriteThatFailsIsReported.
func TestAWriteThatFailsIsReported(t *testing.T) {
	h := newHarness(t, func(d *Deps) { d.Repo = brokenRepo{} })

	if _, err := h.svc.Create(ctx(), CreateInput{Name: "x"}); err == nil {
		t.Fatal("a repository that refuses every write reported success")
	}
}

// brokenRepo fails everything, which is what a full disk looks like from here.
type brokenRepo struct{}

func (brokenRepo) Get(context.Context, collections.Key) (*Task, error) {
	return nil, errors.New("no such file")
}
func (brokenRepo) List(context.Context, collections.Query) ([]Task, error) {
	return nil, errors.New("no such directory")
}
func (brokenRepo) Create(context.Context, *Task) error { return errors.New("no space left") }
func (brokenRepo) Update(context.Context, *Task, collections.Version) error {
	return errors.New("no space left")
}
func (brokenRepo) Delete(context.Context, collections.Key) error { return errors.New("read-only") }
