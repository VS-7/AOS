//go:build !windows

package supervise

import (
	"testing"
	"time"
)

// A process's age decides whether the pid in a record still belongs to the
// daemon it was written about, so reading ps's elapsed time wrong is reading
// a live daemon as somebody else — or the reverse. Every shape ps uses is
// here, and everything it does not is zero: "the platform would not say",
// which is no evidence at all rather than an age of nothing.
func TestParseElapsedReadsEveryShapePsUses(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"00:07", 7 * time.Second},
		{"  01:30", 90 * time.Second},
		{"59:59", 59*time.Minute + 59*time.Second},
		{"01:00:00", time.Hour},
		{"25:10:05", 25*time.Hour + 10*time.Minute + 5*time.Second},
		{"3-04:05:06", 3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second},
		{"", 0},
		{"07", 0},
		{"-", 0},
		{"a:b", 0},
		{"1-2", 0},
		{"1:2:3:4", 0},
		{"x-01:00:00", 0},
	} {
		if got := parseElapsed(tc.in); got != tc.want {
			t.Errorf("parseElapsed(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
