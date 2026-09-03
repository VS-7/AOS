package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/app"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/ids"
	"github.com/OWNER/aos/internal/domain/activity"
	"github.com/OWNER/aos/internal/domain/agent"
	"github.com/OWNER/aos/internal/domain/event"
	"github.com/OWNER/aos/internal/domain/goal"
	"github.com/OWNER/aos/internal/domain/project"
	"github.com/OWNER/aos/internal/domain/task"
	"github.com/OWNER/aos/internal/domain/workspace"
	"github.com/OWNER/aos/internal/transport/realtime"
)

// The scope tests cover the one thing every live update in the application
// depends on: that an event is published on the channel the window is
// actually subscribed to.
//
// The daemon a desktop or CLI installation runs is started with a workspace
// *path* and no id — AOS_WORKSPACE_ID is a deliberate pin nothing sets there —
// and the workspace itself is registered afterwards, by the window's own
// workspace_introspect. So the id was empty at wiring time, and every
// activity, every collection change and every approval request went out on
// realtime.ChannelFor(""), which no socket ever subscribes to.
//
// The visible result was the whole of "nothing updates by itself": a project
// or task an agent created never appeared, the inbox never moved, and the
// approval dialog never opened — so an approval-gated tool waited out its
// deadline and was denied, and the agent reported it could not do the thing.
//
// Each of these builds the App the way that daemon is built (no pinned id),
// registers the workspace after the fact, and asserts the event arrives on
// the registered workspace's channel.

// collectingSink records every frame the hub sends it.
type collectingSink struct {
	frames chan []byte
}

func newCollectingSink() *collectingSink {
	return &collectingSink{frames: make(chan []byte, 32)}
}

func (s *collectingSink) Send(_ context.Context, payload []byte) error {
	clone := make([]byte, len(payload))
	copy(clone, payload)
	select {
	case s.frames <- clone:
	default:
	}
	return nil
}

func (s *collectingSink) Close() error { return nil }

// await returns the first frame of the given type, or fails.
func (s *collectingSink) await(t *testing.T, eventType string) map[string]any {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case raw := <-s.frames:
			var frame map[string]any
			if err := json.Unmarshal(raw, &frame); err != nil {
				t.Fatalf("a frame was not JSON: %v", err)
			}
			if frame["type"] == eventType {
				return frame
			}
		case <-deadline:
			t.Fatalf("no %q frame arrived on the workspace channel", eventType)
		}
	}
}

// listen subscribes a collecting sink to one channel, the way the WebSocket
// handler does — Hub.Subscribe runs the delivery loop itself and returns only
// when the subscription ends, so it belongs on a goroutine of its own.
//
// It returns once *this* subscriber is actually registered, so a publish made
// by the caller's next line cannot be missed.
//
// Both waits are needed, and each covers a case the other does not. The
// subtests below share one app and one channel, and each cancels its
// subscription on cleanup — so when the next one starts, the previous sink
// may still be unwinding. Waiting only for "somebody is subscribed" would be
// satisfied by that departing sink and return before this one had joined.
// Waiting only for the count to grow past what it was fails the other way:
// the count goes 1 → 0 → 1 as one leaves and one arrives, so it never
// exceeds the 1 that was read at the start, and the wait times out.
//
// So: wait for the channel to be empty, then join it, then wait for the
// arrival that can only be this one. Every caller here has a single listener
// at a time, which is what makes the first wait terminate.
func listen(t *testing.T, a *app.App, channel string) *collectingSink {
	t.Helper()
	await(t, "the channel never emptied", func() bool {
		return a.Events.Subscribers(channel) == 0
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	sink := newCollectingSink()
	go a.Events.Subscribe(ctx, channel, sink)

	await(t, "the sink never joined "+channel, func() bool {
		return a.Events.Subscribers(channel) > 0
	})
	return sink
}

// await polls until cond holds, or fails the test saying what never happened.
func await(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// unpinnedApp builds the App a desktop or CLI daemon builds: a workspace root
// and no AOS_WORKSPACE_ID, with the workspace registered afterwards — which is
// the order the real thing happens in, and the order that broke.
func unpinnedApp(t *testing.T) (*app.App, string) {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()

	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome: home,
		})),
		WorkspaceRoot: root,
		Clock:         clockx.System{},
		IDs:           &ids.Sequence{Prefix: "id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	out, err := a.Workspaces.Introspect(context.Background(), workspace.IntrospectInput{Path: root})
	if err != nil {
		t.Fatalf("the workspace could not be registered: %v", err)
	}
	if out.Workspace.ID == "" {
		t.Fatal("the registered workspace has no id")
	}
	return a, out.Workspace.ID
}

func TestAnActivityReachesTheRegisteredWorkspaceChannel(t *testing.T) {
	a, id := unpinnedApp(t)
	ctx := context.Background()

	sink := listen(t, a, realtime.ChannelFor(id))

	if _, err := a.Activities.Publish(ctx, activity.PublishInput{
		Namespace: "task", Event: "created", Title: "New task: probe",
	}); err != nil {
		t.Fatal(err)
	}

	frame := sink.await(t, realtime.EventActivity)
	if frame["workspace"] != id {
		t.Errorf("the frame names workspace %q, want %q", frame["workspace"], id)
	}
}

func TestATaskChangeReachesTheRegisteredWorkspaceChannel(t *testing.T) {
	a, id := unpinnedApp(t)
	ctx := context.Background()

	sink := listen(t, a, realtime.ChannelFor(id))

	if _, err := a.Tasks.Create(ctx, task.CreateInput{Name: "Probe the channel"}); err != nil {
		t.Fatal(err)
	}

	sink.await(t, realtime.EventActivity)
}

// The inbox reads activities, and routine triggers only ever react to them —
// `collection.changed` tells a cache what to refetch and carries no title, so
// it can show nothing and trigger nothing. Project, goal and agent published
// none, although the original app emitted created/updated/deleted for all
// three.
func TestCreatingAProjectGoalOrAgentIsAnActivity(t *testing.T) {
	a, id := unpinnedApp(t)
	ctx := context.Background()

	for _, probe := range []struct {
		what      string
		namespace string
		create    func() error
	}{
		{"project", "project", func() error {
			_, err := a.Projects.Create(ctx, project.CreateInput{Name: "Probe project"})
			return err
		}},
		{"goal", "goal", func() error {
			_, err := a.Goals.Create(ctx, goal.CreateInput{Title: "Probe goal"})
			return err
		}},
		{"agent", "agent", func() error {
			_, err := a.Agents.Create(ctx, agent.CreateInput{Name: "Probe agent"})
			return err
		}},
	} {
		t.Run(probe.what, func(t *testing.T) {
			sink := listen(t, a, realtime.ChannelFor(id))
			if err := probe.create(); err != nil {
				t.Fatal(err)
			}

			frame := sink.await(t, realtime.EventActivity)
			data, ok := frame["data"].(map[string]any)
			if !ok {
				t.Fatalf("the activity carries no object payload: %v", frame["data"])
			}
			// The namespace is the interface's own feature name, because
			// `lib/realtime.ts` invalidates the query key `[namespace]`.
			if data["namespace"] != probe.namespace {
				t.Errorf("namespace = %v, want %q", data["namespace"], probe.namespace)
			}
			if data["event"] != "created" {
				t.Errorf("event = %v, want \"created\"", data["event"])
			}
			if title, _ := data["title"].(string); title == "" {
				t.Error("the activity has no title, so the inbox has nothing to show")
			}
		})
	}
}

func TestAProjectWriteReachesTheRegisteredWorkspaceChannel(t *testing.T) {
	a, id := unpinnedApp(t)
	ctx := context.Background()

	sink := listen(t, a, realtime.ChannelFor(id))

	if _, err := a.Projects.Create(ctx, project.CreateInput{Name: "Probe project"}); err != nil {
		t.Fatal(err)
	}

	frame := sink.await(t, realtime.EventCollectionChanged)
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("the collection.changed frame carries no object payload: %v", frame["data"])
	}
	// The wire names are the contract the interface reads. Go field names
	// (Collection/Op/Path) reach the browser as keys nothing looks for.
	if data["collection"] != "projects" {
		t.Errorf("data.collection = %v, want \"projects\"", data["collection"])
	}
	if data["op"] != "create" {
		t.Errorf("data.op = %v, want \"create\"", data["op"])
	}
	if data["path"] == nil {
		t.Error("data.path is absent; the file explorer filters on it")
	}
}

// AOS_APPROVAL_DEADLINE was declared in env.settings and documented in
// docs/system/08-operacao.md, and nothing read it: every request waited the
// domain's two minutes whatever an operator set.
func TestTheApprovalDeadlineIsTheOneThatWasConfigured(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()

	a, err := app.New(app.Options{
		Env: env.New(env.Map(map[string]string{
			env.KeyHome:             home,
			env.KeyApprovalDeadline: "300ms",
		})),
		WorkspaceRoot: root,
		Clock:         clockx.System{},
		IDs:           &ids.Sequence{Prefix: "id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	started := time.Now()
	res, err := a.Approvals.RequestApproval(context.Background(), event.ApprovalRequest{
		ToolName: "instructions_create", Reason: "nobody is going to answer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Approved {
		t.Error("an unanswered request was approved")
	}
	if waited := time.Since(started); waited > 30*time.Second {
		t.Errorf("waited %s for a 300ms deadline", waited)
	}
}

func TestAPendingApprovalReachesTheRegisteredWorkspaceChannel(t *testing.T) {
	a, id := unpinnedApp(t)
	ctx := context.Background()

	sink := listen(t, a, realtime.ChannelFor(id))

	// The request waits for a person, so it is made from a goroutine and
	// answered from this one — which is the whole shape of ADR-0007, and the
	// shape that silently did not work: nothing was published where anybody
	// could see it, so every such call waited out its deadline.
	done := make(chan event.ApprovalResult, 1)
	go func() {
		res, err := a.Approvals.RequestApproval(ctx, event.ApprovalRequest{
			ToolName: "instructions_create",
			Reason:   "the hook asked",
			Deadline: 5 * time.Second,
		})
		if err != nil {
			t.Errorf("the approval request failed: %v", err)
		}
		done <- res
	}()

	frame := sink.await(t, realtime.EventApprovalRequest)
	if frame["workspace"] != id {
		t.Errorf("the frame names workspace %q, want %q", frame["workspace"], id)
	}
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("the approval frame carries no object payload: %v", frame["data"])
	}
	requestID, _ := data["id"].(string)
	if requestID == "" {
		t.Fatal("the approval frame carries no request id, so no dialog could answer it")
	}

	if !a.Approvals.Decide(requestID, event.ApprovalResult{
		Approved: true, Reason: "the test says yes",
	}) {
		t.Fatal("the pending request could not be answered")
	}

	select {
	case res := <-done:
		if !res.Approved {
			t.Error("the answered request came back denied")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the approval request never returned")
	}
}

// A call made from inside a registered repository addresses *that* workspace.
//
// The routing only ever read the workspace id from the header, so a terminal
// (or a coding agent over MCP) standing in repository B silently addressed
// the daemon's primary workspace: `aos tasks list` there listed repository
// A's tasks and said nothing was missing. The audit that fixed which
// directory `workspace_introspect` registers left this half undone —
// X-Working-Dir arrived, and only that one command read it.
func TestACallIsRoutedToTheWorkspaceItsDirectoryBelongsTo(t *testing.T) {
	a, primary := unpinnedApp(t)
	ctx := context.Background()

	second := t.TempDir()
	registered, err := a.Workspaces.Introspect(ctx, workspace.IntrospectInput{Path: second})
	if err != nil {
		t.Fatal(err)
	}
	if registered.Workspace.ID == primary {
		t.Fatal("the second directory registered as the first workspace")
	}

	// A task written from inside the second directory, naming no workspace.
	from := identity.With(ctx, identity.Identity{WorkingDir: second})
	invokeIn(from, t, a, "tasks_create", `{"name":"only in the second","_reasoning":"probe"}`)

	// It is there.
	raw := invokeIn(from, t, a, "tasks_list", `{"_reasoning":"probe"}`)
	if !strings.Contains(string(raw), "only in the second") {
		t.Errorf("the task is not in the workspace it was created from: %s", raw)
	}

	// And not in the primary one, which is what used to receive it.
	fromPrimary := identity.With(ctx, identity.Identity{WorkspaceID: primary})
	raw = invokeIn(fromPrimary, t, a, "tasks_list", `{"_reasoning":"probe"}`)
	if strings.Contains(string(raw), "only in the second") {
		t.Errorf("the task landed in the primary workspace: %s", raw)
	}
}
