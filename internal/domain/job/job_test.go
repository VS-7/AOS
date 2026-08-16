package job

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
)

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// memQueue is the Queue port in memory. The real one has its own suite beside
// it; what is tested here is the reading and repair the service adds on top.
type memQueue struct {
	jobs   []Job
	failOn string
}

func (q *memQueue) Enqueue(_ context.Context, j Job) (string, error) {
	q.jobs = append(q.jobs, j)
	return j.ID, nil
}

func (q *memQueue) Claim(context.Context, []string, string, time.Duration) (*Job, error) {
	return nil, nil //nolint:nilnil // nothing to claim
}
func (q *memQueue) Heartbeat(context.Context, string, string, time.Duration) error { return nil }
func (q *memQueue) Complete(context.Context, string, json.RawMessage) error        { return nil }
func (q *memQueue) Fail(context.Context, string, error, *time.Duration) error      { return nil }

func (q *memQueue) RecoverStale(context.Context) (int, error) {
	if q.failOn == "recover" {
		return 0, errors.New("the database is locked")
	}
	var n int
	for i := range q.jobs {
		if q.jobs[i].Status == Claimed {
			q.jobs[i].Status = Pending
			n++
		}
	}
	return n, nil
}

func (q *memQueue) Get(_ context.Context, jobID string) (*Job, error) {
	if q.failOn == "get" {
		return nil, errors.New("the database is locked")
	}
	for i := range q.jobs {
		if q.jobs[i].ID == jobID {
			return &q.jobs[i], nil
		}
	}
	return nil, nil //nolint:nilnil // absence is an answer
}

func (q *memQueue) List(_ context.Context, f Filter) ([]Job, error) {
	if q.failOn == "list" {
		return nil, errors.New("the database is locked")
	}
	var out []Job
	for _, j := range q.jobs {
		if f.Queue != "" && j.Queue != f.Queue {
			continue
		}
		if f.Status != "" && j.Status != f.Status {
			continue
		}
		if f.Workspace != "" && j.Workspace != f.Workspace {
			continue
		}
		if f.Kind != "" && j.Kind != f.Kind {
			continue
		}
		out = append(out, j)
	}
	if f.Limit > 0 && f.Limit < len(out) {
		out = out[:f.Limit]
	}
	return out, nil
}

func (q *memQueue) Purge(context.Context, time.Duration) (int, error) {
	if q.failOn == "purge" {
		return 0, errors.New("the database is locked")
	}
	kept := q.jobs[:0]
	var n int
	for _, j := range q.jobs {
		if j.Terminal() {
			n++
			continue
		}
		kept = append(kept, j)
	}
	q.jobs = kept
	return n, nil
}

func (q *memQueue) Close() error { return nil }

func newService(t *testing.T, jobs ...Job) (*Service, *memQueue) {
	t.Helper()
	q := &memQueue{jobs: jobs}
	return NewService(Deps{Queue: q, Clock: clockx.Fixed{At: start}}), q
}

func ctx() context.Context { return context.Background() }

// TestStatsSeparatesABusyQueueFromAnIncident. A claimed job whose lease has
// lapsed is not busy: its worker is gone, and that is the shape of a real
// problem rather than of a loaded machine.
func TestStatsSeparatesABusyQueueFromAnIncident(t *testing.T) {
	live := start.Add(time.Minute)
	lapsed := start.Add(-time.Minute)

	svc, _ := newService(t,
		Job{ID: "j-pending", Queue: QueueChat, Status: Pending},
		Job{ID: "j-live", Queue: QueueChat, Status: Claimed, LeaseUntil: &live},
		Job{ID: "j-stale", Queue: QueueTask, Status: Claimed, LeaseUntil: &lapsed},
		Job{ID: "j-dead", Queue: QueueTask, Status: Dead},
		Job{ID: "j-done", Queue: QueueRoutine, Status: Succeeded},
	)

	out, err := svc.Stats(ctx(), StatsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 5 {
		t.Fatalf("total = %d", out.Total)
	}
	if out.ByStatus["claimed"] != 2 || out.ByQueue[QueueTask] != 2 {
		t.Fatalf("counts = %+v / %+v", out.ByStatus, out.ByQueue)
	}
	if len(out.Stale) != 1 || out.Stale[0] != "j-stale" {
		t.Fatalf("stale = %v — a live claim was reported as an incident", out.Stale)
	}
	if len(out.Dead) != 1 || out.Dead[0] != "j-dead" {
		t.Fatalf("dead = %v", out.Dead)
	}
}

// TestRecoverHandsBackWhatLapsed.
func TestRecoverHandsBackWhatLapsed(t *testing.T) {
	lapsed := start.Add(-time.Minute)
	svc, q := newService(t, Job{ID: "j-1", Queue: QueueChat, Status: Claimed, LeaseUntil: &lapsed})

	out, err := svc.Recover(ctx(), RecoverInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Recovered != 1 {
		t.Fatalf("recovered = %d", out.Recovered)
	}
	if q.jobs[0].Status != Pending {
		t.Fatalf("the job is %s", q.jobs[0].Status)
	}
}

// TestPurgeTakesOnlyTerminalWork.
func TestPurgeTakesOnlyTerminalWork(t *testing.T) {
	svc, q := newService(t,
		Job{ID: "j-done", Status: Succeeded},
		Job{ID: "j-dead", Status: Dead},
		Job{ID: "j-waiting", Status: Pending},
	)

	out, err := svc.Purge(ctx(), PurgeInput{OlderThanDays: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.Removed != 2 || out.OlderThan != "24h0m0s" {
		t.Fatalf("out = %+v", out)
	}
	if len(q.jobs) != 1 || q.jobs[0].ID != "j-waiting" {
		t.Fatalf("left = %+v", q.jobs)
	}
}

// TestListFiltersOnEveryFieldItAdvertises.
func TestListFiltersOnEveryFieldItAdvertises(t *testing.T) {
	svc, _ := newService(t,
		Job{ID: "j-1", Queue: QueueChat, Kind: "turn", Workspace: "alpha", Status: Pending},
		Job{ID: "j-2", Queue: QueueRoutine, Kind: "tick", Workspace: "beta", Status: Dead},
	)

	cases := []struct {
		name string
		in   ListInput
		want int
	}{
		{"everything", ListInput{}, 2},
		{"by queue", ListInput{Queue: QueueChat}, 1},
		{"by status", ListInput{Status: Dead}, 1},
		{"by workspace", ListInput{Workspace: "alpha"}, 1},
		{"by kind", ListInput{Kind: "tick"}, 1},
		{"limited", ListInput{Limit: 1}, 1},
	}
	for _, tc := range cases {
		out, err := svc.List(ctx(), tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if out.Total != tc.want {
			t.Fatalf("%s: total = %d, want %d", tc.name, out.Total, tc.want)
		}
	}
}

// TestGetReadsOneJobAndSaysWhenThereIsNone.
func TestGetReadsOneJobAndSaysWhenThereIsNone(t *testing.T) {
	svc, _ := newService(t, Job{ID: "j-1", Queue: QueueChat, Status: Pending})

	got, err := svc.Get(ctx(), GetInput{ID: "j-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "j-1" {
		t.Fatalf("got %+v", got)
	}
	if _, err := svc.Get(ctx(), GetInput{ID: "j-nothing"}); err == nil {
		t.Fatal("a job that does not exist was read")
	}
}

// TestWithoutAQueueEveryReadSaysWhereToLookInstead. A terminal has no queue;
// the daemon does, and the error says so rather than reporting an empty one.
func TestWithoutAQueueEveryReadSaysWhereToLookInstead(t *testing.T) {
	svc := NewService(Deps{Clock: clockx.Fixed{At: start}})

	calls := map[string]func() error{
		"list":    func() error { _, err := svc.List(ctx(), ListInput{}); return err },
		"get":     func() error { _, err := svc.Get(ctx(), GetInput{ID: "j-1"}); return err },
		"stats":   func() error { _, err := svc.Stats(ctx(), StatsInput{}); return err },
		"recover": func() error { _, err := svc.Recover(ctx(), RecoverInput{}); return err },
		"purge":   func() error { _, err := svc.Purge(ctx(), PurgeInput{}); return err },
	}
	for name, call := range calls {
		err := call()
		if err == nil {
			t.Fatalf("%s reported success with no queue behind it", name)
		}
		got, ok := apperr.As(err)
		if !ok || !strings.HasSuffix(got.Code, "JOB_QUEUE_UNAVAILABLE") {
			t.Fatalf("%s: error = %v", name, err)
		}
	}
}

// TestAQueueThatCannotBeReadIsReportedRatherThanCountedAsEmpty.
func TestAQueueThatCannotBeReadIsReportedRatherThanCountedAsEmpty(t *testing.T) {
	svc, q := newService(t)

	q.failOn = "list"
	if _, err := svc.List(ctx(), ListInput{}); err == nil {
		t.Fatal("an unreadable queue listed as empty")
	}
	if _, err := svc.Stats(ctx(), StatsInput{}); err == nil {
		t.Fatal("an unreadable queue reported statistics")
	}
	q.failOn = "recover"
	if _, err := svc.Recover(ctx(), RecoverInput{}); err == nil {
		t.Fatal("a failed recovery reported success")
	}
	q.failOn = "purge"
	if _, err := svc.Purge(ctx(), PurgeInput{}); err == nil {
		t.Fatal("a failed purge reported success")
	}
}

// TestAStatusThatIsNotOneIsRefused.
func TestAStatusThatIsNotOneIsRefused(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.List(ctx(), ListInput{Status: "running"})
	if err == nil {
		t.Fatal("a status that is not one was accepted")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "JOB_INVALID_STATUS") {
		t.Fatalf("error = %v", err)
	}
}

// TestExhaustedCountsAgainstTheDefaultWhenNoLimitWasSet.
func TestExhaustedCountsAgainstTheDefaultWhenNoLimitWasSet(t *testing.T) {
	cases := []struct {
		job  Job
		want bool
	}{
		{Job{Attempts: 2}, false},
		{Job{Attempts: 3}, true},
		{Job{Attempts: 1, MaxTries: 1}, true},
		{Job{Attempts: 4, MaxTries: 5}, false},
	}
	for _, tc := range cases {
		if got := tc.job.Exhausted(); got != tc.want {
			t.Errorf("%+v exhausted = %v, want %v", tc.job, got, tc.want)
		}
	}
}

// TestTerminalIsWhatWillNotRunAgain.
func TestTerminalIsWhatWillNotRunAgain(t *testing.T) {
	terminal := map[Status]bool{Succeeded: true, Dead: true}
	for _, s := range Statuses {
		if got := (Job{Status: s}).Terminal(); got != terminal[s] {
			t.Errorf("%s terminal = %v, want %v", s, got, terminal[s])
		}
		if !s.Valid() {
			t.Errorf("%s is not in Statuses", s)
		}
	}
	if Status("running").Valid() {
		t.Error("a status that is not one validated")
	}
}

// TestRegisterPublishesTheWholeGroup, and enqueue is not part of it: a queue
// anything can write to is a queue whose contents nobody can attribute.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	svc, _ := newService(t)
	reg := command.NewRegistry()
	Register(reg, svc)

	want := []string{"jobs_get", "jobs_list", "jobs_purge", "jobs_recover", "jobs_stats"}
	got := make([]string, 0, len(want))
	for _, d := range reg.Sorted() {
		got = append(got, d.Key())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for _, d := range reg.Sorted() {
		switch d.Key() {
		case "jobs_purge":
			if !d.Annotations().DestructiveHint {
				t.Error("purging the queue must be announced destructive")
			}
			if d.InRegistry() {
				t.Error("purging the queue is not the agent's to do")
			}
		case "jobs_recover":
			if d.InRegistry() {
				t.Error("recovering the queue is not the agent's to do")
			}
		case "jobs_list", "jobs_get", "jobs_stats":
			if !d.Annotations().ReadOnlyHint {
				t.Errorf("%s must be announced read-only", d.Key())
			}
		}
	}
}
