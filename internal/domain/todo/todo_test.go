package todo

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/fakes"
)

type countingIDs struct{ n int }

func (g *countingIDs) New() string { g.n++; return "s" + strconv.Itoa(g.n) }

// existingParent says yes to one task and no to everything else, which is what
// the Parent port is for: a step cannot hang off a task that is not there.
type existingParent map[string]bool

func (p existingParent) Exists(_ context.Context, taskID string) (bool, error) {
	return p[taskID], nil
}

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newService(t *testing.T) (*Service, *fakes.Repo[Todo]) {
	t.Helper()
	repo := fakes.NewRepo[Todo]("todos").WithKeyFunc(func(v *Todo) collections.Key {
		return collections.Key{"taskId": v.TaskID, "id": v.ID}
	})
	return NewService(Deps{
		Repo:   repo,
		Parent: existingParent{"t-1": true},
		Clock:  &clockx.Stepping{At: start, Step: time.Minute},
		IDs:    &countingIDs{},
	}), repo
}

func ctx() context.Context { return context.Background() }

func mustCreate(t *testing.T, svc *Service, title string) *Todo {
	t.Helper()
	got, err := svc.Create(ctx(), CreateInput{Task: "t-1", Title: title})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestAStepCannotHangOffATaskThatIsNotThere.
func TestAStepCannotHangOffATaskThatIsNotThere(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.Create(ctx(), CreateInput{Task: "t-missing", Title: "step"}); err == nil {
		t.Fatal("a step was created under a task that does not exist")
	}
}

// TestAPlanWrittenOneCallAtATimeKeepsItsOrder. Without the assignment every
// step would land at zero and the plan would read in whatever order the
// filesystem returned.
func TestAPlanWrittenOneCallAtATimeKeepsItsOrder(t *testing.T) {
	svc, _ := newService(t)
	for _, title := range []string{"reproduce", "fix", "verify"} {
		mustCreate(t, svc, title)
	}

	out, err := svc.List(ctx(), ListInput{Task: "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"reproduce", "fix", "verify"}
	for i, step := range out.Todos {
		if step.Title != want[i] {
			t.Fatalf("position %d is %q, want %q", i, step.Title, want[i])
		}
		if step.Order != i+1 {
			t.Fatalf("%q has order %d", step.Title, step.Order)
		}
	}
}

// TestTheLifecycleTableIsExhaustive: every pair of states is either allowed by
// the graph or refused, and the test walks all twenty-five.
func TestTheLifecycleTableIsExhaustive(t *testing.T) {
	allowed := map[Status]map[Status]bool{
		Pending:    {InProgress: true, Blocked: true, Finished: true, Skipped: true},
		InProgress: {Blocked: true, Finished: true, Pending: true, Skipped: true},
		Blocked:    {InProgress: true, Pending: true, Skipped: true},
		Finished:   {Pending: true, InProgress: true},
		Skipped:    {Pending: true},
	}
	for _, from := range Statuses {
		for _, to := range Statuses {
			want := allowed[from][to]
			if got := from.CanMoveTo(to); got != want {
				t.Errorf("%s → %s: allowed = %v, want %v", from, to, got, want)
			}
		}
	}
}

// TestAnInvalidMoveIsRefusedAndSaysWhatWasPossible.
func TestAnInvalidMoveIsRefusedAndSaysWhatWasPossible(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "reproduce")

	if _, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: step.ID, Status: Finished}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: step.ID, Status: Blocked}); err == nil {
		t.Fatal("a finished step was moved straight to blocked")
	}
}

// TestUpdateRefusesToWriteStatus. A field that silently did nothing would be
// worse: the caller would read success and believe the step had moved.
func TestUpdateRefusesToWriteStatus(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "reproduce")

	if _, err := svc.Update(ctx(), UpdateInput{Task: "t-1", ID: step.ID, Status: Finished}); err == nil {
		t.Fatal("status was written through update")
	}
	after, err := svc.Get(ctx(), GetInput{Task: "t-1", ID: step.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != Pending {
		t.Fatalf("the step is %s", after.Status)
	}
}

// TestFinishingWithoutEvidenceWarnsAndStillHappens. Not every step is
// verifiable, and a system that refuses to record an honest one teaches people
// to write dishonest evidence.
func TestFinishingWithoutEvidenceWarnsAndStillHappens(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "decide the approach")

	out, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: step.ID, Status: Finished})
	if err != nil {
		t.Fatal(err)
	}
	if out.Todo.Status != Finished {
		t.Fatalf("the step is %s", out.Todo.Status)
	}
	if out.Warning == "" {
		t.Fatal("finishing with no evidence said nothing about it")
	}

	withEvidence := mustCreate(t, svc, "verify")
	out, err = svc.SetStatus(ctx(), SetStatusInput{
		Task: "t-1", ID: withEvidence.ID, Status: Finished,
		Evidence: "go test ./internal/domain/todo passes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Warning != "" {
		t.Fatalf("a step finished with evidence still warned: %q", out.Warning)
	}
	if out.Todo.Evidence == "" {
		t.Fatal("the evidence was not recorded")
	}
}

// TestCountPendingIsWhatTheReviewGuardReads. A skipped step counts as settled:
// a step that stopped applying does not block a review.
func TestCountPendingIsWhatTheReviewGuardReads(t *testing.T) {
	svc, _ := newService(t)
	a := mustCreate(t, svc, "reproduce")
	b := mustCreate(t, svc, "fix")
	c := mustCreate(t, svc, "document")

	pending, err := svc.CountPending(ctx(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if pending != 3 {
		t.Fatalf("pending = %d", pending)
	}

	for _, move := range []struct {
		id     string
		status Status
	}{{a.ID, Finished}, {b.ID, Finished}, {c.ID, Skipped}} {
		if _, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: move.id, Status: move.status}); err != nil {
			t.Fatal(err)
		}
	}

	pending, err = svc.CountPending(ctx(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("pending = %d after two finished and one skipped", pending)
	}

	progress, err := svc.Progress(ctx(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if progress != (Progress{Completed: 3, Total: 3}) {
		t.Fatalf("progress = %+v", progress)
	}
}

// TestPendingIDsAreWhatACheckpointRecords.
func TestPendingIDsAreWhatACheckpointRecords(t *testing.T) {
	svc, _ := newService(t)
	done := mustCreate(t, svc, "reproduce")
	open := mustCreate(t, svc, "fix")

	if _, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: done.ID, Status: Finished}); err != nil {
		t.Fatal(err)
	}
	ids, err := svc.PendingIDs(ctx(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != open.ID {
		t.Fatalf("pending = %v, want [%s]", ids, open.ID)
	}
}

// TestDeleteRemovesOneStep.
func TestDeleteRemovesOneStep(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "reproduce")

	if _, err := svc.Delete(ctx(), DeleteInput{Task: "t-1", ID: step.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx(), GetInput{Task: "t-1", ID: step.ID}); err == nil {
		t.Fatal("the deleted step is still readable")
	}
	if _, err := svc.Delete(ctx(), DeleteInput{Task: "t-1", ID: step.ID}); err == nil {
		t.Fatal("deleting it twice reported success")
	}
}

// TestEveryCommandNeedsTheParent, because a step identifier means nothing
// without the task it belongs to.
func TestEveryCommandNeedsTheParent(t *testing.T) {
	svc, _ := newService(t)
	if _, err := svc.List(ctx(), ListInput{}); err == nil {
		t.Fatal("a plan was listed without naming its task")
	}
	if _, err := svc.Get(ctx(), GetInput{ID: "s1"}); err == nil {
		t.Fatal("a step was read without naming its task")
	}
}

// TestAMoveToWhereItAlreadyIsIsAcceptedAndChangesNothing, so a retried call is
// not an error.
func TestAMoveToWhereItAlreadyIsIsAcceptedAndChangesNothing(t *testing.T) {
	svc, _ := newService(t)
	step := mustCreate(t, svc, "reproduce")

	out, err := svc.SetStatus(ctx(), SetStatusInput{Task: "t-1", ID: step.ID, Status: Pending})
	if err != nil {
		t.Fatal(err)
	}
	if out.From != Pending || out.To != Pending {
		t.Fatalf("out = %+v", out)
	}
}
