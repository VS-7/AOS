// Package supervise is the operating system as the gateway sees it: spawning a
// detached process, signalling it, probing its health, and holding a lock while
// doing so.
//
// The five ports live in one package because they are five halves of one job
// and change together — a change to how the daemon is spawned is a change to
// how it is found and to what is written down about it.
package supervise

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/gofrs/flock"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/safe"
	"github.com/OWNER/aos/internal/domain/gateway"
)

// Processes spawns and signals daemons.
type Processes struct{}

// NewProcesses builds the process driver.
func NewProcesses() Processes { return Processes{} }

// Start launches a detached daemon and returns its identifier.
//
// Detached is the requirement: the daemon must outlive the terminal that
// started it, so it gets its own process group and its output goes to a file
// rather than to a console that is about to close.
func (Processes) Start(ctx context.Context, cmd gateway.Command) (int, error) {
	if cmd.LogFile != "" {
		if err := os.MkdirAll(filepath.Dir(cmd.LogFile), 0o700); err != nil {
			return 0, err
		}
	}

	// Deliberately not exec.CommandContext: cancelling the context that started
	// a daemon must not kill the daemon. The whole point is that it outlives
	// the call.
	c := exec.Command(cmd.Path, cmd.Args...) //nolint:noctx // see above
	c.Dir = cmd.Dir
	c.Env = cmd.Env
	if c.Env == nil {
		c.Env = os.Environ()
	}

	if cmd.LogFile != "" {
		// Append: a restart must not erase the log that explains the last exit.
		log, err := os.OpenFile(cmd.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return 0, err
		}
		defer func() { _ = log.Close() }()
		c.Stdout, c.Stderr = log, log
	}
	detach(c)

	if err := c.Start(); err != nil {
		return 0, err
	}
	pid := c.Process.Pid

	// The child is waited on in the background, and this is not optional.
	//
	// A daemon that dies while its supervisor is still running becomes a
	// zombie until somebody reaps it, and a zombie's pid is still in the
	// process table — so `kill -0` succeeds and Alive reports a dead daemon as
	// running. The command-line tool never notices, because it exits moments
	// later and the kernel reparents the child. The desktop app would notice:
	// it starts the daemon and stays up for hours.
	//
	// Waiting does not attach the child to this process's lifetime. It keeps
	// running if this one exits, at which point init inherits it.
	_ = safe.Go(context.WithoutCancel(ctx), "reap "+filepath.Base(cmd.Path),
		func(context.Context) error {
			//nolint:errcheck // the exit status is the daemon's business, not the supervisor's
			_ = c.Wait()
			return nil
		})

	return pid, nil
}

// Alive reports whether a process exists.
//
// On Unix this is signal 0, which performs the permission and existence checks
// without delivering anything.
func (Processes) Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// Terminate asks a process to stop.
func (Processes) Terminate(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return terminate(p)
}

// Kill stops a process that did not listen.
func (Processes) Kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// Health probes the daemon's health endpoint over HTTP.
type Health struct {
	// Timeout bounds one probe. It is short because the probe runs in a loop:
	// a slow answer is indistinguishable from no answer, and waiting on it
	// only delays the next attempt.
	Timeout time.Duration
}

// NewHealth builds the health prober.
func NewHealth() *Health { return &Health{Timeout: 2 * time.Second} }

// Probe returns nil when the daemon answered.
func (h *Health) Probe(ctx context.Context, host string, port int) error {
	timeout := h.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/api/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint answered %d", res.StatusCode)
	}
	return nil
}

// Store holds the record of the running daemon.
type Store struct{ path string }

// NewStore builds the record store over a file path.
func NewStore(path string) *Store { return &Store{path: path} }

// Read returns the record, or nil when there is none.
func (s *Store) Read(ctx context.Context) (*gateway.Meta, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var meta gateway.Meta
	if err := jsonUnmarshal(raw, &meta); err != nil {
		// A record that does not parse names no process, which is the same
		// situation as no record at all — and refusing to start because of a
		// corrupt file would need a person to delete it by hand.
		return nil, nil
	}
	if meta.PID <= 0 {
		return nil, nil
	}
	return &meta, nil
}

// Write replaces the record atomically.
func (s *Store) Write(ctx context.Context, m gateway.Meta) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := jsonMarshalIndent(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return atomicfs.WriteFile(s.path, raw, 0o600)
}

// Clear removes the record.
func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Lock is the cross-process lock the supervisor takes before reading state.
type Lock struct {
	path string

	// Timeout bounds the wait for the lock. Waiting forever would turn a
	// crashed supervisor into a permanently unusable gateway.
	Timeout time.Duration
}

// NewLock builds the lock over a file path.
func NewLock(path string) *Lock { return &Lock{path: path, Timeout: 10 * time.Second} }

// Lock acquires the file lock and returns the release function.
func (l *Lock) Lock(ctx context.Context) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return nil, err
	}
	timeout := l.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fl := flock.New(l.path)
	got, err := fl.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !got {
		return nil, errors.New("the gateway lock is held by another process")
	}
	return fl.Unlock, nil
}

// Resolver finds the daemon binary.
type Resolver struct {
	// Explicit is a path given by configuration. The desktop app sets it,
	// because the binary it ships alongside is not on any PATH.
	Explicit string

	// Args and Dir are passed to whatever is resolved.
	Args []string
	Dir  string
	Log  string
	Env  []string
}

// Resolve mirrors the original's cascade: an explicit path, then a binary
// sitting next to this executable, then whatever is on PATH.
//
// The order is from most specific to least, and each step is a different kind
// of installation: a packaged desktop app, a pair of binaries installed
// together, and a development machine.
func (r Resolver) Resolve(_ context.Context) (gateway.Command, error) {
	var tried []string

	candidates := []string{}
	if r.Explicit != "" {
		candidates = append(candidates, r.Explicit)
	}
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), daemonName()))
	}

	for _, c := range candidates {
		tried = append(tried, c)
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return r.commandFor(c), nil
		}
	}

	if found, err := exec.LookPath(daemonName()); err == nil {
		return r.commandFor(found), nil
	}
	tried = append(tried, daemonName()+" (on PATH)")

	return gateway.Command{}, errBinaryNotFound(tried)
}

func (r Resolver) commandFor(path string) gateway.Command {
	return gateway.Command{Path: path, Args: r.Args, Dir: r.Dir, Env: r.Env, LogFile: r.Log}
}

// daemonName is the daemon's executable name on this platform.
func daemonName() string {
	name := build.Name + "d"
	if isWindows() {
		return name + ".exe"
	}
	return name
}

// Sleeper waits, and stops waiting when the context is cancelled.
type Sleeper struct{}

// Sleep blocks for d, or until ctx ends.
func (Sleeper) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func errBinaryNotFound(tried []string) error {
	return apperr.New("GATEWAY_BINARY_NOT_FOUND").
		Causer("supervise.Resolver.Resolve").
		Msgf("could not find the %s binary", daemonName()).
		Issue("tried", tried).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "install it beside " + build.Name + ", or point at it with " + env.Key("DAEMON_PATH"),
		})
}
