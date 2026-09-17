package routine

import (
	"testing"
	"time"
)

// A daemon's ticks land wherever its clock started, not on a minute: 18:05:00.3,
// 18:20:00.3. The window asked for its first candidate at from+1m, and Next
// rounds a time with seconds up to the following minute — so the minute right
// after `from` was never a candidate in any window. A cron at :06 never fired
// on a daemon ticking at :05:00.3 and :20:00.3, and with a one-minute tick an
// every-minute cron never fired at all.
func TestEveryMinuteBelongsToExactlyOneWindow(t *testing.T) {
	for _, tc := range []struct {
		name string
		tick time.Duration
		cron string
	}{
		{name: "the minute after the tick's own, on the default tick", tick: 15 * time.Minute, cron: "6 * * * *"},
		{name: "every minute, on a one-minute tick", tick: time.Minute, cron: "* * * * *"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schedule, err := Parse(tc.cron)
			if err != nil {
				t.Fatal(err)
			}
			offset := 300 * time.Millisecond
			start := time.Date(2026, 9, 13, 18, 5, 0, 0, time.UTC).Add(offset)
			var last time.Time
			fired := 0
			for now := start; now.Before(start.Add(2 * time.Hour)); now = now.Add(tc.tick) {
				if DueInWindow(schedule, last, now, tc.tick) {
					fired++
					last = now
				}
			}
			want := 2 // 18:06 and 19:06
			if tc.cron == "* * * * *" {
				want = int(2 * time.Hour / tc.tick)
			}
			if fired != want {
				t.Errorf("fired %d times in two hours, want %d", fired, want)
			}
		})
	}
}

// A routine does not fire for a slot that passed before it existed. With no
// firing yet the window reached a whole tick back whenever it was created: a
// routine made at 20:02:34 with its next run shown as 20:03 fired at 20:02:48
// for 20:02, and a daily nine o'clock routine made at 09:05 fired on the next
// tick although it said its next run was tomorrow.
func TestARoutineDoesNotFireForASlotBeforeItWasCreated(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tick          time.Duration
		cron          string
		created       time.Time
		early, onTime time.Time
	}{
		{
			name: "every minute, on a one-minute tick", tick: time.Minute, cron: "* * * * *",
			created: time.Date(2026, 9, 13, 20, 2, 34, 0, time.UTC),
			early:   time.Date(2026, 9, 13, 20, 2, 48, 0, time.UTC),
			onTime:  time.Date(2026, 9, 13, 20, 3, 48, 0, time.UTC),
		},
		{
			name: "daily at nine, on the default tick", tick: 15 * time.Minute, cron: "0 9 * * *",
			created: time.Date(2026, 9, 14, 9, 5, 0, 0, time.UTC),
			early:   time.Date(2026, 9, 14, 9, 14, 0, 0, time.UTC),
			onTime:  time.Date(2026, 9, 15, 9, 10, 0, 0, time.UTC),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, func(d *Deps) { d.Tick = tc.tick })
			h.clock.At = tc.created
			h.create(t, CreateInput{
				Name:     "Made between ticks",
				Triggers: []TriggerInput{{Type: Scheduled, Cron: tc.cron}},
			})

			if _, err := h.svc.ProcessScheduled(asAgent("atlas"), tc.early); err != nil {
				t.Fatal(err)
			}
			if n := h.executor.count(); n != 0 {
				t.Fatalf("fired %d times for a slot before the routine was created", n)
			}
			if _, err := h.svc.ProcessScheduled(asAgent("atlas"), tc.onTime); err != nil {
				t.Fatal(err)
			}
			if n := h.executor.count(); n != 1 {
				t.Fatalf("fired %d times at its first slot, want once", n)
			}
		})
	}
}
