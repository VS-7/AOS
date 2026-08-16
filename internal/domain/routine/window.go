package routine

import (
	"fmt"
	"strings"
	"time"
)

// DefaultTick is how often the scheduler evaluates cron triggers.
//
// It is the original's `*/15 * * * *`, and it is the real resolution of the
// system: a routine with a cron of `* * * * *` does not fire every minute, it
// is evaluated every fifteen. The original leaves the user to discover that;
// here it is reported by routines get.
const DefaultTick = 15 * time.Minute

// Schedule is a parsed cron expression.
//
// Five fields, standard semantics: minute, hour, day of month, month, day of
// week. Ranges, steps and lists are supported; the named shorthands are not,
// because a cron in a file somebody edits is better off unambiguous.
type Schedule struct {
	expr    string
	minutes []bool // 60
	hours   []bool // 24
	days    []bool // 32, index 1..31
	months  []bool // 13, index 1..12
	weekly  []bool // 7, Sunday 0

	// dayRestricted and weekdayRestricted record whether each field was
	// narrowed. Cron's oddest rule is that when both are, they are OR-ed rather
	// than AND-ed, and a scheduler that gets it wrong silently never fires.
	dayRestricted     bool
	weekdayRestricted bool
}

// Parse compiles a five-field cron expression.
func Parse(expr string) (*Schedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("a cron expression has five fields and %q has %d", expr, len(fields))
	}
	s := &Schedule{expr: strings.Join(fields, " ")}

	var err error
	if s.minutes, _, err = parseField(fields[0], 0, 59); err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	if s.hours, _, err = parseField(fields[1], 0, 23); err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	if s.days, s.dayRestricted, err = parseField(fields[2], 1, 31); err != nil {
		return nil, fmt.Errorf("day of month: %w", err)
	}
	if s.months, _, err = parseField(fields[3], 1, 12); err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	if s.weekly, s.weekdayRestricted, err = parseField(fields[4], 0, 6); err != nil {
		return nil, fmt.Errorf("day of week: %w", err)
	}
	return s, nil
}

// Expr returns the normalised expression.
func (s *Schedule) Expr() string { return s.expr }

// Matches reports whether the expression fires at this minute.
func (s *Schedule) Matches(t time.Time) bool {
	if !s.minutes[t.Minute()] || !s.hours[t.Hour()] || !s.months[int(t.Month())] {
		return false
	}
	day, weekday := s.days[t.Day()], s.weekly[int(t.Weekday())]
	switch {
	case s.dayRestricted && s.weekdayRestricted:
		// Cron's OR rule: "1 * * 13 5" means the 13th or a Friday, not both.
		return day || weekday
	case s.dayRestricted:
		return day
	case s.weekdayRestricted:
		return weekday
	default:
		return true
	}
}

// Next returns the first minute at or after from that the expression fires,
// searching up to a year ahead.
func (s *Schedule) Next(from time.Time) (time.Time, bool) {
	t := from.Truncate(time.Minute)
	if t.Before(from) {
		t = t.Add(time.Minute)
	}
	limit := from.AddDate(1, 0, 0)
	for !t.After(limit) {
		if s.Matches(t) {
			return t, true
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, false
}

// DueInWindow reports whether a cron fires inside the window (last, now].
//
// A cron that would fire several times inside one window fires once — that is
// the effective resolution of the system, and the reason it is a single boolean
// rather than a count. With no previous firing the window is one tick back, so
// a routine created between ticks does not fire for every minute since the
// epoch.
func DueInWindow(s *Schedule, last, now time.Time, tick time.Duration) bool {
	if s == nil {
		return false
	}
	if tick <= 0 {
		tick = DefaultTick
	}
	from := last
	if from.IsZero() || now.Sub(from) > tick {
		from = now.Add(-tick)
	}
	if !now.After(from) {
		return false
	}
	next, ok := s.Next(from.Add(time.Minute))
	return ok && !next.After(now)
}

// parseField compiles one cron field, reporting whether it was narrowed from
// "every value".
func parseField(field string, min, max int) ([]bool, bool, error) {
	set := make([]bool, max+1)
	restricted := field != "*"

	for _, part := range strings.Split(field, ",") {
		step := 1
		if slash := strings.IndexByte(part, '/'); slash >= 0 {
			parsed, err := atoi(part[slash+1:])
			if err != nil || parsed <= 0 {
				return nil, false, fmt.Errorf("%q is not a step", part[slash+1:])
			}
			step = parsed
			part = part[:slash]
		}

		from, to := min, max
		switch {
		case part == "*" || part == "":
			// the whole range
		case strings.ContainsRune(part, '-'):
			bounds := strings.SplitN(part, "-", 2)
			var err error
			if from, err = atoi(bounds[0]); err != nil {
				return nil, false, fmt.Errorf("%q is not a number", bounds[0])
			}
			if to, err = atoi(bounds[1]); err != nil {
				return nil, false, fmt.Errorf("%q is not a number", bounds[1])
			}
		default:
			parsed, err := atoi(part)
			if err != nil {
				return nil, false, fmt.Errorf("%q is not a number", part)
			}
			from, to = parsed, parsed
		}
		if from < min || to > max || from > to {
			return nil, false, fmt.Errorf("%d-%d is outside %d-%d", from, to, min, max)
		}
		for v := from; v <= to; v += step {
			set[v] = true
		}
	}
	return set, restricted, nil
}

func atoi(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a digit: %q", r)
		}
		n = n*10 + int(r-'0')
		if n > 9999 {
			return 0, fmt.Errorf("too large")
		}
	}
	return n, nil
}

// render turns a filter value into the string the comparison uses. Numbers
// coming out of JSON are float64 and out of YAML are int, and a filter written
// as 1 must match a payload carrying 1.0.
func render(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%g", typed)
	case float32:
		return render(float64(typed))
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
