// Package sqlitequeue is the durable work queue on SQLite, in pure Go.
//
// The driver is modernc.org/sqlite rather than the faster mattn binding, and
// the reason is ADR-0008: the binding needs CGO, and CGO costs the trivial
// cross-compilation that ADR-0001 chose Go for. A queue that moves tens of jobs
// an hour does not care about the difference; a release process that needs six
// toolchains does.
//
// This is the only database in the system. Everything else is files (ADR-0004).
// The one thing files cannot give is what this is here for: an atomic claim.
package sqlitequeue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/aos/internal/domain/job"

	_ "modernc.org/sqlite" // the pure-Go driver, registered as "sqlite"
)

// Queue is the SQLite-backed implementation of job.Queue.
type Queue struct {
	db    *sql.DB
	clock func() time.Time

	// writes serialises writers inside this process. SQLite in WAL mode allows
	// one writer at a time; without this, concurrent claims fail with
	// SQLITE_BUSY and the retry storm looks like a deadlock.
	writes sync.Mutex
}

// Options configure the queue.
type Options struct {
	// Path is the database file. Empty opens an in-memory database, which is
	// what the tests use and what a run with no state directory falls back to.
	Path string

	// Clock is injected so a test can control lease expiry without sleeping.
	Clock func() time.Time
}

// Open creates or opens the queue database.
func Open(opts Options) (*Queue, error) {
	clock := opts.Clock
	if clock == nil {
		clock = time.Now //nolint:forbidigo // the adapter takes a clock; this is the default it falls back to
	}

	dsn := "file::memory:?cache=shared&_pragma=busy_timeout(5000)"
	if opts.Path != "" {
		if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
			return nil, err
		}
		// WAL is the original's journal mode and the right one here: a reader
		// counting the queue must not block the worker draining it.
		dsn = "file:" + opts.Path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection. SQLite serialises writes anyway, and a pool over a single
	// file mostly produces contention that looks like a bug.
	db.SetMaxOpenConns(1)

	q := &Queue{db: db, clock: clock}
	if err := q.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return q, nil
}

// Close releases the database.
func (q *Queue) Close() error { return q.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS jobs (
	id          TEXT PRIMARY KEY,
	queue       TEXT NOT NULL,
	kind        TEXT NOT NULL,
	payload     BLOB,
	workspace   TEXT NOT NULL DEFAULT '',
	status      TEXT NOT NULL,
	attempts    INTEGER NOT NULL DEFAULT 0,
	max_tries   INTEGER NOT NULL DEFAULT 3,
	run_at      INTEGER NOT NULL,
	lease_until INTEGER,
	claimed_by  TEXT NOT NULL DEFAULT '',
	result      BLOB,
	error       TEXT NOT NULL DEFAULT '',
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL,
	started_at  INTEGER,
	ended_at    INTEGER
);

-- The claim's index. Without it every claim scans the table, which is fine at a
-- hundred rows and not at a hundred thousand.
CREATE INDEX IF NOT EXISTS jobs_claimable ON jobs (status, queue, run_at);
CREATE INDEX IF NOT EXISTS jobs_lease ON jobs (status, lease_until);
CREATE INDEX IF NOT EXISTS jobs_workspace ON jobs (workspace, status);
`

func (q *Queue) migrate(ctx context.Context) error {
	if _, err := q.db.ExecContext(ctx, schema); err != nil {
		return err
	}
	// The subconscious deduplication set shares this database, for the reason
	// given in signatures.go: it is operational state, not domain.
	_, err := q.db.ExecContext(ctx, signatureSchema)
	return err
}

// Enqueue records a new job.
func (q *Queue) Enqueue(ctx context.Context, j job.Job) (string, error) {
	if strings.TrimSpace(j.ID) == "" {
		return "", errors.New("sqlitequeue: a job needs an identifier")
	}
	if strings.TrimSpace(j.Queue) == "" {
		return "", errors.New("sqlitequeue: a job needs a queue")
	}
	now := q.clock()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	if j.RunAt.IsZero() {
		j.RunAt = now
	}
	if j.MaxTries <= 0 {
		j.MaxTries = job.DefaultMaxTries
	}
	j.Status = job.Pending
	j.UpdatedAt = now

	q.writes.Lock()
	defer q.writes.Unlock()

	_, err := q.db.ExecContext(ctx, `
		INSERT INTO jobs (id, queue, kind, payload, workspace, status, attempts, max_tries,
		                  run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
		j.ID, j.Queue, j.Kind, []byte(j.Payload), j.Workspace, string(j.Status), j.MaxTries,
		stamp(j.RunAt), stamp(j.CreatedAt), stamp(j.UpdatedAt))
	if err != nil {
		return "", err
	}
	return j.ID, nil
}

// Claim takes the next eligible job and holds it for lease.
//
// This is the operation the whole package exists for. The UPDATE ... WHERE
// id = (SELECT ... LIMIT 1) runs inside one statement, so two workers arriving
// at the same instant cannot both take the same row: the second one's WHERE no
// longer matches. That is the guarantee a filesystem queue cannot give without
// reimplementing a database, badly.
func (q *Queue) Claim(ctx context.Context, queues []string, worker string, lease time.Duration) (*job.Job, error) {
	if lease <= 0 {
		lease = job.DefaultLease
	}
	now := q.clock()
	until := now.Add(lease)

	filter, names := queueFilter(queues)
	// In statement order: the four SET columns, the started_at coalesce, then
	// the two WHERE terms and the queue names. Getting this wrong is silent —
	// SQLite happily binds a status string to an integer column — so the order
	// here mirrors the statement below line for line.
	args := []any{
		string(job.Claimed), stamp(until), worker, stamp(now),
		stamp(now),
		string(job.Pending), stamp(now),
	}
	args = append(args, names...)

	q.writes.Lock()
	defer q.writes.Unlock()

	// The ordering is the original's recovery-first: the oldest eligible job
	// goes next, so a retry that has been waiting does not starve behind a
	// stream of fresh work.
	row := q.db.QueryRowContext(ctx, `
		UPDATE jobs
		   SET status = ?, lease_until = ?, claimed_by = ?, updated_at = ?,
		       attempts = attempts + 1,
		       started_at = COALESCE(started_at, ?)
		 WHERE id = (
		       SELECT id FROM jobs
		        WHERE status = ? AND run_at <= ? `+filter+`
		        ORDER BY run_at ASC, created_at ASC
		        LIMIT 1
		 )
		RETURNING `+columns, args...)

	got, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no work is the common case, not an error
	}
	if err != nil {
		return nil, err
	}
	return got, nil
}

// Heartbeat extends the lease of a job this worker still holds.
//
// The worker is part of the WHERE: a process that was reaped and came back must
// not be able to extend a lease on work that has since been handed to somebody
// else.
func (q *Queue) Heartbeat(ctx context.Context, jobID, worker string, lease time.Duration) error {
	if lease <= 0 {
		lease = job.DefaultLease
	}
	now := q.clock()

	q.writes.Lock()
	defer q.writes.Unlock()

	res, err := q.db.ExecContext(ctx, `
		UPDATE jobs SET lease_until = ?, updated_at = ?
		 WHERE id = ? AND claimed_by = ? AND status = ?`,
		stamp(now.Add(lease)), stamp(now), jobID, worker, string(job.Claimed))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("sqlitequeue: %s is no longer held by %s", jobID, worker)
	}
	return nil
}

// Complete records a job that finished.
func (q *Queue) Complete(ctx context.Context, jobID string, result json.RawMessage) error {
	now := q.clock()

	q.writes.Lock()
	defer q.writes.Unlock()

	_, err := q.db.ExecContext(ctx, `
		UPDATE jobs
		   SET status = ?, result = ?, error = '', lease_until = NULL, claimed_by = '',
		       updated_at = ?, ended_at = ?
		 WHERE id = ?`,
		string(job.Succeeded), []byte(result), stamp(now), stamp(now), jobID)
	return err
}

// Fail records an attempt that did not work, scheduling a retry or giving up.
func (q *Queue) Fail(ctx context.Context, jobID string, cause error, retryIn *time.Duration) error {
	current, err := q.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("sqlitequeue: no job %s", jobID)
	}

	now := q.clock()
	message := ""
	if cause != nil {
		message = cause.Error()
	}

	if current.Exhausted() {
		q.writes.Lock()
		defer q.writes.Unlock()
		_, err := q.db.ExecContext(ctx, `
			UPDATE jobs
			   SET status = ?, error = ?, lease_until = NULL, claimed_by = '',
			       updated_at = ?, ended_at = ?
			 WHERE id = ?`,
			string(job.Dead), message, stamp(now), stamp(now), jobID)
		return err
	}

	wait := job.Backoff(current.Attempts)
	if retryIn != nil {
		wait = *retryIn
	}

	q.writes.Lock()
	defer q.writes.Unlock()

	_, err = q.db.ExecContext(ctx, `
		UPDATE jobs
		   SET status = ?, error = ?, lease_until = NULL, claimed_by = '',
		       run_at = ?, updated_at = ?
		 WHERE id = ?`,
		string(job.Pending), message, stamp(now.Add(wait)), stamp(now), jobID)
	return err
}

// RecoverStale hands back jobs whose lease lapsed.
//
// A job whose worker died has already had its attempt counted, so a job that is
// reaped repeatedly still reaches its limit and dies rather than cycling
// forever.
func (q *Queue) RecoverStale(ctx context.Context) (int, error) {
	now := q.clock()

	q.writes.Lock()
	defer q.writes.Unlock()

	res, err := q.db.ExecContext(ctx, `
		UPDATE jobs
		   SET status = ?, claimed_by = '', lease_until = NULL, run_at = ?, updated_at = ?
		 WHERE status = ? AND lease_until IS NOT NULL AND lease_until < ?`,
		string(job.Pending), stamp(now), stamp(now), string(job.Claimed), stamp(now))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// Get reads one job.
func (q *Queue) Get(ctx context.Context, jobID string) (*job.Job, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+columns+` FROM jobs WHERE id = ?`, jobID)
	got, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // absence is an answer here
	}
	return got, err
}

// List reads the queue.
func (q *Queue) List(ctx context.Context, f job.Filter) ([]job.Job, error) {
	query := `SELECT ` + columns + ` FROM jobs WHERE 1=1`
	var args []any
	if f.Queue != "" {
		query += " AND queue = ?"
		args = append(args, f.Queue)
	}
	if f.Status != "" {
		query += " AND status = ?"
		args = append(args, string(f.Status))
	}
	if f.Workspace != "" {
		query += " AND workspace = ?"
		args = append(args, f.Workspace)
	}
	if f.Kind != "" {
		query += " AND kind = ?"
		args = append(args, f.Kind)
	}
	query += " ORDER BY created_at DESC, id DESC"
	if f.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []job.Job
	for rows.Next() {
		got, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *got)
	}
	return out, rows.Err()
}

// Purge removes terminal jobs older than the window.
func (q *Queue) Purge(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := q.clock().Add(-olderThan)

	q.writes.Lock()
	defer q.writes.Unlock()

	res, err := q.db.ExecContext(ctx, `
		DELETE FROM jobs WHERE status IN (?, ?) AND updated_at < ?`,
		string(job.Succeeded), string(job.Dead), stamp(cutoff))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

const columns = `id, queue, kind, payload, workspace, status, attempts, max_tries,
	run_at, lease_until, claimed_by, result, error, created_at, updated_at, started_at, ended_at`

// scanner is what QueryRow and Rows have in common.
type scanner interface{ Scan(dest ...any) error }

func scanJob(s scanner) (*job.Job, error) {
	var (
		j          job.Job
		payload    []byte
		result     []byte
		status     string
		runAt      int64
		createdAt  int64
		updatedAt  int64
		leaseUntil sql.NullInt64
		startedAt  sql.NullInt64
		endedAt    sql.NullInt64
	)
	err := s.Scan(
		&j.ID, &j.Queue, &j.Kind, &payload, &j.Workspace, &status, &j.Attempts, &j.MaxTries,
		&runAt, &leaseUntil, &j.ClaimedBy, &result, &j.Error, &createdAt, &updatedAt,
		&startedAt, &endedAt,
	)
	if err != nil {
		return nil, err
	}
	j.Status = job.Status(status)
	j.Payload = json.RawMessage(payload)
	j.Result = json.RawMessage(result)
	j.RunAt = unstamp(runAt)
	j.CreatedAt = unstamp(createdAt)
	j.UpdatedAt = unstamp(updatedAt)
	j.LeaseUntil = nullTime(leaseUntil)
	j.StartedAt = nullTime(startedAt)
	j.EndedAt = nullTime(endedAt)
	return &j, nil
}

// queueFilter builds the IN clause for the queues a worker drains. Empty means
// every queue, which is what a single-worker installation wants.
func queueFilter(queues []string) (string, []any) {
	names := make([]string, 0, len(queues))
	for _, q := range queues {
		if strings.TrimSpace(q) != "" {
			names = append(names, q)
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	args := make([]any, 0, len(names))
	for _, n := range names {
		args = append(args, n)
	}
	return " AND queue IN (?" + strings.Repeat(",?", len(names)-1) + ")", args
}

// Times are stored as Unix nanoseconds. Integers compare and index correctly
// whatever the machine's locale, which is not true of a formatted string.
func stamp(t time.Time) int64 { return t.UTC().UnixNano() }

func unstamp(n int64) time.Time { return time.Unix(0, n).UTC() }

func nullTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := unstamp(v.Int64)
	return &t
}
