// Package ids is the source of new identifiers.
//
// It is a port with two implementations for the same reason the clock is: a
// record written during a test must be reproducible, and a UUID drawn from the
// operating system's entropy is the one thing that makes a golden file
// impossible. Production draws from crypto/rand; a test draws from a counter.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

// Generator hands out identifiers. One method, because that is the whole job.
type Generator interface {
	New() string
}

// UUID generates random version 4 UUIDs, which is what the original uses for
// memories, chats and messages.
type UUID struct{}

// New returns a random UUID in the canonical 8-4-4-4-12 hexadecimal form.
//
// It reads from crypto/rand, which on every supported platform is a blocking
// read from the kernel CSPRNG that cannot fail after boot. A failure here is
// therefore not a condition a caller could handle — it means the system has no
// entropy source — so it panics rather than widening every identity-producing
// signature in the domain with an error nobody can act on.
func (UUID) New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("ids: the system entropy source is unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return format(b)
}

// Fixed always returns the same identifier. It is for the test that asserts on
// one record and does not care what it is called.
type Fixed struct{ ID string }

func (f Fixed) New() string { return f.ID }

// Sequence hands out predictable identifiers with a shared prefix, so a test
// that creates several records can name them in its assertions. It is safe for
// concurrent use, because the concurrency tests create records from several
// goroutines at once.
type Sequence struct {
	Prefix string
	n      atomic.Int64
}

// New returns "{prefix}-1", "{prefix}-2", … The prefix defaults to "id".
func (s *Sequence) New() string {
	prefix := s.Prefix
	if prefix == "" {
		prefix = "id"
	}
	return fmt.Sprintf("%s-%d", prefix, s.n.Add(1))
}

func format(b [16]byte) string {
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}
