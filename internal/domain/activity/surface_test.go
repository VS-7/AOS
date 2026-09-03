package activity

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/command"
)

// TestRegisterPublishesTheWholeGroup, and Publish is not part of it.
//
// An activity records something the system did. A surface that could write one
// directly would let an agent fabricate the record of a change that never
// happened, so every entry comes from the mutation that caused it.
func TestRegisterPublishesTheWholeGroup(t *testing.T) {
	svc, _, _, _ := newService(t)
	reg := command.NewRegistry()
	Register(reg, svc)

	want := []string{
		"activity_delete", "activity_events", "activity_get", "activity_list",
		"activity_purge", "activity_read", "activity_read-all",
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
		case "activity_purge", "activity_delete":
			if !d.Annotations().DestructiveHint {
				t.Errorf("%s must be announced destructive", d.Key())
			}
			if d.InRegistry() {
				t.Errorf("%s is reachable by an agent; editing the log is not the agent's to do", d.Key())
			}
		case "activity_list", "activity_get":
			if !d.Annotations().ReadOnlyHint {
				t.Errorf("%s must be announced read-only", d.Key())
			}
		}
	}
}

// TestASinkCanBeAttachedAfterConstruction. The routine evaluator needs it:
// routines react to activities, and a routine's own run publishes one.
func TestASinkCanBeAttachedAfterConstruction(t *testing.T) {
	svc, _, _, _ := newService(t)
	late := &recordingSink{}
	svc.AddSink(late)
	svc.AddSink(nil) // ignored rather than panicking on the next publish

	if _, err := svc.Publish(asAgent("atlas"), PublishInput{
		Namespace: "task", Event: "created", Title: "a task exists",
	}); err != nil {
		t.Fatal(err)
	}
	if late.len() != 1 {
		t.Fatal("a sink attached after construction saw nothing")
	}
}

// TestTheInputsThatCannotBeAccepted.
func TestTheInputsThatCannotBeAccepted(t *testing.T) {
	svc, _, _, _ := newService(t)

	cases := []struct {
		name string
		call func() error
		code string
	}{
		{"a since that is not an instant", func() error {
			_, err := svc.List(asAgent("atlas"), ListInput{Since: "yesterday"})
			return err
		}, "ACTIVITY_INVALID_TIME"},
		{"an entry that is not there", func() error {
			_, err := svc.Get(asAgent("atlas"), GetInput{ID: "a-missing"})
			return err
		}, "ACTIVITY_NOT_FOUND"},
		{"marking one that is not there", func() error {
			_, err := svc.MarkAsRead(asAgent("atlas"), MarkInput{ID: "a-missing"})
			return err
		}, "ACTIVITY_NOT_FOUND"},
		{"deleting one that is not there", func() error {
			_, err := svc.Delete(asAgent("atlas"), DeleteInput{ID: "a-missing"})
			return err
		}, "ACTIVITY_NOT_FOUND"},
		{"an entry with no namespace", func() error {
			_, err := svc.Publish(asAgent("atlas"), PublishInput{Event: "created", Title: "x"})
			return err
		}, "ACTIVITY_INCOMPLETE"},
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

// TestSinceFiltersTheWindow.
func TestSinceFiltersTheWindow(t *testing.T) {
	svc, _, _, clock := newService(t)
	ctx := asAgent("atlas")

	if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "early"}); err != nil {
		t.Fatal(err)
	}
	cut := clock.Now()
	if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "late"}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.List(ctx, ListInput{Since: cut.Format("2006-01-02T15:04:05Z07:00")})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Activities[0].Title != "late" {
		t.Fatalf("got %d entries: %+v", out.Total, out.Activities)
	}
}

// TestWithoutAReadStoreTheInboxStillReads. It reads as entirely unread, which
// is the honest answer when nothing is recording what was seen.
func TestWithoutAReadStoreTheInboxStillReads(t *testing.T) {
	log := &memLog{}
	svc := NewService(Deps{Log: log, Clock: &clockFixed{}, IDs: &countingIDs{}})
	ctx := asAgent("atlas")

	if _, err := svc.Publish(ctx, PublishInput{Namespace: "task", Event: "created", Title: "one"}); err != nil {
		t.Fatal(err)
	}
	out, err := svc.List(ctx, ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Unread != 1 {
		t.Fatalf("unread = %d", out.Unread)
	}
}

type clockFixed struct{}

func (clockFixed) Now() time.Time { return start }
