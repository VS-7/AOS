package supervise_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/domain/gateway"
)

func ctx() context.Context { return context.Background() }

// TestARealProcessIsStartedSignalledAndReaped exercises the whole process
// driver against the operating system, because that is the only place its
// behaviour actually lives.
func TestARealProcessIsStartedSignalledAndReaped(t *testing.T) {
	procs := supervise.NewProcesses()
	log := filepath.Join(t.TempDir(), "logs", "daemon.log")

	pid, err := procs.Start(ctx(), gateway.Command{
		Path: sleepBinary(t), Args: []string{"30"}, LogFile: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = procs.Kill(pid) })

	if !procs.Alive(pid) {
		t.Fatal("the process is not alive right after being started")
	}
	if _, err := os.Stat(filepath.Dir(log)); err != nil {
		t.Errorf("the log directory was not created: %v", err)
	}

	if err := procs.Terminate(pid); err != nil {
		t.Fatal(err)
	}
	waitGone(t, procs, pid)
}

// TestAProcessThatIgnoresTheRequestIsKilled: the escalation the gateway relies
// on has to work against a real process that traps the signal.
func TestAProcessThatIgnoresTheRequestIsKilled(t *testing.T) {
	procs := supervise.NewProcesses()
	script := filepath.Join(t.TempDir(), "stubborn.sh")
	body := "#!/bin/sh\ntrap '' TERM\nwhile true; do sleep 0.1; done\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil { //nolint:gosec // a test fixture that must be executable
		t.Fatal(err)
	}

	pid, err := procs.Start(ctx(), gateway.Command{Path: script})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = procs.Kill(pid) })

	if err := procs.Terminate(pid); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if !procs.Alive(pid) {
		t.Skip("this shell does not trap TERM the way the test assumes")
	}
	if err := procs.Kill(pid); err != nil {
		t.Fatal(err)
	}
	waitGone(t, procs, pid)
}

func TestAliveIsFalseForNothingAndForGarbage(t *testing.T) {
	procs := supervise.NewProcesses()
	for _, pid := range []int{0, -1, -12345} {
		if procs.Alive(pid) {
			t.Errorf("pid %d reported alive", pid)
		}
	}
}

// TestTheHealthProbeDistinguishesServingFromListening is the point of probing
// rather than checking liveness: a server that answers 500 is running and is
// not serving.
func TestTheHealthProbeDistinguishesServingFromListening(t *testing.T) {
	health := supervise.NewHealth()

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	host, port := splitHostPort(t, ok.URL)
	if err := health.Probe(ctx(), host, port); err != nil {
		t.Fatalf("a healthy daemon was reported unhealthy: %v", err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	host, port = splitHostPort(t, broken.URL)
	if err := health.Probe(ctx(), host, port); err == nil {
		t.Fatal("a daemon answering 500 was reported healthy")
	}
}

func TestTheHealthProbeFailsFastOnAClosedPort(t *testing.T) {
	health := supervise.NewHealth()
	health.Timeout = 500 * time.Millisecond

	start := time.Now()
	// Port 1 is privileged and closed; nothing will answer.
	if err := health.Probe(ctx(), "127.0.0.1", 1); err == nil {
		t.Fatal("something answered on port 1")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the probe took %s — it is meant to be short, because it runs in a loop", elapsed)
	}
}

func TestTheRecordRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "gateway.json")
	store := supervise.NewStore(path)

	if got, err := store.Read(ctx()); err != nil || got != nil {
		t.Fatalf("with no file, read = %+v, %v", got, err)
	}

	want := gateway.Meta{
		PID: 4242, Port: 5326, Host: "127.0.0.1",
		StartedAt: time.Unix(0, 0).UTC(), Version: "v0.4.0",
		Command: "/usr/local/bin/aosd", Args: []string{"--serve"},
	}
	if err := store.Write(ctx(), want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PID != want.PID || got.Port != want.Port || got.Command != want.Command {
		t.Fatalf("record = %+v", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %04o", perm)
	}

	if err := store.Clear(ctx()); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Read(ctx()); err != nil || got != nil {
		t.Fatalf("after clearing, read = %+v, %v", got, err)
	}
	// Clearing what is not there is not an error: the caller asked for a state,
	// and that state holds.
	if err := store.Clear(ctx()); err != nil {
		t.Errorf("clearing twice = %v", err)
	}
}

// TestACorruptRecordReadsAsNoRecord: a record that does not parse names no
// process, and refusing to start because of it would need a person to delete a
// file by hand.
func TestACorruptRecordReadsAsNoRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	for _, body := range []string{"{not json", "{}", `{"pid":0}`, `{"pid":-1}`} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := supervise.NewStore(path).Read(ctx())
		if err != nil {
			t.Fatalf("%q: %v", body, err)
		}
		if got != nil {
			t.Errorf("%q read as a record: %+v", body, got)
		}
	}
}

// TestTheLockIsHeldAcrossProcesses is what defect #18 is about. Two supervisors
// in one process are the cheap case; this checks the file lock itself, which is
// the mechanism that also works between two terminals.
func TestTheLockIsHeldAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.lock")

	first := supervise.NewLock(path)
	unlock, err := first.Lock(ctx())
	if err != nil {
		t.Fatal(err)
	}

	second := supervise.NewLock(path)
	second.Timeout = 200 * time.Millisecond
	if _, err := second.Lock(ctx()); err == nil {
		t.Fatal("the lock was granted twice")
	}

	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	again, err := second.Lock(ctx())
	if err != nil {
		t.Fatalf("the lock was not released: %v", err)
	}
	if err := again(); err != nil {
		t.Fatal(err)
	}
}

// TestTheLockDoesNotWaitForever: a crashed supervisor must not leave the
// gateway permanently unusable.
func TestTheLockDoesNotWaitForever(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.lock")
	held := supervise.NewLock(path)
	unlock, err := held.Lock(ctx())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()

	waiting := supervise.NewLock(path)
	waiting.Timeout = 150 * time.Millisecond
	start := time.Now()
	if _, err := waiting.Lock(ctx()); err == nil {
		t.Fatal("expected a timeout")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("waited %s for a lock with a 150ms budget", elapsed)
	}
}

func TestTheLockIsExclusiveUnderConcurrency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.lock")

	var mu sync.Mutex
	var concurrent, peak int
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := supervise.NewLock(path)
			l.Timeout = 5 * time.Second
			unlock, err := l.Lock(ctx())
			if err != nil {
				return
			}
			mu.Lock()
			concurrent++
			if concurrent > peak {
				peak = concurrent
			}
			mu.Unlock()

			time.Sleep(5 * time.Millisecond)

			mu.Lock()
			concurrent--
			mu.Unlock()
			_ = unlock()
		}()
	}
	wg.Wait()
	if peak > 1 {
		t.Fatalf("%d holders at once", peak)
	}
}

// TestAnExplicitPathWins: the desktop app ships its daemon somewhere no PATH
// knows about, and pointing at it is the whole mechanism.
func TestAnExplicitPathWins(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "custom-daemon")
	if err := os.WriteFile(explicit, []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // fixture
		t.Fatal(err)
	}

	got, err := supervise.Resolver{Explicit: explicit, Args: []string{"--serve"}}.Resolve(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != explicit {
		t.Fatalf("path = %q", got.Path)
	}
	if len(got.Args) != 1 || got.Args[0] != "--serve" {
		t.Fatalf("args = %v", got.Args)
	}
}

func TestAnUnresolvableBinaryIsReportedWithWhatWasTried(t *testing.T) {
	_, err := supervise.Resolver{Explicit: filepath.Join(t.TempDir(), "nope")}.Resolve(ctx())
	if err == nil {
		t.Skip("a daemon binary is on PATH in this environment, so there is nothing to fail")
	}
	if !containsAll(err.Error(), "could not find") {
		t.Fatalf("error = %v", err)
	}
}

func TestTheSleeperStopsWhenTheContextEnds(t *testing.T) {
	cancelled, cancel := context.WithCancel(ctx())
	cancel()
	start := time.Now()
	if err := (supervise.Sleeper{}).Sleep(cancelled, time.Hour); err == nil {
		t.Fatal("a cancelled sleep should report the cancellation")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the sleep took %s despite a cancelled context", elapsed)
	}
}

func sleepBinary(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary on this machine")
	}
	return path
}

func waitGone(t *testing.T, procs supervise.Processes, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !procs.Alive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process %d is still alive", pid)
}

func splitHostPort(t *testing.T, raw string) (string, int) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return u.Hostname(), port
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
