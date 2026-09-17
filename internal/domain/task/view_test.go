package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func joined(states []Status) string {
	out := make([]string, 0, len(states))
	for _, s := range states {
		out = append(out, string(s))
	}
	return strings.Join(out, ",")
}

// TestAViewSaysWhereTheTaskCanMoveNext. The desktop offered all eight statuses
// in every picker and on every kanban column, and learned the graph only from
// the refusal: a Start button on a planning task, a drop on Finished from
// backlog. The graph is published with the task so a reader offers only the
// moves that exist, from the one table that decides them.
func TestAViewSaysWhereTheTaskCanMoveNext(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Planning})
	if got := joined(task.NextStates); got != "todo,backlog,stopped" {
		t.Fatalf("a planning task can move to %q", got)
	}

	h.move(t, task.ID, Todo)
	got, err := h.svc.Get(ctx(), GetInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if joined(got.NextStates) != joined(Todo.nextStatuses()) {
		t.Fatalf("get answered %q for a todo task", joined(got.NextStates))
	}

	listed, err := h.svc.List(ctx(), ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tasks) != 1 || joined(listed.Tasks[0].NextStates) != "in_progress,backlog,stopped" {
		t.Fatalf("list answered %+v", listed.Tasks)
	}
}

// TestFinishedWorkPublishesNoMoveRatherThanNone. An absent list reads as "not
// told", which a reader falls back from; an empty one is the answer.
func TestFinishedWorkPublishesNoMoveRatherThanNone(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})
	h.move(t, task.ID, InProgress, InReview, Finished)

	got, err := h.svc.Get(ctx(), GetInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"nextStates":[]`) {
		t.Fatalf("a finished task serialised as %s", raw)
	}
}

// TestACheckoutIsCutOnTheBranchTheTaskWasCreatedWith. The create dialog asks
// for a branch name, and the input had no field for it: the name was dropped on
// the way in and the checkout came out on the generated one.
func TestACheckoutIsCutOnTheBranchTheTaskWasCreatedWith(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{
		Name: "Fix the denial pattern", Status: Todo,
		Worktree: true, Base: "develop", Branch: " feature/denial ",
	})
	if task.Worktree.Branch != "feature/denial" || task.Worktree.Base != "develop" {
		t.Fatalf("worktree = %+v", task.Worktree)
	}

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Branch != "feature/denial" {
		t.Fatalf("the checkout was cut on %q", tree.Branch)
	}
}
