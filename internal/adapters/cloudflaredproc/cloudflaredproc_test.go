package cloudflaredproc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/domain/tunnel"
)

// fakeCloudflared writes a shell script standing in for the real binary, so
// this test proves the real Spawn/Wait/Stop wiring without depending on
// cloudflared actually being installed. Unix only — os/exec + a shell script
// is the simplest fake; Windows CI, if any, skips this file's tests.
func fakeCloudflared(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary is a shell script; unix only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudflared")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSpawnSucceedsWhenCloudflaredReportsReady(t *testing.T) {
	dir := fakeCloudflared(t, `echo "Registered tunnel connection" >&2
sleep 5
`)
	r := Runner{Binary: filepath.Join(dir, "cloudflared")}
	// A ceiling, not the assertion. What this test claims is that a
	// cloudflared which reports a connection makes Spawn return a process —
	// how many seconds the fork, the exec and the first line of stderr take
	// is not part of that claim. Three seconds was: under `go test -race
	// ./...` the whole tree competes for the same cores, and spawning a
	// shell script inside that took longer than the wait, so this test failed
	// on load rather than on behaviour. The timeout path has its own tests
	// below, which assert it deliberately and cheaply.
	proc, err := r.Spawn(context.Background(), "example.com", "tok", 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = proc.Stop() }()
	if proc.PID() == 0 {
		t.Fatal("expected a nonzero PID")
	}
}

func TestSpawnReturnsErrBinaryMissingWhenNotOnPath(t *testing.T) {
	r := Runner{Binary: filepath.Join(t.TempDir(), "does-not-exist")}
	_, err := r.Spawn(context.Background(), "example.com", "tok", time.Second)
	if !errors.Is(err, tunnel.ErrBinaryMissing) {
		t.Fatalf("expected ErrBinaryMissing, got %v", err)
	}
}

func TestSpawnReturnsErrReadinessTimeoutWhenNeverReady(t *testing.T) {
	dir := fakeCloudflared(t, `sleep 5
`)
	r := Runner{Binary: filepath.Join(dir, "cloudflared")}
	_, err := r.Spawn(context.Background(), "example.com", "tok", 200*time.Millisecond)
	if !errors.Is(err, tunnel.ErrReadinessTimeout) {
		t.Fatalf("expected ErrReadinessTimeout, got %v", err)
	}
}

func TestSpawnReturnsErrReadinessTimeoutWhenProcessExitsEarly(t *testing.T) {
	dir := fakeCloudflared(t, `exit 1
`)
	r := Runner{Binary: filepath.Join(dir, "cloudflared")}
	_, err := r.Spawn(context.Background(), "example.com", "tok", 3*time.Second)
	if !errors.Is(err, tunnel.ErrReadinessTimeout) {
		t.Fatalf("expected ErrReadinessTimeout for an early exit, got %v", err)
	}
}

func TestStopSignalsTheProcess(t *testing.T) {
	// No trap: SIGTERM's default disposition ends the shell (this test's
	// direct child) even though the grandchild `sleep` does not itself
	// receive it — enough to prove Stop reaches the process Spawn started
	// and Wait unblocks, without depending on trap semantics varying by
	// shell.
	dir := fakeCloudflared(t, `echo "Registered tunnel connection" >&2
sleep 30
`)
	r := Runner{Binary: filepath.Join(dir, "cloudflared")}
	// A ceiling, not the assertion — the same reason
	// TestSpawnSucceedsWhenCloudflaredReportsReady carries thirty seconds.
	// Three was enough alone and not under `go test -race ./...`, where the
	// whole tree competes for the same cores and a fork plus an exec plus the
	// first line of stderr can take longer than the wait. What this test
	// claims is that Stop reaches the process, which the timeout below is
	// what actually bounds.
	proc, err := r.Spawn(context.Background(), "example.com", "tok", 30*time.Second)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if err := proc.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- proc.Wait() }()
	select {
	case <-waitErr:
		// Any return — clean or signalled — means Stop reached the process.
	case <-time.After(3 * time.Second):
		t.Fatal("process did not exit after Stop")
	}
	_ = syscall.SIGTERM // documents which signal Stop sends, for the reader
}

// The pipe keeps being read after readiness.
//
// It used to be abandoned at the marker, so cloudflared's own logging filled
// the pipe's buffer and then blocked on its next write — the tunnel stalled,
// with nothing in any log to say so, after however many minutes of output it
// took to fill 64 KiB. A binary that keeps talking must keep being heard.
func TestStderrKeepsBeingReadAfterReadiness(t *testing.T) {
	// Well past any pipe buffer: 4000 lines of 200 characters is ~800 KB.
	dir := fakeCloudflared(t, `echo "Registered tunnel connection" >&2
i=0
while [ $i -lt 4000 ]; do
  echo "2026-08-27T00:00:00Z INF a log line that is long enough to matter, repeated until the pipe would have filled and the writer would have blocked forever" >&2
  i=$((i+1))
done
echo "done" >&2
`)
	r := Runner{Binary: filepath.Join(dir, "cloudflared")}
	proc, err := r.Spawn(context.Background(), "example.com", "tok", 30*time.Second)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer func() { _ = proc.Stop() }()

	// The fake exits on its own once it has written everything. If the reader
	// stopped at the marker, the writer blocks and this never returns.
	done := make(chan error, 1)
	go func() { done <- proc.Wait() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("cloudflared never finished writing: the stderr pipe filled and the process blocked on it")
	}
}
