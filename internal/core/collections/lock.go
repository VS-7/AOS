package collections

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// lockShards keeps contention on the mutex map low without allocating a mutex
// per path in a workspace that may hold tens of thousands of records.
const lockShards = 64

// PathLock serialises writers per canonical path within the process, and across
// processes via an advisory lock file.
//
// Both halves are necessary. In-process: the daemon runs twenty job workers and
// N parallel instances of the same agent, all sharing one memory graph. Across
// processes: the CLI writes directly when the daemon is stopped, and nine
// simultaneous MCP clients were observed on the reference machine.
//
// Advisory locks are not mandatory locks: a text editor saving the same file
// ignores them. That is inherent to a format meant to be edited by hand, and is
// why the engine also has a watcher and optimistic concurrency.
type PathLock struct {
	shards [lockShards]shard

	// lockDir holds the advisory lock files. It is deliberately NOT the
	// directory of the record: those live in the user's Git repository, and a
	// stray .lock file there would be committed, diffed and complained about.
	// An empty lockDir disables inter-process locking, which is what tests and
	// single-process tools use.
	lockDir string
}

type shard struct {
	mu    sync.Mutex
	holds map[string]*hold
}

type hold struct {
	mu   sync.Mutex
	refs int
}

// NewPathLock builds a lock. Pass the directory for advisory lock files —
// ~/.aos/runtime/locks in a real deployment — in any process that shares a
// workspace with another one. An empty string keeps locking in-process only.
//
// Two processes agree on a lock file because both derive its name from the
// canonical path of the record, so no coordination is needed beyond sharing the
// directory.
func NewPathLock(lockDir string) *PathLock {
	l := &PathLock{lockDir: lockDir}
	for i := range l.shards {
		l.shards[i].holds = map[string]*hold{}
	}
	return l
}

// With runs fn while holding the lock for path. Paths are canonicalised first,
// so "./a/../b.md" and "b.md" contend on the same mutex.
func (l *PathLock) With(ctx context.Context, path string, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	canonical := canonicalPath(path)
	h := l.acquire(canonical)
	defer l.release(canonical, h)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Re-check after waiting: a caller cancelled while queued must not proceed.
	if err := ctx.Err(); err != nil {
		return err
	}

	if l.lockDir == "" {
		return fn()
	}
	lockFile, err := l.lockFileFor(canonical)
	if err != nil {
		return IOError("lock", path, err)
	}
	fl := flock.New(lockFile)
	locked, err := fl.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return IOError("lock", path, err)
	}
	if !locked {
		return IOError("lock", path, context.DeadlineExceeded)
	}
	defer func() { _ = fl.Unlock() }()
	return fn()
}

// WithAll locks several paths at once, always in lexicographic order. Ordering
// is what makes a multi-file operation deadlock-free; it is enforced here so no
// caller has to remember.
func (l *PathLock) WithAll(ctx context.Context, paths []string, fn func() error) error {
	ordered := make([]string, len(paths))
	copy(ordered, paths)
	sort.Strings(ordered)

	var run func(i int) error
	run = func(i int) error {
		if i == len(ordered) {
			return fn()
		}
		return l.With(ctx, ordered[i], func() error { return run(i + 1) })
	}
	return run(0)
}

func (l *PathLock) shardFor(path string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(path))
	return &l.shards[h.Sum32()%lockShards]
}

func (l *PathLock) acquire(path string) *hold {
	s := l.shardFor(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.holds[path]
	if !ok {
		h = &hold{}
		s.holds[path] = h
	}
	h.refs++
	return h
}

// release drops the entry once nobody is waiting on it, so the map does not
// grow to one entry per record ever written.
func (l *PathLock) release(path string, h *hold) {
	s := l.shardFor(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	h.refs--
	if h.refs == 0 {
		delete(s.holds, path)
	}
}

// lockFileFor names the advisory lock of a record: the hash of its canonical
// path, inside the lock directory. Hashing keeps the name short and free of
// separators, and makes it independent of how deep the record lives.
func (l *PathLock) lockFileFor(canonical string) (string, error) {
	if err := os.MkdirAll(l.lockDir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(canonical))
	return filepath.Join(l.lockDir, hex.EncodeToString(sum[:16])+".lock"), nil
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

// HeldCount reports how many paths currently have a lock entry. It exists so a
// test can prove the map does not grow to one entry per record ever written.
func (l *PathLock) HeldCount() int {
	n := 0
	for i := range l.shards {
		l.shards[i].mu.Lock()
		n += len(l.shards[i].holds)
		l.shards[i].mu.Unlock()
	}
	return n
}
