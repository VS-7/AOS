package sqlitequeue_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/sqlitequeue"
	"github.com/OWNER/aos/internal/domain/job"
)

// clock is a settable time source, so lease expiry is tested by moving the
// clock rather than by sleeping.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func open(t *testing.T) (*sqlitequeue.Queue, *clock) {
	t.Helper()
	c := &clock{at: start}
	// A file rather than :memory:, because the shared-cache in-memory database
	// is process-global and two tests running in parallel would see each
	// other's jobs.
	q, err := sqlitequeue.Open(sqlitequeue.Options{
		Path:  filepath.Join(t.TempDir(), "jobs.sqlite"),
		Clock: c.now,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	return q, c
}

func ctx() context.Context { return context.Background() }

func enqueue(t *testing.T, q *sqlitequeue.Queue, id, queue string) {
	t.Helper()
	if _, err := q.Enqueue(ctx(), job.Job{
		ID: id, Queue: queue, Kind: "turn",
		Payload: json.RawMessage(`{"chat":"c-1"}`),
	}); err != nil {
		t.Fatal(err)
	}
}

// TestAClaimedJobIsHandedToExactlyOneWorker. This is the whole reason the queue
// is a database: two workers arriving at the same instant must not both get the
// same row.
func TestAClaimedJobIsHandedToExactlyOneWorker(t *testing.T) {
	q, _ := open(t)
	const jobs = 40
	for i := range jobs {
		enqueue(t, q, fmt.Sprintf("j-%02d", i), job.QueueChat)
	}

	var (
		mu      sync.Mutex
		claimed = map[string]int{}
		empty   atomic.Int64
		wg      sync.WaitGroup
	)
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker := fmt.Sprintf("w-%d", w)
			for {
				got, err := q.Claim(ctx(), nil, worker, time.Minute)
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				if got == nil {
					empty.Add(1)
					return
				}
				mu.Lock()
				claimed[got.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(claimed) != jobs {
		t.Fatalf("%d of %d jobs were claimed", len(claimed), jobs)
	}
	for id, times := range claimed {
		if times != 1 {
			t.Fatalf("%s was handed out %d times", id, times)
		}
	}
	if empty.Load() == 0 {
		t.Fatal("no worker ever saw an empty queue, so the drain did not finish")
	}
}

// TestAnEmptyQueueIsNotAnError. It is the common case: a worker asks, there is
// nothing, and it waits.
func TestAnEmptyQueueIsNotAnError(t *testing.T) {
	q, _ := open(t)
	got, err := q.Claim(ctx(), nil, "w-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("an empty queue produced %+v", got)
	}
}

// TestAWorkerOnlyDrainsTheQueuesItAsksFor, which is what keeps a slow batch of
// one kind from starving another.
func TestAWorkerOnlyDrainsTheQueuesItAsksFor(t *testing.T) {
	q, _ := open(t)
	enqueue(t, q, "j-chat", job.QueueChat)
	enqueue(t, q, "j-task", job.QueueTask)

	got, err := q.Claim(ctx(), []string{job.QueueTask}, "w-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "j-task" {
		t.Fatalf("claimed %+v", got)
	}
	if again, _ := q.Claim(ctx(), []string{job.QueueTask}, "w-1", time.Minute); again != nil {
		t.Fatalf("a second task job appeared: %+v", again)
	}
}

// TestAJobIsNotEligibleUntilItsTime, which is how a retry backs off.
func TestAJobIsNotEligibleUntilItsTime(t *testing.T) {
	q, c := open(t)
	if _, err := q.Enqueue(ctx(), job.Job{
		ID: "j-later", Queue: job.QueueChat, Kind: "turn",
		RunAt: start.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	if got, _ := q.Claim(ctx(), nil, "w-1", time.Minute); got != nil {
		t.Fatal("a job scheduled for later was claimed now")
	}
	c.advance(5 * time.Minute)
	if got, _ := q.Claim(ctx(), nil, "w-1", time.Minute); got == nil {
		t.Fatal("the job was not claimable once its time came")
	}
}

// TestAWorkerThatDiesLosesItsClaim. Without this, one killed process is a
// permanent hole in the queue.
func TestAWorkerThatDiesLosesItsClaim(t *testing.T) {
	q, c := open(t)
	enqueue(t, q, "j-1", job.QueueChat)

	got, err := q.Claim(ctx(), nil, "w-dead", 10*time.Minute)
	if err != nil || got == nil {
		t.Fatalf("claim = %+v, %v", got, err)
	}
	if got.Attempts != 1 {
		t.Fatalf("attempts = %d after the first claim", got.Attempts)
	}

	// While the lease holds, nobody else gets it.
	if other, _ := q.Claim(ctx(), nil, "w-other", time.Minute); other != nil {
		t.Fatal("a live claim was handed to a second worker")
	}
	if n, err := q.RecoverStale(ctx()); err != nil || n != 0 {
		t.Fatalf("recovered %d live jobs (%v)", n, err)
	}

	c.advance(11 * time.Minute)
	n, err := q.RecoverStale(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("recovered %d, want 1", n)
	}

	back, err := q.Claim(ctx(), nil, "w-other", time.Minute)
	if err != nil || back == nil {
		t.Fatalf("the recovered job was not claimable: %+v, %v", back, err)
	}
	if back.Attempts != 2 {
		t.Fatalf("attempts = %d — a job reaped repeatedly must still reach its limit", back.Attempts)
	}
}

// TestAHeartbeatKeepsAClaimAlive, and only for the worker that holds it.
func TestAHeartbeatKeepsAClaimAlive(t *testing.T) {
	q, c := open(t)
	enqueue(t, q, "j-1", job.QueueChat)

	if _, err := q.Claim(ctx(), nil, "w-1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	c.advance(4 * time.Minute)
	if err := q.Heartbeat(ctx(), "j-1", "w-1", 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	c.advance(4 * time.Minute)
	if n, err := q.RecoverStale(ctx()); err != nil || n != 0 {
		t.Fatalf("a heartbeating worker lost its job: %d (%v)", n, err)
	}

	// A process that was reaped and came back must not be able to extend a
	// lease on work that has since been handed to somebody else.
	if err := q.Heartbeat(ctx(), "j-1", "w-impostor", time.Minute); err == nil {
		t.Fatal("a worker that does not hold the job extended its lease")
	}
}

// TestAFailedJobRetriesUntilItsLimitAndThenDies. It stays in the table: "it ran
// and failed three times" and "it never arrived" are different problems.
func TestAFailedJobRetriesUntilItsLimitAndThenDies(t *testing.T) {
	q, c := open(t)
	if _, err := q.Enqueue(ctx(), job.Job{
		ID: "j-1", Queue: job.QueueChat, Kind: "turn", MaxTries: 3,
	}); err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		got, err := q.Claim(ctx(), nil, "w-1", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("attempt %d: nothing to claim", attempt)
		}
		if err := q.Fail(ctx(), "j-1", errors.New("the provider timed out"), nil); err != nil {
			t.Fatal(err)
		}
		c.advance(job.Backoff(attempt))
	}

	after, err := q.Get(ctx(), "j-1")
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != job.Dead {
		t.Fatalf("status = %s after three attempts", after.Status)
	}
	if after.Error == "" || after.EndedAt == nil {
		t.Fatalf("a dead job records nothing about why: %+v", after)
	}
	if got, _ := q.Claim(ctx(), nil, "w-1", time.Minute); got != nil {
		t.Fatal("a dead job was claimed again")
	}
}

// TestAnExplicitRetryDelayOverridesTheBackoff, which is what a rate-limited
// provider's Retry-After header becomes.
func TestAnExplicitRetryDelayOverridesTheBackoff(t *testing.T) {
	q, c := open(t)
	enqueue(t, q, "j-1", job.QueueChat)
	if _, err := q.Claim(ctx(), nil, "w-1", time.Minute); err != nil {
		t.Fatal(err)
	}

	wait := 90 * time.Second
	if err := q.Fail(ctx(), "j-1", errors.New("rate limited"), &wait); err != nil {
		t.Fatal(err)
	}
	c.advance(80 * time.Second)
	if got, _ := q.Claim(ctx(), nil, "w-1", time.Minute); got != nil {
		t.Fatal("the job was claimable before the delay it asked for")
	}
	c.advance(20 * time.Second)
	if got, _ := q.Claim(ctx(), nil, "w-1", time.Minute); got == nil {
		t.Fatal("the job never became claimable")
	}
}

// TestCompleteStoresWhatTheJobProduced.
func TestCompleteStoresWhatTheJobProduced(t *testing.T) {
	q, _ := open(t)
	enqueue(t, q, "j-1", job.QueueChat)
	if _, err := q.Claim(ctx(), nil, "w-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := q.Complete(ctx(), "j-1", json.RawMessage(`{"answer":"done"}`)); err != nil {
		t.Fatal(err)
	}

	after, err := q.Get(ctx(), "j-1")
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != job.Succeeded {
		t.Fatalf("status = %s", after.Status)
	}
	if string(after.Result) != `{"answer":"done"}` {
		t.Fatalf("result = %s", after.Result)
	}
	if after.ClaimedBy != "" || after.LeaseUntil != nil {
		t.Fatalf("a finished job still holds a lease: %+v", after)
	}
}

// TestTheOldestEligibleJobGoesFirst, so a retry that has been waiting does not
// starve behind a stream of fresh work.
func TestTheOldestEligibleJobGoesFirst(t *testing.T) {
	q, c := open(t)
	enqueue(t, q, "j-old", job.QueueChat)
	c.advance(time.Hour)
	enqueue(t, q, "j-new", job.QueueChat)

	got, err := q.Claim(ctx(), nil, "w-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "j-old" {
		t.Fatalf("claimed %q first", got.ID)
	}
}

// TestListFiltersOnEveryFieldItAdvertises.
func TestListFiltersOnEveryFieldItAdvertises(t *testing.T) {
	q, _ := open(t)
	if _, err := q.Enqueue(ctx(), job.Job{
		ID: "j-1", Queue: job.QueueChat, Kind: "turn", Workspace: "alpha",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Enqueue(ctx(), job.Job{
		ID: "j-2", Queue: job.QueueRoutine, Kind: "tick", Workspace: "beta",
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		filter job.Filter
		want   int
	}{
		{"everything", job.Filter{}, 2},
		{"by queue", job.Filter{Queue: job.QueueChat}, 1},
		{"by workspace", job.Filter{Workspace: "beta"}, 1},
		{"by kind", job.Filter{Kind: "tick"}, 1},
		{"by status", job.Filter{Status: job.Pending}, 2},
		{"by status that matches nothing", job.Filter{Status: job.Dead}, 0},
		{"limited", job.Filter{Limit: 1}, 1},
	}
	for _, tc := range cases {
		got, err := q.List(ctx(), tc.filter)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != tc.want {
			t.Fatalf("%s: %d jobs, want %d", tc.name, len(got), tc.want)
		}
	}
}

// TestPurgeTakesOnlyFinishedWork.
func TestPurgeTakesOnlyFinishedWork(t *testing.T) {
	q, c := open(t)
	enqueue(t, q, "j-done", job.QueueChat)
	enqueue(t, q, "j-waiting", job.QueueChat)

	if _, err := q.Claim(ctx(), []string{job.QueueChat}, "w-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := q.Complete(ctx(), "j-done", nil); err != nil {
		t.Fatal(err)
	}

	c.advance(8 * 24 * time.Hour)
	n, err := q.Purge(ctx(), 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged %d, want 1", n)
	}
	left, err := q.List(ctx(), job.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].ID != "j-waiting" {
		t.Fatalf("left = %+v", left)
	}
}

// TestAJobThatIsNotThereIsAnswered, not an error.
func TestAJobThatIsNotThereIsAnswered(t *testing.T) {
	q, _ := open(t)
	got, err := q.Get(ctx(), "j-nothing")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %+v", got)
	}
	if err := q.Fail(ctx(), "j-nothing", errors.New("x"), nil); err == nil {
		t.Fatal("failing a job that does not exist reported success")
	}
}

// TestAJobNeedsAnIdentifierAndAQueue.
func TestAJobNeedsAnIdentifierAndAQueue(t *testing.T) {
	q, _ := open(t)
	if _, err := q.Enqueue(ctx(), job.Job{Queue: job.QueueChat}); err == nil {
		t.Fatal("a job with no identifier was accepted")
	}
	if _, err := q.Enqueue(ctx(), job.Job{ID: "j-1"}); err == nil {
		t.Fatal("a job with no queue was accepted")
	}
}

// TestTheQueueSurvivesReopening, which is the point of it being on disk.
func TestTheQueueSurvivesReopening(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.sqlite")
	c := &clock{at: start}

	first, err := sqlitequeue.Open(sqlitequeue.Options{Path: path, Clock: c.now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Enqueue(ctx(), job.Job{ID: "j-1", Queue: job.QueueTask, Kind: "run"}); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := sqlitequeue.Open(sqlitequeue.Options{Path: path, Clock: c.now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()

	got, err := second.Claim(ctx(), nil, "w-1", time.Minute)
	if err != nil || got == nil {
		t.Fatalf("the job did not survive the restart: %+v, %v", got, err)
	}
	if got.Kind != "run" {
		t.Fatalf("kind = %q", got.Kind)
	}
}

// TestBackoffGrowsAndThenStops. An unbounded backoff turns a transient outage
// into a job that retries next week.
func TestBackoffGrowsAndThenStops(t *testing.T) {
	cases := map[int]time.Duration{
		0: 10 * time.Second,
		1: 10 * time.Second,
		2: 20 * time.Second,
		3: 40 * time.Second,
		7: 10 * time.Minute,
		9: 10 * time.Minute,
	}
	for attempt, want := range cases {
		if got := job.Backoff(attempt); got != want {
			t.Errorf("Backoff(%d) = %s, want %s", attempt, got, want)
		}
	}
}
