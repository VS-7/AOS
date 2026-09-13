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
