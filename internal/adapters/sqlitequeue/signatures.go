package sqlitequeue

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Signatures is the subconscious's deduplication set, kept in the same database
// as the queue.
//
// The Subconsciente (Go) note diverges from the original here on purpose: the
// original holds signatures in process memory, so every restart lets the same
// memory be formed again — and a daemon restarts often during development and
// on every automatic update. This is operational state, not domain, which is
// why it lives beside the queue rather than in a file under .aos (ADR-0004).
type Signatures struct{ q *Queue }

// Signatures returns the deduplication set backed by this queue's database.
func (q *Queue) Signatures() *Signatures { return &Signatures{q: q} }

const signatureSchema = `
CREATE TABLE IF NOT EXISTS subconscious_signatures (
	agent      TEXT NOT NULL,
	signature  TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (agent, signature)
);

CREATE INDEX IF NOT EXISTS signatures_expiry ON subconscious_signatures (expires_at);
`

// Seen reports whether this draft has already been stored and has not expired.
//
// An expired row is deleted as it is read. That keeps the table bounded without
// a sweeper: the only rows that outlive their window are the ones nobody asks
// about again, and those are removed by Prune on the tick.
func (s *Signatures) Seen(ctx context.Context, agentID, sig string) (bool, error) {
	var expires int64
	err := s.q.db.QueryRowContext(ctx,
		`SELECT expires_at FROM subconscious_signatures WHERE agent = ? AND signature = ?`,
		agentID, sig).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if s.q.clock().After(unstamp(expires)) {
		s.q.writes.Lock()
		defer s.q.writes.Unlock()
		_, _ = s.q.db.ExecContext(ctx,
			`DELETE FROM subconscious_signatures WHERE agent = ? AND signature = ?`, agentID, sig)
		return false, nil
	}
	return true, nil
}

// Mark records that this draft was stored.
//
// A repeat extends the window rather than being refused: a lesson the observer
// keeps arriving at is one whose suppression should keep holding.
func (s *Signatures) Mark(ctx context.Context, agentID, sig string, ttl time.Duration) error {
	now := s.q.clock()

	s.q.writes.Lock()
	defer s.q.writes.Unlock()

	_, err := s.q.db.ExecContext(ctx, `
		INSERT INTO subconscious_signatures (agent, signature, expires_at, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (agent, signature) DO UPDATE SET expires_at = excluded.expires_at`,
		agentID, sig, stamp(now.Add(ttl)), stamp(now))
	return err
}

// Prune removes expired signatures and reports how many went.
func (s *Signatures) Prune(ctx context.Context) (int, error) {
	s.q.writes.Lock()
	defer s.q.writes.Unlock()

	res, err := s.q.db.ExecContext(ctx,
		`DELETE FROM subconscious_signatures WHERE expires_at < ?`, stamp(s.q.clock()))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
