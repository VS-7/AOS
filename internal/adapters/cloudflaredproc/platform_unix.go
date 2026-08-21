//go:build !windows

package cloudflaredproc

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// detach puts cloudflared in its own process group, so a signal sent to the
// terminal that started it — Ctrl-C ending the command — does not reach it.
// Mirrors internal/adapters/supervise's own detach for the same reason.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// terminate sends SIGTERM: the signal cloudflared can catch to close its
// connection cleanly.
func terminate(p *os.Process) error {
	if p == nil {
		return nil
	}
	if err := p.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
