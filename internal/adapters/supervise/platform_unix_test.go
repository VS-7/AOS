//go:build !windows

package supervise

import (
	"testing"
	"time"
)

// A process's age decides whether the pid in a record still belongs to the
// daemon it was written about, so reading ps's elapsed time wrong is reading
// a live daemon as somebody else — or the reverse. Every shape ps uses is
// here, and everything it does not is refused: "the platform would not say",
// which is no evidence at all rather than an age of nothing. 00:00 is an age,
// not a refusal — it is what ps prints for a process in its first second, and
// the pid that was just reused is the one that reads youngest.
func TestParseElapsedReadsEveryShapePsUses(t *testing.T) {
	for _, tc := range []struct {
		in    string
		want  time.Duration
		known bool
	}{
		{"00:00", 0, true},
		{"00:07", 7 * time.Second, true},
		{"  01:30", 90 * time.Second, true},
		{"59:59", 59*time.Minute + 59*time.Second, true},
		{"01:00:00", time.Hour, true},
		{"25:10:05", 25*time.Hour + 10*time.Minute + 5*time.Second, true},
		{"3-04:05:06", 3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second, true},
		{"", 0, false},
		{"07", 0, false},
		{"-", 0, false},
		{"a:b", 0, false},
		{"1-2", 0, false},
		{"1:2:3:4", 0, false},
		{"x-01:00:00", 0, false},
	} {
		got, known := parseElapsed(tc.in)
		if got != tc.want || known != tc.known {
			t.Errorf("parseElapsed(%q) = %s, %t, want %s, %t", tc.in, got, known, tc.want, tc.known)
		}
	}
}
