package subconscious

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// Signature is the idempotency key of a memory draft.
//
// Without it, an observer running every turn recreates the same memory over and
// over. It hashes the normalised semantic content — the category plus the
// lowercased, whitespace-collapsed title and body — so a cosmetic rewording
// does not produce a second memory while a genuine change does.
func Signature(d Draft) string {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(d.Category))))
	h.Write([]byte{0})
	h.Write([]byte(normalize(d.Title)))
	h.Write([]byte{0})
	h.Write([]byte(normalize(d.Content)))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// normalize collapses everything that is presentation rather than meaning.
func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// Signatures records which drafts have already been stored.
//
// It is a port because of one divergence from the original: the original keeps
// signatures in process memory, so a restart lets every memory be formed again.
// Ours persists, because a daemon restarts often during development and on
// every automatic update.
type Signatures interface {
	Seen(ctx context.Context, agentID, sig string) (bool, error)
	Mark(ctx context.Context, agentID, sig string, ttl time.Duration) error
}

// MemorySignatures is the in-process implementation.
//
// It is what a run with no state directory gets, and what the tests use. Its
// behaviour matches the persistent one within a process; what it does not
// survive is a restart, which is precisely the property the persistent one
// exists for.
type MemorySignatures struct {
	mu    sync.Mutex
	seen  map[string]time.Time
	clock func() time.Time
}

// NewMemorySignatures builds an in-process set.
func NewMemorySignatures(clock func() time.Time) *MemorySignatures {
	if clock == nil {
		clock = time.Now //nolint:forbidigo // the fallback when no clock is injected
	}
	return &MemorySignatures{seen: map[string]time.Time{}, clock: clock}
}

// Seen reports whether this draft has already been stored and has not expired.
func (s *MemorySignatures) Seen(_ context.Context, agentID, sig string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	until, ok := s.seen[agentID+"/"+sig]
	if !ok {
		return false, nil
	}
	if s.clock().After(until) {
		delete(s.seen, agentID+"/"+sig)
		return false, nil
	}
	return true, nil
}

// Mark records that this draft was stored.
func (s *MemorySignatures) Mark(_ context.Context, agentID, sig string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[agentID+"/"+sig] = s.clock().Add(ttl)
	return nil
}

// Len reports how many live signatures are held, for a test that wants to see
// the set shrink rather than only that a draft was suppressed.
func (s *MemorySignatures) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}
