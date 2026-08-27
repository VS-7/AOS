package task

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/fakes"
)

type countingIDs struct{ n int }

func (g *countingIDs) New() string { g.n++; return "t" + strconv.Itoa(g.n) }

// plan is the todo aggregate as the review guard sees it: a count, a list and
// a progress. Nothing here can rewrite the plan it is being judged against.
type plan struct {
	pending map[string][]string
	total   map[string]int
	err     error
}

func (p *plan) CountPending(_ context.Context, taskID string) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	return len(p.pending[taskID]), nil
}

func (p *plan) PendingIDs(_ context.Context, taskID string) ([]string, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.pending[taskID], nil
}

func (p *plan) Progress(_ context.Context, taskID string) (Progress, error) {
	if p.err != nil {
		return Progress{}, p.err
	}
	total := p.total[taskID]
	return Progress{Completed: total - len(p.pending[taskID]), Total: total}, nil
}

// directory resolves an assignee to what it is. The answer decides whether the
// task is dispatched at all.
type directory map[string]ResolvedAssignee

func (d directory) Resolve(_ context.Context, id string) (ResolvedAssignee, error) {
	if got, ok := d[id]; ok {
		return got, nil
	}
	return ResolvedAssignee{ID: id, Type: AssigneeUnknown}, nil
}

type worktrees struct {
	created  []WorktreeSpec
	existing []string
	removed  []string
	failWith error
}

func (w *worktrees) Create(_ context.Context, spec WorktreeSpec) (string, error) {
	if w.failWith != nil {
		return "", w.failWith
	}
	w.created = append(w.created, spec)
	w.existing = append(w.existing, spec.Path)
	return spec.Path, nil
}

func (w *worktrees) Remove(_ context.Context, path string) error {
	w.removed = append(w.removed, path)
	kept := w.existing[:0]
	for _, p := range w.existing {
		if p != path {
			kept = append(kept, p)
		}
	}
	w.existing = kept
	return nil
}

func (w *worktrees) List(context.Context) ([]string, error) {
	return append([]string(nil), w.existing...), nil
}

// setup records that the script ran and under whose policy.
type setup struct {
	ranFor   string
	inDir    string
	script   string
	failWith error
}

func (s *setup) Run(_ context.Context, agentID, dir, script string) error {
	s.ranFor, s.inDir, s.script = agentID, dir, script
	return s.failWith
}

type policy struct {
	worktrees WorktreePolicy
	types     []string
}

func (p policy) Worktrees(context.Context) (WorktreePolicy, error) { return p.worktrees, nil }
func (p policy) TaskTypes(context.Context) ([]string, error)       { return p.types, nil }

type notifier struct {
	events []string
	data   []map[string]any
}

func (n *notifier) TaskChanged(_ context.Context, event string, _ *Task, data map[string]any) {
	n.events = append(n.events, event)
	n.data = append(n.data, data)
}

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

type harness struct {
	svc       *Service
	repo      *fakes.Repo[Task]
	plan      *plan
	worktrees *worktrees
	setup     *setup
	notifier  *notifier
	clock     *clockx.Stepping
}

func newHarness(t *testing.T, opts ...func(*Deps)) *harness {
	t.Helper()
	repo := fakes.NewRepo[Task]("tasks").WithKeyFunc(func(v *Task) collections.Key {
		return collections.Key{"id": v.ID}
	})
	p := &plan{pending: map[string][]string{}, total: map[string]int{}}
	trees := &worktrees{}
	scripts := &setup{}
	notes := &notifier{}
	clock := &clockx.Stepping{At: start, Step: time.Minute}

	deps := Deps{
		Repo:      repo,
		Plan:      p,
		Directory: directory{"atlas": {ID: "atlas", Type: AssigneeAgent, Name: "Atlas"}},
		Worktrees: trees,
		Setup:     scripts,
		Policy: policy{
			worktrees: WorktreePolicy{BranchPrefix: "aos", Limit: 2, DeleteOld: true, Root: "/tmp/wt"},
			types:     []string{"bug", "feature"},
		},
		Notifier: notes,
		Clock:    clock,
		IDs:      &countingIDs{},
	}
	for _, opt := range opts {
		opt(&deps)
	}
	return &harness{
		svc: NewService(deps), repo: repo, plan: p,
		worktrees: trees, setup: scripts, notifier: notes, clock: clock,
	}
}

func ctx() context.Context { return context.Background() }

func (h *harness) create(t *testing.T, in CreateInput) *View {
	t.Helper()
	got, err := h.svc.Create(ctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// move walks a task to a status through the graph, so a test that needs a task
// in_progress does not have to write the intermediate steps every time.
func (h *harness) move(t *testing.T, id string, path ...Status) {
	t.Helper()
	for _, to := range path {
		if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: id, Status: to}); err != nil {
			t.Fatalf("moving %s to %s: %v", id, to, err)
		}
	}
}

// TestTheLifecycleTableIsExhaustive walks all sixty-four pairs. The graph is
// the only description of how work moves, so it is worth asserting whole rather
// than sampling the interesting edges.
func TestTheLifecycleTableIsExhaustive(t *testing.T) {
	allowed := map[Status]map[Status]bool{
		Suggestion: {Backlog: true, Finished: true},
		Backlog:    {Planning: true, Todo: true, Stopped: true},
		Planning:   {Todo: true, Backlog: true, Stopped: true},
		Todo:       {InProgress: true, Backlog: true, Stopped: true},
		InProgress: {InReview: true, Stopped: true, Todo: true},
		Stopped:    {InProgress: true, Todo: true, Backlog: true},
		InReview:   {Finished: true, InProgress: true},
		Finished:   {},
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

// TestFinishedWorkIsNotReopened. The way back is a new task that says what was
// wrong, which leaves a record that reopening does not.
func TestFinishedWorkIsNotReopened(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})
	h.move(t, task.ID, InProgress, InReview, Finished)

	for _, to := range Statuses {
		if to == Finished {
			continue
		}
		if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: task.ID, Status: to}); err == nil {
			t.Fatalf("finished work moved to %s", to)
		}
	}
}

// TestATaskCannotBeCreatedPastTheGuards. Creating one straight into in_progress
// would skip the dependency guard; into in_review, the plan guard.
func TestATaskCannotBeCreatedPastTheGuards(t *testing.T) {
	h := newHarness(t)
	for _, status := range []Status{InProgress, InReview, Stopped, Finished} {
		if _, err := h.svc.Create(ctx(), CreateInput{Name: "shortcut", Status: status}); err == nil {
			t.Fatalf("a task was created directly in %s", status)
		}
	}
	for _, status := range []Status{Suggestion, Backlog, Planning, Todo} {
		if _, err := h.svc.Create(ctx(), CreateInput{Name: "proper", Status: status}); err != nil {
			t.Fatalf("a task could not be created in %s: %v", status, err)
		}
	}
}

// TestUpdateRefusesToWriteStatus. This is the original's prose rule — "use
// set_status for lifecycle moves; never change status via update" — made
// mechanical.
func TestUpdateRefusesToWriteStatus(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})

	if _, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, Status: Finished}); err == nil {
		t.Fatal("status was written through update")
	}
	after, err := h.svc.Get(ctx(), GetInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != Todo {
		t.Fatalf("the task is %s", after.Status)
	}
}

// TestReviewIsBlockedByAnOpenPlan is the master prompt's hardest task rule,
// enforced rather than advised: "only move the task to in_review when all todos
// are finished and validated".
func TestReviewIsBlockedByAnOpenPlan(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Fix the denial pattern", Type: "bug", Status: Todo})
	h.move(t, task.ID, InProgress)

	h.plan.total[task.ID] = 3
	h.plan.pending[task.ID] = []string{"s2", "s3"}

	_, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: task.ID, Status: InReview})
	if err == nil {
		t.Fatal("a task with two open steps reached review")
	}
	if !strings.Contains(err.Error(), "TASK_REVIEW_BLOCKED") {
		t.Fatalf("error = %v", err)
	}

	h.plan.pending[task.ID] = nil
	if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: task.ID, Status: InReview}); err != nil {
		t.Fatalf("a task with a finished plan was still refused: %v", err)
	}
}

// TestWorkDoesNotStartOnUnfinishedDependencies.
func TestWorkDoesNotStartOnUnfinishedDependencies(t *testing.T) {
	h := newHarness(t)
	first := h.create(t, CreateInput{Name: "Land the queue", Status: Todo})
	second := h.create(t, CreateInput{Name: "Land the scheduler", Status: Todo, DependsOn: []string{first.ID}})

	if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: second.ID, Status: InProgress}); err == nil {
		t.Fatal("work started on an unfinished dependency")
	}

	h.move(t, first.ID, InProgress, InReview, Finished)
	if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: second.ID, Status: InProgress}); err != nil {
		t.Fatalf("work would not start after its dependency finished: %v", err)
	}
}

// TestABlockedTaskSaysWhatIsBlockingIt, without being asked twice.
func TestABlockedTaskSaysWhatIsBlockingIt(t *testing.T) {
	h := newHarness(t)
	first := h.create(t, CreateInput{Name: "Land the queue", Status: Todo})
	second := h.create(t, CreateInput{Name: "Land the scheduler", Status: Todo, DependsOn: []string{first.ID}})

	got, err := h.svc.Get(ctx(), GetInput{ID: second.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Blocked) != 1 || got.Blocked[0] != first.ID {
		t.Fatalf("blocked = %v", got.Blocked)
	}
}

// TestADependencyCycleIsRefused, because neither task could ever start and the
// system would have no way to say so later.
func TestADependencyCycleIsRefused(t *testing.T) {
	h := newHarness(t)
	a := h.create(t, CreateInput{Name: "A", Status: Todo})
	b := h.create(t, CreateInput{Name: "B", Status: Todo, DependsOn: []string{a.ID}})

	if _, err := h.svc.Update(ctx(), UpdateInput{ID: a.ID, DependsOn: ptr([]string{b.ID})}); err == nil {
		t.Fatal("a two-task cycle was accepted")
	}
	if _, err := h.svc.Update(ctx(), UpdateInput{ID: a.ID, DependsOn: ptr([]string{a.ID})}); err == nil {
		t.Fatal("a task was made to depend on itself")
	}
}

// TestADependencyThatNoLongerExistsBlocksNothing. Refusing to start the work
// over a dangling reference is the wrong penalty.
func TestADependencyThatNoLongerExistsBlocksNothing(t *testing.T) {
	h := newHarness(t)
	first := h.create(t, CreateInput{Name: "Land the queue", Status: Todo})
	second := h.create(t, CreateInput{Name: "Land the scheduler", Status: Todo, DependsOn: []string{first.ID}})

	if _, err := h.svc.Delete(ctx(), DeleteInput{ID: first.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: second.ID, Status: InProgress}); err != nil {
		t.Fatalf("a dangling dependency blocked the work: %v", err)
	}
}

// TestStoppingWritesTheCheckpointAndResumingConsumesIt. Leaving it would make
// the next stop look like it happened where the previous one did.
func TestStoppingWritesTheCheckpointAndResumingConsumesIt(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})
	h.move(t, task.ID, InProgress)

	if _, err := h.svc.Update(ctx(), UpdateInput{ID: task.ID, Chat: ptr("chat-9")}); err != nil {
		t.Fatal(err)
	}
	h.plan.total[task.ID] = 4
	h.plan.pending[task.ID] = []string{"s3", "s4"}

	out, err := h.svc.SetStatus(ctx(), SetStatusInput{
		ID: task.ID, Status: Stopped, Reason: "waiting on the provider key",
	})
	if err != nil {
		t.Fatal(err)
	}
	cp := out.Task.Checkpoint
	if cp == nil {
		t.Fatal("stopping wrote no checkpoint")
	}
	if cp.ChatID != "chat-9" {
		t.Fatalf("checkpoint chat = %q", cp.ChatID)
	}
	if len(cp.PendingTodoIDs) != 2 {
		t.Fatalf("checkpoint pending = %v", cp.PendingTodoIDs)
	}
	if cp.Progress != (Progress{Completed: 2, Total: 4}) {
		t.Fatalf("checkpoint progress = %+v", cp.Progress)
	}
	if cp.Reason == "" {
		t.Fatal("a run that stopped for no recorded reason is one nobody can decide whether to resume")
	}

	resumed, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: task.ID, Status: InProgress})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Task.Checkpoint != nil {
		t.Fatal("resuming kept the checkpoint of the run that had already ended")
	}
}

// TestOnlyAnAgentAssigneeIsDispatchable. It is the boundary between automated
// and human-owned work, and an unknown assignee falls on the human side: the
// system does not guess who work belongs to.
func TestOnlyAnAgentAssigneeIsDispatchable(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Directory = directory{
			"atlas": {ID: "atlas", Type: AssigneeAgent},
			"u-7":   {ID: "u-7", Type: AssigneeUser},
		}
	})

	cases := map[string]bool{"atlas": true, "u-7": false, "nobody": false, "": false}
	for assigned, want := range cases {
		task := h.create(t, CreateInput{Name: "work for " + assigned, Status: Todo, Assigned: assigned})
		got, err := h.svc.Get(ctx(), GetInput{ID: task.ID})
		if err != nil {
			t.Fatal(err)
		}
		if got.Assignee.Dispatchable() != want {
			t.Fatalf("%q: dispatchable = %v, want %v (type %q)",
				assigned, got.Assignee.Dispatchable(), want, got.Assignee.Type)
		}
	}
}

// TestAnUnknownTaskTypeIsRefused, because the type drives the guidance injected
// into the prompt and a typo would silently drop it.
func TestAnUnknownTaskTypeIsRefused(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Create(ctx(), CreateInput{Name: "x", Type: "bugs"}); err == nil {
		t.Fatal("a task type outside the workspace taxonomy was accepted")
	}
	if _, err := h.svc.Create(ctx(), CreateInput{Name: "x", Type: "bug"}); err != nil {
		t.Fatal(err)
	}
}

// TestABranchIsCutFromThePrefixAndTheSlug, and the setup script runs inside it
// under the assigned agent's policy — not with free rein, as in the original.
func TestABranchIsCutFromThePrefixAndTheSlug(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Policy = policy{
			worktrees: WorktreePolicy{
				BranchPrefix: "aos", Limit: 2, DeleteOld: true,
				Root: "/tmp/wt", OnCreateScript: "go mod download",
			},
			types: []string{"bug", "feature"},
		}
	})
	task := h.create(t, CreateInput{
		Name: "Fix the denial pattern", Type: "bug", Status: Todo,
		Assigned: "atlas", Worktree: true,
	})

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Branch != "aos/fix-the-denial-pattern" {
		t.Fatalf("branch = %q", tree.Branch)
	}
	if tree.Path == "" {
		t.Fatal("the checkout has no path")
	}
	if h.setup.script != "go mod download" || h.setup.inDir != tree.Path {
		t.Fatalf("setup ran %q in %q", h.setup.script, h.setup.inDir)
	}
	if h.setup.ranFor != "atlas" {
		t.Fatalf("the setup script ran under %q, not the assigned agent's policy", h.setup.ranFor)
	}

	// Branching twice returns what exists rather than cutting a second one.
	again, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.created) != 1 || again.Path != tree.Path {
		t.Fatalf("a second checkout was cut: %+v", h.worktrees.created)
	}
}

// TestAFailedSetupScriptLeavesTheCheckoutStanding. Destroying the branch over a
// failed script would lose whatever the script did manage to do.
func TestAFailedSetupScriptLeavesTheCheckoutStanding(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Policy = policy{
			worktrees: WorktreePolicy{BranchPrefix: "aos", Root: "/tmp/wt", OnCreateScript: "make setup"},
		}
		d.Setup = &setup{failWith: errors.New("make: command not found")}
	})
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Assigned: "atlas", Worktree: true})

	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err == nil {
		t.Fatal("a failed setup script was silent")
	}
	if tree == nil || tree.Path == "" {
		t.Fatal("the checkout was destroyed along with the failed script")
	}

	stored, err := h.svc.Get(ctx(), GetInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if stored.Worktree.Path == "" {
		t.Fatal("the task does not record the checkout that exists")
	}
}

// TestTheOldestFinishedCheckoutIsPrunedAndAnActiveOneIsNot.
func TestTheOldestFinishedCheckoutIsPrunedAndAnActiveOneIsNot(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Policy = policy{worktrees: WorktreePolicy{
			BranchPrefix: "aos", Limit: 2, DeleteOld: true, Root: "/tmp/wt",
		}}
	})

	done := h.create(t, CreateInput{Name: "Finished work", Status: Todo, Worktree: true})
	if _, err := h.svc.Branch(ctx(), BranchInput{ID: done.ID}); err != nil {
		t.Fatal(err)
	}
	h.move(t, done.ID, InProgress, InReview, Finished)

	busy := h.create(t, CreateInput{Name: "Active work", Status: Todo, Worktree: true})
	if _, err := h.svc.Branch(ctx(), BranchInput{ID: busy.ID}); err != nil {
		t.Fatal(err)
	}
	h.move(t, busy.ID, InProgress)

	// Two exist and the limit is two, so cutting a third has to make room.
	next := h.create(t, CreateInput{Name: "New work", Status: Todo, Worktree: true})
	if _, err := h.svc.Branch(ctx(), BranchInput{ID: next.ID}); err != nil {
		t.Fatal(err)
	}

	if len(h.worktrees.removed) != 1 {
		t.Fatalf("removed = %v", h.worktrees.removed)
	}
	busyView, err := h.svc.Get(ctx(), GetInput{ID: busy.ID})
	if err != nil {
		t.Fatal(err)
	}
	if busyView.Worktree.Path == "" {
		t.Fatal("the checkout of work in progress was pruned")
	}
	doneView, err := h.svc.Get(ctx(), GetInput{ID: done.ID})
	if err != nil {
		t.Fatal(err)
	}
	if doneView.Worktree.Path != "" {
		t.Fatal("a pruned checkout is still recorded on its task")
	}
}

// TestThePruneRefusesRatherThanTakingAnActiveCheckout.
func TestThePruneRefusesRatherThanTakingAnActiveCheckout(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Policy = policy{worktrees: WorktreePolicy{
			BranchPrefix: "aos", Limit: 1, DeleteOld: true, Root: "/tmp/wt",
		}}
	})
	busy := h.create(t, CreateInput{Name: "Active work", Status: Todo, Worktree: true})
	if _, err := h.svc.Branch(ctx(), BranchInput{ID: busy.ID}); err != nil {
		t.Fatal(err)
	}
	h.move(t, busy.ID, InProgress)

	next := h.create(t, CreateInput{Name: "New work", Status: Todo, Worktree: true})
	_, err := h.svc.Branch(ctx(), BranchInput{ID: next.ID})
	if err == nil {
		t.Fatal("the limit was exceeded rather than reported")
	}
	if !strings.Contains(err.Error(), "TASK_WORKTREE_LIMIT") {
		t.Fatalf("error = %v", err)
	}
	if len(h.worktrees.removed) != 0 {
		t.Fatalf("an active checkout was taken: %v", h.worktrees.removed)
	}
}

// TestAMoveIsAnnounced, which is what a routine with an activity trigger reacts
// to. The payload carries the type, so "when a bug enters in_review" can be a
// filter rather than a second lookup.
func TestAMoveIsAnnounced(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Fix the denial pattern", Type: "bug", Status: Todo})
	h.move(t, task.ID, InProgress, InReview)

	var found map[string]any
	for i, event := range h.notifier.events {
		if event == "status_changed" && h.notifier.data[i]["to"] == "in_review" {
			found = h.notifier.data[i]
		}
	}
	if found == nil {
		t.Fatalf("no in_review announcement in %v", h.notifier.events)
	}
	if found["type"] != "bug" || found["from"] != "in_progress" {
		t.Fatalf("payload = %+v", found)
	}
}

// TestDeleteTakesTheCheckoutWithIt. The checkout lives outside the task
// directory, so the collection cascade cannot reach it.
func TestDeleteTakesTheCheckoutWithIt(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo, Worktree: true})
	tree, err := h.svc.Branch(ctx(), BranchInput{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := h.svc.Delete(ctx(), DeleteInput{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if len(h.worktrees.removed) != 1 || h.worktrees.removed[0] != tree.Path {
		t.Fatalf("removed = %v, want [%s]", h.worktrees.removed, tree.Path)
	}
}

// TestExistsIsTheParentPortTheSubcollectionsHold.
func TestExistsIsTheParentPortTheSubcollectionsHold(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Land the queue", Status: Todo})

	if ok, err := h.svc.Exists(ctx(), task.ID); err != nil || !ok {
		t.Fatalf("exists = %v, %v", ok, err)
	}
	if ok, err := h.svc.Exists(ctx(), "t-missing"); err != nil || ok {
		t.Fatalf("a missing task reported %v, %v — absence is an answer, not a failure", ok, err)
	}
}

// TestAMoveToWhereItAlreadyIsIsAcceptedAndRunsNoGuard, so a retried call is not
// an error and does not fail on a plan that has since changed.
func TestAMoveToWhereItAlreadyIsIsAcceptedAndRunsNoGuard(t *testing.T) {
	h := newHarness(t)
	task := h.create(t, CreateInput{Name: "Ship the runtime", Status: Todo})
	h.move(t, task.ID, InProgress, InReview)
	h.plan.pending[task.ID] = []string{"s1"}

	out, err := h.svc.SetStatus(ctx(), SetStatusInput{ID: task.ID, Status: InReview})
	if err != nil {
		t.Fatalf("re-asserting the current status failed: %v", err)
	}
	if out.From != InReview || out.To != InReview {
		t.Fatalf("out = %+v", out)
	}
}

// A worktree the person cut by hand is not the pruner's to take.
//
// `git worktree list` reports every checkout of the repository, and one nobody
// made a task for is exactly the shape of somebody's own branch with
// uncommitted work in it. The pruner used to call that "the safest thing to
// remove" and remove it with --force. Only what sits under the directory this
// workspace places its own checkouts in is a candidate.
func TestThePruneLeavesAWorktreeTheWorkspaceDidNotPlace(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Policy = policy{worktrees: WorktreePolicy{
			BranchPrefix: "aos", Limit: 1, DeleteOld: true, Root: "/tmp/wt",
		}}
	})
	// Two checkouts git knows about and no task claims: one of ours, left
	// behind by a run that did not finish, and one of the person's own.
	h.worktrees.existing = []string{"/tmp/wt/leftover", "/home/someone/my-own-branch"}

	next := h.create(t, CreateInput{Name: "New work", Status: Todo, Worktree: true})
	if _, err := h.svc.Branch(ctx(), BranchInput{ID: next.ID}); err != nil {
		t.Fatalf("the prune could not make room from its own leftovers: %v", err)
	}
	for _, removed := range h.worktrees.removed {
		if removed == "/home/someone/my-own-branch" {
			t.Fatal("a worktree outside the workspace's own root was removed")
		}
	}
	if len(h.worktrees.removed) != 1 || h.worktrees.removed[0] != "/tmp/wt/leftover" {
		t.Fatalf("removed = %v, want only the leftover under the workspace's root", h.worktrees.removed)
	}
}
