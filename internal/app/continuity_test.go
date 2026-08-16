package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/comment"
	"github.com/OWNER/aos/internal/domain/job"
	"github.com/OWNER/aos/internal/domain/memory"
	"github.com/OWNER/aos/internal/domain/routine"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/todo"
	"github.com/OWNER/aos/internal/runtime/agentloop"
	"github.com/OWNER/aos/internal/runtime/providers/fake"
	"github.com/OWNER/aos/internal/runtime/subconscious"
)

// TestTheDeliveryOfPhaseSix is the phase in one test.
//
// A routine fires on its own, the agent it belongs to takes a task from todo to
// in_review without anybody in the conversation, it reports progress where a
// person will find it, and the background observer forms a memory from the run
// that nobody asked it to form. Everything below the provider is the real
// thing: the queue, the plan, the guards, the activity log.
func TestTheDeliveryOfPhaseSix(t *testing.T) {
	a, root := conversing(t)
	ctx := agentCtx()

	if err := os.WriteFile(filepath.Join(root, "BUG.md"),
		[]byte("# The denial pattern\n\nA path glob stops at the separator.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The work, with a plan. The plan is what the review guard counts, so this
	// is the thing that makes in_review mean something later.
	work, err := a.Tasks.Create(ctx, task.CreateInput{
		Name: "Fix the denial pattern", Type: "bug", Status: task.Todo,
		Assigned: "atlas", Summary: "The sandbox matches command lines with a path glob.",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"Reproduce it", "Fix it"} {
		if _, err := a.Todos.Create(ctx, todo.CreateInput{Task: work.ID, Title: step}); err != nil {
			t.Fatal(err)
		}
	}

	// The routine that does the work. It reacts to an activity, which is how
	// reactive automation is built: when a bug is assigned, go and do it.
	made, err := a.Routines.Create(ctx, routine.CreateInput{
		Agent: "atlas",
		Name:  "Work the assigned bug",
		Triggers: []routine.TriggerInput{{
			Type: routine.Activity, Namespace: "task", Event: "status_changed",
			Filters: []routine.Filter{
				{Field: "to", Operator: routine.OpEq, Value: "in_progress"},
				{Field: "type", Operator: routine.OpEq, Value: "bug"},
			},
		}},
		Scope:   routine.Scope{},
		Content: "Read the bug, finish each step of the plan with evidence, comment on what you did, and move the task to review.",
	})
	if err != nil {
		t.Fatal(err)
	}

	// What the agent does when the routine fires. Read the bug, close both
	// steps with evidence, report in a comment, move to review.
	provider := play(t,
		fake.Step{
			Reasoning: "read the report before touching anything",
			Calls: []agentloop.ToolCall{
				fake.Call("c1", "Read", map[string]any{
					"file_path":  "BUG.md",
					"_reasoning": "the task points at this file",
				}),
			},
			Usage: agentloop.Usage{Input: 900, Output: 30},
		},
		fake.Step{
			Calls: []agentloop.ToolCall{
				fake.Call("c2", "todos_set-status", map[string]any{
					"task": work.ID, "id": stepID(t, a, work.ID, 0), "status": "finished",
					"evidence":   "the new test fails before the fix",
					"_reasoning": "the step is done and the evidence is what proves it",
				}),
				fake.Call("c3", "todos_set-status", map[string]any{
					"task": work.ID, "id": stepID(t, a, work.ID, 1), "status": "finished",
					"evidence":   "matchLine spans the separator; go test passes",
					"_reasoning": "the fix landed",
				}),
			},
			Usage: agentloop.Usage{Input: 1100, Output: 60},
		},
		fake.Step{
			Calls: []agentloop.ToolCall{
				fake.Call("c4", "comments_create", map[string]any{
					"task":       work.ID,
					"body":       "Reproduced and fixed. The glob stopped at the separator; the line matcher now spans it.",
					"_reasoning": "nobody is in the chat, so the progress goes where it can be found",
				}),
			},
			Usage: agentloop.Usage{Input: 1300, Output: 40},
		},
		fake.Step{
			Calls: []agentloop.ToolCall{
				fake.Call("c5", "tasks_set-status", map[string]any{
					"id": work.ID, "status": "in_review",
					"_reasoning": "both steps are finished with evidence",
				}),
			},
			Usage: agentloop.Usage{Input: 1500, Output: 30},
		},
		fake.Step{
			Text:  "Both steps are done and the task is in review.",
			Usage: agentloop.Usage{Input: 1700, Output: 20},
		},
	)

	// What the background observer decides is worth remembering. Nothing in the
	// script above asks for this: the agent never called memories_store.
	observer := observing(t, fake.Step{
		Text: `{"guidance":"","drafts":[{
			"title":"Path globs stop at the separator",
			"description":"A denial pattern written as a path glob never matches a command line that contains a path.",
			"content":"Match a command line with a spanning wildcard.",
			"category":"learning","confidence":0.95,
			"scopes":["internal/runtime/sandbox/**"]
		}]}`,
	})

	// Starting the work publishes the activity the routine is waiting for, and
	// the routine fires from inside that publication.
	if _, err := a.Tasks.SetStatus(ctx, task.SetStatusInput{
		ID: work.ID, Status: task.InProgress,
	}); err != nil {
		t.Fatal(err)
	}

	// 1. The routine ran, and it ran on its own.
	if provider.Steps() != 5 {
		t.Fatalf("the agent took %d steps, want 5", provider.Steps())
	}
	history, err := a.Routines.Runs(ctx, routine.RunsInput{Agent: "atlas", ID: made.Routine.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Runs) != 1 {
		t.Fatalf("the routine recorded %d runs", len(history.Runs))
	}
	run := history.Runs[0]
	if run.Status != routine.RunSucceeded {
		t.Fatalf("the run is %s: %s", run.Status, run.Error)
	}
	if run.Trigger != routine.Activity || run.ChatID == "" {
		t.Fatalf("the run does not point at what caused it or where it happened: %+v", run)
	}

	// 2. The task reached in_review, through the guard rather than around it.
	after, err := a.Tasks.Get(ctx, task.GetInput{ID: work.ID})
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != task.InReview {
		t.Fatalf("the task is %s", after.Status)
	}
	if after.Progress != (task.Progress{Completed: 2, Total: 2}) {
		t.Fatalf("progress = %+v", after.Progress)
	}

	// 3. The evidence is on the plan, and the progress report is on the task.
	plan, err := a.Todos.List(ctx, todo.ListInput{Task: work.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.Todos {
		if step.Evidence == "" {
			t.Fatalf("%q was finished with no evidence", step.Title)
		}
	}
	discussion, err := a.Comments.List(ctx, comment.ListInput{Task: work.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(discussion.Comments) != 1 {
		t.Fatalf("the task has %d comments", len(discussion.Comments))
	}
	if discussion.Comments[0].Author != "atlas" || discussion.Comments[0].AuthorType != "agent" {
		t.Fatalf("the comment is attributed to %q/%q",
			discussion.Comments[0].Author, discussion.Comments[0].AuthorType)
	}

	// 4. A memory formed without the agent asking for one.
	waitFor(t, "the background observer to store a memory", func() bool {
		return observer.Steps() > 0 && memoryCount(t, a) > 0
	})
	recalled, err := a.Memories.Recall(ctx, memory.RecallInput{Agent: "atlas"})
	if err != nil {
		t.Fatal(err)
	}
	var formed *memory.Memory
	for i := range recalled.Memories {
		if strings.Contains(recalled.Memories[i].Title, "Path globs") {
			formed = &recalled.Memories[i]
		}
	}
	if formed == nil {
		t.Fatalf("the observer's memory is not in the graph: %+v", recalled.Memories)
	}
	if formed.Category != memory.CatLearning || formed.Confidence != 0.95 {
		t.Fatalf("the memory came back changed: %+v", formed)
	}

	// And it is a file, in the agent's own directory, like every other memory.
	path := filepath.Join(root, ".aos", "agents", "atlas", "memories", formed.ID+".memory.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the memory is not on disk: %v", err)
	}

	// 5. The whole thing is in the activity log, in order.
	log, err := a.Activities.List(ctx, activity.ListInput{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, entry := range log.Activities {
		seen[entry.Namespace+"."+entry.Event] = true
	}
	for _, want := range []string{"task.created", "task.status_changed", "routine.fired"} {
		if !seen[want] {
			t.Fatalf("the log does not record %s: %v", want, keysOf(seen))
		}
	}
}

// TestADisabledRoutineDoesNotReactToAnything, so switching one off is a real
// boundary rather than a label.
func TestADisabledRoutineDoesNotReactToAnything(t *testing.T) {
	a, _ := conversing(t)
	ctx := agentCtx()

	if _, err := a.Routines.Create(ctx, routine.CreateInput{
		Agent: "atlas", Name: "Off", Status: routine.Disabled,
		Triggers: []routine.TriggerInput{{Type: routine.Activity, Namespace: "task"}},
		Content:  "This should never run.",
	}); err != nil {
		t.Fatal(err)
	}
	provider := play(t, fake.Step{Text: "I should not be here."})

	if _, err := a.Tasks.Create(ctx, task.CreateInput{Name: "Anything", Status: task.Todo}); err != nil {
		t.Fatal(err)
	}
	if provider.Steps() != 0 {
		t.Fatalf("a disabled routine ran %d steps", provider.Steps())
	}
}

// TestAQueuedTurnIsRunByTheWorker. It is the other half of autonomy: work that
// outlives the request that asked for it.
func TestAQueuedTurnIsRunByTheWorker(t *testing.T) {
	a, _ := conversing(t)
	ctx := agentCtx()

	if a.Queue == nil || a.Worker == nil {
		t.Fatal("the installation has no queue")
	}
	provider := play(t, fake.Step{Text: "Answered from the queue."})

	chatID := mustChat(t, a)
	// Post rather than Send: Send would dispatch the turn immediately, and this
	// test is about the queue running it later.
	sent, err := a.Chats.Post(ctx, chatID, "answer this later")
	if err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]string{
		"ChatID": chatID, "MessageID": sent.ID, "AgentID": "atlas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Queue.Enqueue(ctx, job.Job{
		ID: "j-queued", Queue: job.QueueChat, Kind: "aos.turn", Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := a.Worker.Start(runCtx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = a.Worker.Stop(stopCtx)
	}()

	waitFor(t, "the queued turn to finish", func() bool {
		got, err := a.Queue.Get(context.Background(), "j-queued")
		return err == nil && got != nil && got.Status == job.Succeeded
	})
	if provider.Steps() == 0 {
		t.Fatal("the worker marked the job done without running the turn")
	}
}

// stepID reads the identifier of the nth step of a plan, so the script can name
// what it is closing rather than assuming a sequence.
func stepID(t *testing.T, a *app.App, taskID string, n int) string {
	t.Helper()
	plan, err := a.Todos.List(agentCtx(), todo.ListInput{Task: taskID})
	if err != nil {
		t.Fatal(err)
	}
	if n >= len(plan.Todos) {
		t.Fatalf("the plan has %d steps and the script wants %d", len(plan.Todos), n+1)
	}
	return plan.Todos[n].ID
}

func memoryCount(t *testing.T, a *app.App) int {
	t.Helper()
	got, err := a.Memories.Recall(agentCtx(), memory.RecallInput{Agent: "atlas"})
	if err != nil {
		return 0
	}
	return got.Total
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// waitFor polls rather than sleeping a fixed amount: the background observation
// is detached by design, and the test must not encode how fast it is.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// compile-time proof that the observer the runner holds is the one this package
// builds, rather than something that merely happens to have the method.
var _ interface {
	Schedule(context.Context, subconscious.Input)
} = (*subconscious.Observer)(nil)
