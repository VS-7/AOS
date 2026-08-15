// Package clockx is the only place in production code that reads the wall
// clock.
//
// Everything else takes a Clock. Without that, a golden file built from a
// record changes every second, and a test that asserts on a timestamp is a test
// that fails on a slow machine.
package clockx

import "time"

// Clock reports the current time.
type Clock interface{ Now() time.Time }

// System reads the real clock.
type System struct{}

// Now returns the current time.
func (System) Now() time.Time { return time.Now() } //nolint:forbidigo // this is the one allowed call site

// Fixed always reports the same instant.
type Fixed struct{ At time.Time }

// Now returns the fixed instant.
func (f Fixed) Now() time.Time { return f.At }

// Stepping advances by a fixed step on every read, so a sequence of records
// gets distinct, predictable timestamps.
type Stepping struct {
	At   time.Time
	Step time.Duration
}

// Now returns the current instant and advances.
func (s *Stepping) Now() time.Time {
	now := s.At
	step := s.Step
	if step == 0 {
		step = time.Second
	}
	s.At = s.At.Add(step)
	return now
}
