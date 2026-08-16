package clockx_test

import (
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/clockx"
)

var refTime = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

func TestSystemClockAdvances(t *testing.T) {
	var c clockx.Clock = clockx.System{}
	first := c.Now()
	if first.IsZero() {
		t.Fatal("the system clock returned the zero time")
	}
	if second := c.Now(); second.Before(first) {
		t.Fatal("time went backwards")
	}
}

// TestFixedClockNeverMoves is what makes a golden file derived from a record
// stable, and a test that asserts on a timestamp pass on a slow machine.
func TestFixedClockNeverMoves(t *testing.T) {
	c := clockx.Fixed{At: refTime}
	for i := 0; i < 5; i++ {
		if !c.Now().Equal(refTime) {
			t.Fatalf("read %d moved: %v", i, c.Now())
		}
	}
}

// TestSteppingClockGivesDistinctTimestamps: a sequence of records created in
// one test must not share a timestamp, or ordering by it is untestable.
func TestSteppingClockGivesDistinctTimestamps(t *testing.T) {
	c := &clockx.Stepping{At: refTime, Step: time.Minute}
	seen := map[time.Time]bool{}
	prev := time.Time{}
	for i := 0; i < 5; i++ {
		now := c.Now()
		if seen[now] {
			t.Fatalf("timestamp %v repeated", now)
		}
		if !prev.IsZero() && !now.After(prev) {
			t.Fatalf("%v does not follow %v", now, prev)
		}
		seen[now], prev = true, now
	}
}

func TestSteppingClockDefaultsToOneSecond(t *testing.T) {
	c := &clockx.Stepping{At: refTime}
	first := c.Now()
	if got := c.Now().Sub(first); got != time.Second {
		t.Fatalf("step = %v, want 1s", got)
	}
}

// TestSettingTheSteppingClockMovesIt, which is how a test files a record on a
// particular day without building the record by hand.
func TestSettingTheSteppingClockMovesIt(t *testing.T) {
	c := &clockx.Stepping{At: refTime, Step: time.Minute}
	later := refTime.AddDate(0, 2, 0)

	c.Set(later)
	if got := c.Now(); !got.Equal(later) {
		t.Fatalf("Now = %v, want %v", got, later)
	}
	if got := c.Now(); !got.Equal(later.Add(time.Minute)) {
		t.Fatalf("the clock stopped stepping after being set: %v", got)
	}
}
