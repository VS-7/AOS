//go:build !windows

package supervise

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// detach puts the daemon in its own process group, so that a signal sent to
// the terminal — the Ctrl-C that ends the command that started it — does not
// reach it.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// terminate sends SIGTERM: the signal a daemon can catch in order to finish
// what it is writing.
func terminate(p *os.Process) error {
	if err := p.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func isWindows() bool { return false }
