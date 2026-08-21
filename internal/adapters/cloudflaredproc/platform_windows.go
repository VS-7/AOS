//go:build windows

package cloudflaredproc

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// detach starts cloudflared without a console, so it does not flash a window
// and does not die with the command that started it.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // DETACHED_PROCESS
	}
}

// terminate has no graceful equivalent here a Go program can receive
// reliably, so this is the kill — mirrors internal/adapters/supervise's own
// account of the same platform gap.
func terminate(p *os.Process) error {
	if p == nil {
		return nil
	}
	if err := p.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
