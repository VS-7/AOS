// Package cloudflaredproc spawns and supervises the system's cloudflared
// binary — the process side of internal/domain/tunnel.Runner. It lives under
// internal/adapters, not internal/domain/tunnel, because os/exec is forbidden
// under internal/domain (see internal/architecture/rules.go).
package cloudflaredproc

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/domain/tunnel"
)

// readyMarker is the line cloudflared writes to stderr once a named tunnel's
// connection registers. cloudflaredproc_test.go's fake binary prints the
// identical line, which is what makes Spawn testable without the real one.
const readyMarker = "Registered tunnel connection"

// Runner is the real tunnel.Runner, over the system's cloudflared.
type Runner struct {
	// Binary overrides "cloudflared" — used by tests to point at a fake
	// script instead of the real binary.
	Binary string
}

// New builds a Runner over the system's cloudflared.
func New() Runner { return Runner{Binary: "cloudflared"} }

// Spawn starts cloudflared for hostname/token and waits for it to report a
// registered connection, up to timeout. On any failure the process it
// started (if any) is already killed.
func (r Runner) Spawn(ctx context.Context, hostname, token string, timeout time.Duration) (tunnel.Process, error) {
	binary := r.Binary
	if binary == "" {
		binary = "cloudflared"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return nil, fmt.Errorf("%w: %v", tunnel.ErrBinaryMissing, err)
	}

	cmd := exec.Command(binary, "tunnel", "--hostname", hostname, "run", "--token", token)
	detach(cmd)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("piping cloudflared's stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting cloudflared: %w", err)
	}

	ready := make(chan struct{})
	exited := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), readyMarker) {
				close(ready)
				return
			}
		}
	}()
	go func() { exited <- cmd.Wait() }()

	select {
	case <-ready:
		return &process{cmd: cmd, exited: exited}, nil
	case err := <-exited:
		if err == nil {
			err = fmt.Errorf("cloudflared exited before reporting a connection")
		}
		return nil, fmt.Errorf("%w: %v", tunnel.ErrReadinessTimeout, err)
	case <-time.After(timeout):
		_ = terminate(cmd.Process)
		<-exited
		return nil, fmt.Errorf("%w: no connection reported within %s", tunnel.ErrReadinessTimeout, timeout)
	}
}

// process adapts a running *exec.Cmd to tunnel.Process.
type process struct {
	cmd    *exec.Cmd
	exited chan error
}

func (p *process) PID() int { return p.cmd.Process.Pid }

// Wait blocks until the process exits — Spawn's own goroutine already called
// cmd.Wait(), so this reads its result rather than calling Wait twice, which
// os/exec documents as an error.
func (p *process) Wait() error { return <-p.exited }

// Stop signals the process to end gracefully — SIGTERM on unix, the closest
// equivalent on Windows (see platform_windows.go).
func (p *process) Stop() error { return terminate(p.cmd.Process) }
