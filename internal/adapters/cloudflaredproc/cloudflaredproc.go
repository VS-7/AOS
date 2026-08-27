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
		return nil, fmt.Errorf("%w: %w", tunnel.ErrBinaryMissing, err)
	}

	// Not exec.CommandContext(ctx, ...): ctx bounds only the readiness wait
	// below, and cloudflared must keep running long after this call returns —
	// tying it to ctx would kill the tunnel the moment the request that
	// started it completes.
	cmd := exec.Command(binary, "tunnel", "--hostname", hostname, "run", "--token", token) //nolint:noctx // process outlives ctx by design, see above
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

	// The reader does not stop at the marker. It used to, and then nothing
	// read that pipe again for as long as the tunnel lived: cloudflared logs
	// steadily, the pipe's buffer is finite, and once it filled the process
	// blocked on its next write to stderr — the tunnel stopped forwarding and
	// nothing said why. Reading for as long as there is anything to read
	// costs one goroutine and is the whole fix.
	go func() {
		scanner := bufio.NewScanner(stderr)
		// cloudflared's own lines are short; the ceiling is for a stack trace
		// or a wrapped error, which would otherwise stop the scan and put us
		// right back in the blocked-writer case.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		signalled := false
		for scanner.Scan() {
			if !signalled && strings.Contains(scanner.Text(), readyMarker) {
				signalled = true
				close(ready)
			}
		}
	}()
	// Wait runs independently of that drain, and deliberately.
	//
	// os/exec asks callers to finish reading a StderrPipe before calling
	// Wait, and gating Wait on the read reaching EOF is the obvious way to
	// honour it — but it does not terminate here: cloudflared's own children
	// inherit the descriptor, so a grandchild that outlives its parent holds
	// the pipe open and EOF never comes. Stop would then signal a process
	// that had already gone while Wait blocked forever. The documented risk
	// of the other order is that Wait closes the pipe under an in-flight
	// read; the scan simply ends, which is exactly what it should do when the
	// process it was reading has exited.
	go func() { exited <- cmd.Wait() }()

	select {
	case <-ready:
		return &process{cmd: cmd, exited: exited}, nil
	case err := <-exited:
		if err == nil {
			err = fmt.Errorf("cloudflared exited before reporting a connection")
		}
		return nil, fmt.Errorf("%w: %w", tunnel.ErrReadinessTimeout, err)
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
