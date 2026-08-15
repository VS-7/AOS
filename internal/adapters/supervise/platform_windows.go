//go:build windows

package supervise

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// detach starts the daemon without a console, so it does not flash a window and
// does not die with the one that started it.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // DETACHED_PROCESS
	}
}

// terminate has no graceful equivalent here that a Go program can receive
// reliably, so this is the kill. The gateway's escalation still behaves
// correctly: it asks, waits, and kills — the first two are simply the same
// thing on this platform.
func terminate(p *os.Process) error {
	if err := p.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func isWindows() bool { return true }
