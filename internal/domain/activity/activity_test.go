package activity

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/identity"
)

// memLog is the Log port in memory. The domain may not touch a filesystem, and
// the real writer has its own suite beside it.
type memLog struct {
	mu      sync.Mutex
	entries []Activity
	failOn  string
}

func (l *memLog) Append(_ context.Context, a Activity) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failOn == "append" {
		return errors.New("disk full")
	}
	l.entries = append(l.entries, a)
	return nil
}

func (l *memLog) Load(_ context.Context, since time.Time) ([]Activity, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Activity, 0, len(l.entries))
	for _, a := range l.entries {
		if since.IsZero() || !a.CreatedAt.Before(since) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (l *memLog) Months(context.Context) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, a := range l.entries {
		if !seen[a.Month()] {
			seen[a.Month()] = true
			out = append(out, a.Month())
		}
	}
	sort.Strings(out)
	return out, nil
}

func (l *memLog) Rewrite(_ context.Context, month string, entries []Activity) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := make([]Activity, 0, len(l.entries))
	for _, a := range l.entries {
		if a.Month() != month {
			kept = append(kept, a)
		}
	}
	l.entries = append(kept, entries...)
	sort.SliceStable(l.entries, func(i, j int) bool { return l.entries[i].CreatedAt.Before(l.entries[j].CreatedAt) })
	return nil
}

func (l *memLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

type memRead struct {
	mu    sync.Mutex
	state ReadState
	saves int
}

func (r *memRead) Load(context.Context) (ReadState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// A copy, so a service holding the result cannot mutate the store.
	out := ReadState{Watermark: map[string]time.Time{}, IDs: map[string][]string{}}
	for k, v := range r.state.Watermark {
		out.Watermark[k] = v
	}
	for k, v := range r.state.IDs {
		out.IDs[k] = append([]string(nil), v...)
	}
	return out, nil
}

func (r *memRead) Save(_ context.Context, s ReadState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = s
	r.saves++
	return nil
}

type countingIDs struct {
	mu sync.Mutex
	n  int
}

func (g *countingIDs) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return "a" + strconv.Itoa(g.n)
}

type recordingSink struct {
	mu   sync.Mutex
	seen []Activity
}

func (s *recordingSink) OnActivity(_ context.Context, a Activity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = append(s.seen, a)
}

func (s *recordingSink) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}

type panickingSink struct{}

func (panickingSink) OnActivity(context.Context, Activity) { panic("a routine blew up") }

var start = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newService(t *testing.T, sinks ...Sink) (*Service, *memLog, *memRead, *clockx.Stepping) {
	t.Helper()
	log, read := &memLog{}, &memRead{}
	clock := &clockx.Stepping{At: start, Step: time.Minute}
	return NewService(Deps{
		Log: log, Read: read, Clock: clock, IDs: &countingIDs{}, Sinks: sinks,
	}), log, read, clock
}

func asAgent(id string) context.Context {
	return identity.With(context.Background(), identity.Identity{AgentID: id})
}

// TestPublishWritesThenFansOut. The order is the contract: the log is the
// product and the fan-out is the convenience.
func TestPublishWritesThenFansOut(t *testing.T) {
	sink := &recordingSink{}
	svc, log, _, _ := newService(t, sink)

	got, err := svc.Publish(asAgent("atlas"), PublishInput{
		Namespace: "task", Event: "status_changed",
		Title: "Ship the runtime moved to in_review",
		Data:  map[string]any{"task": "t1", "type": "bug"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Actor != "atlas" || got.ActorType != ActorAgent {
		t.Fatalf("authorship = %q/%q", got.Actor, got.ActorType)
	}
	if !got.CreatedAt.Equal(start) {
		t.Fatalf("createdAt = %s", got.CreatedAt)
	}
	if log.len() != 1 {
		t.Fatalf("the log holds %d entries", log.len())
	}
	if sink.len() != 1 {
		t.Fatalf("the sink saw %d entries", sink.len())
	}
}

// TestAConsumerThatBlowsUpDoesNotFailTheMutation. This is the whole reason the
// fan-out is best-effort: a routine that panics must not roll back the task
// whose status changed.
func TestAConsumerThatBlowsUpDoesNotFailTheMutation(t *testing.T) {
	good := &recordingSink{}
	svc, log, _, _ := newService(t, panickingSink{}, good)

	if _, err := svc.Publish(asAgent("atlas"), PublishInput{
		Namespace: "task", Event: "created", Title: "a task exists",
	}); err != nil {
		t.Fatalf("a panicking consumer failed the publish: %v", err)
	}
	if log.len() != 1 {
		t.Fatal("the entry was not recorded")
	}
	if good.len() != 1 {
		t.Fatal("a panicking consumer stopped the one after it")
	}
}

// TestAPublishThatCannotBeWrittenFails, because the log is the one part of the
// fan-out the caller is entitled to hear about.
func TestAPublishThatCannotBeWrittenFails(t *testing.T) {
	sink := &recordingSink{}
	svc, log, _, _ := newService(t, sink)
	log.failOn = "append"

	if _, err := svc.Publish(asAgent("atlas"), PublishInput{
		Namespace: "task", Event: "created", Title: "a task exists",
	}); err == nil {
		t.Fatal("a log that could not be written reported success")
	}
	if sink.len() != 0 {
		t.Fatal("the fan-out ran for an entry that was never recorded")
	}
}

// TestAnActivityWithNoActorIsAttributedToTheSystem. A tick or a purge has no
// person behind it, and attributing it to whoever last logged in is a lie the
// audit trail would then carry forever.
func TestAnActivityWithNoActorIsAttributedToTheSystem(t *testing.T) {
	svc, _, _, _ := newService(t)

	got, err := svc.Publish(context.Background(), PublishInput{
		Namespace: "activity", Event: "purged", Title: "90 days expired",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Actor != ActorSystem || got.ActorType != ActorSystem {
		t.Fatalf("authorship = %q/%q", got.Actor, got.ActorType)
	}
}

// TestAnIncompleteActivityIsRefused: without a namespace and an event there is
// nothing a routine trigger could match on.
func TestAnIncompleteActivityIsRefused(t *testing.T) {
	svc, _, _, _ := newService(t)
	for _, in := range []PublishInput{
		{Event: "created", Title: "x"},
		{Namespace: "task", Title: "x"},
	} {
		if _, err := svc.Publish(asAgent("atlas"), in); err == nil {
			t.Fatalf("%+v was accepted", in)
		}
	}
}

// TestReadStateIsPerActor. An agent and a person have independent inboxes over
// the same stream.
func TestReadStateIsPerActor(t *testing.T) {
	svc, _, _, _ := newService(t)
	ctx := asAgent("atlas")

	first, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "two"}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.MarkAsRead(ctx, MarkInput{ID: first.ID}); err != nil {
		t.Fatal(err)
	}

	mine, err := svc.List(ctx, ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if mine.Unread != 1 {
		t.Fatalf("atlas has %d unread, want 1", mine.Unread)
	}

	theirs, err := svc.List(asAgent("nova"), ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if theirs.Unread != 2 {
		t.Fatalf("nova has %d unread, want 2 — read state leaked between actors", theirs.Unread)
	}
}

// TestMarkingSomethingAlreadyReadWritesNothing. Without this the overlay file is
// rewritten every time a desktop re-renders the inbox.
func TestMarkingSomethingAlreadyReadWritesNothing(t *testing.T) {
	svc, _, read, _ := newService(t)
	ctx := asAgent("atlas")

	one, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkAsRead(ctx, MarkInput{ID: one.ID}); err != nil {
		t.Fatal(err)
	}
	saves := read.saves

	out, err := svc.MarkAsRead(ctx, MarkInput{ID: one.ID})
	if err != nil {
		t.Fatal(err)
	}
	if out.Changed {
		t.Fatal("marking an already-read entry reported a change")
	}
	if read.saves != saves {
		t.Fatalf("the overlay was rewritten %d extra times", read.saves-saves)
	}
}

// TestTheWatermarkCoversEverythingUpToNowAndNothingAfter.
func TestTheWatermarkCoversEverythingUpToNowAndNothingAfter(t *testing.T) {
	svc, _, _, _ := newService(t)
	ctx := asAgent("atlas")

	for i := range 3 {
		if _, err := svc.Publish(ctx, PublishInput{
			Namespace: "task", Event: "created", Title: "n" + strconv.Itoa(i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.MarkAllAsRead(ctx, MarkAllInput{}); err != nil {
		t.Fatal(err)
	}

	after, err := svc.List(ctx, ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if after.Unread != 0 {
		t.Fatalf("%d unread after marking all read", after.Unread)
	}

	if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "later"}); err != nil {
		t.Fatal(err)
	}
	later, err := svc.List(ctx, ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if later.Unread != 1 {
		t.Fatalf("%d unread, want 1 — the watermark swallowed an entry published after it", later.Unread)
	}
}

// TestTheInboxFiltersAndCountsTheWholeMatch. The unread count is for everything
// that matched, not for the page — otherwise it shrinks as you page through.
func TestTheInboxFiltersAndCountsTheWholeMatch(t *testing.T) {
	svc, _, _, _ := newService(t)
	ctx := asAgent("atlas")

	for _, ev := range []string{"created", "status_changed", "status_changed"} {
		if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: ev, Title: ev}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Publish(ctx, PublishInput{Namespace: "memory", Event: "stored", Title: "m"}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.List(ctx, ListInput{Namespace: "task", Event: "status_changed", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || len(out.Activities) != 1 {
		t.Fatalf("total %d, page %d", out.Total, len(out.Activities))
	}
	if out.Unread != 2 {
		t.Fatalf("unread = %d, want the whole match", out.Unread)
	}
	// Newest first.
	if out.Activities[0].Title != "status_changed" {
		t.Fatalf("first = %+v", out.Activities[0])
	}
}

// TestPurgeDropsWholeMonthsAndSaysWhichItHadToRewrite. Rewriting an audit log is
// worth knowing about even when it is correct.
func TestPurgeDropsWholeMonthsAndSaysWhichItHadToRewrite(t *testing.T) {
	log, read := &memLog{}, &memRead{}
	clock := &clockx.Stepping{At: start, Step: time.Second}
	svc := NewService(Deps{Log: log, Read: read, Clock: clock, IDs: &countingIDs{}})
	ctx := asAgent("atlas")

	// One entry in January, two in March: one old, one current.
	write := func(when time.Time) {
		t.Helper()
		clock.Set(when)
		if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: when.String()}); err != nil {
			t.Fatal(err)
		}
	}
	write(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	write(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	write(time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC))

	clock.Set(time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC))
	out, err := svc.Purge(ctx, PurgeInput{OlderThanDays: 20})
	if err != nil {
		t.Fatal(err)
	}
	if out.Removed != 2 {
		t.Fatalf("removed %d, want 2", out.Removed)
	}
	if len(out.Dropped) != 1 || out.Dropped[0] != "2026-01" {
		t.Fatalf("dropped = %v", out.Dropped)
	}
	if len(out.Rewritten) != 1 || out.Rewritten[0] != "2026-03" {
		t.Fatalf("rewritten = %v", out.Rewritten)
	}
	if log.len() != 1 {
		t.Fatalf("%d entries survived, want 1", log.len())
	}
}

// TestDeleteRemovesOneEntryAndNamesThePartitionItRewrote.
func TestDeleteRemovesOneEntryAndNamesThePartitionItRewrote(t *testing.T) {
	svc, log, _, _ := newService(t)
	ctx := asAgent("atlas")

	one, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "two"}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Delete(ctx, DeleteInput{ID: one.ID})
	if err != nil {
		t.Fatal(err)
	}
	if out.Month != "2026-03" {
		t.Fatalf("month = %q", out.Month)
	}
	if log.len() != 1 {
		t.Fatalf("%d entries left", log.len())
	}
	if _, err := svc.Get(ctx, GetInput{ID: one.ID}); err == nil {
		t.Fatal("the deleted entry is still readable")
	}
}

// TestATriggerKeyMatchesTheNamespaceAndOptionallyTheEvent.
func TestATriggerKeyMatchesTheNamespaceAndOptionallyTheEvent(t *testing.T) {
	a := Activity{Namespace: "task", Event: "status_changed"}
	for _, tc := range []struct {
		key  Key
		want bool
	}{
		{Key{Namespace: "task", Event: "status_changed"}, true},
		{Key{Namespace: "TASK", Event: "STATUS_CHANGED"}, true},
		{Key{Namespace: "task"}, true},
		{Key{Namespace: "task", Event: "created"}, false},
		{Key{Namespace: "memory"}, false},
	} {
		if got := tc.key.Matches(a); got != tc.want {
			t.Fatalf("%+v matched %v, want %v", tc.key, got, tc.want)
		}
	}
}

// TestConcurrentPublishesAllLand, which is what the -race flag is for on a log
// that every mutation in the system writes to.
func TestConcurrentPublishesAllLand(t *testing.T) {
	sink := &recordingSink{}
	svc, log, _, _ := newService(t, sink)
	ctx := asAgent("atlas")

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Publish(ctx, PublishInput{
				Namespace: "task", Event: "created", Title: strconv.Itoa(i),
			})
		}()
	}
	wg.Wait()

	if log.len() != 100 {
		t.Fatalf("%d of 100 publications landed", log.len())
	}
	if sink.len() != 100 {
		t.Fatalf("the sink saw %d of 100", sink.len())
	}
}
