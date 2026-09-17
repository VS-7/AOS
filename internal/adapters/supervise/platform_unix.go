//go:build !windows

package supervise

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
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

// alive asks with signal 0, which performs the permission and existence
// checks without delivering anything.
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// commandLineTimeout bounds the one process this asks the question through.
// It is asked once per supervision call rather than in a loop, and a `ps`
// that has not answered in a second is not going to say anything the caller
// can use.
const commandLineTimeout = time.Second

// commandLine asks the operating system what a process is running.
//
// Through `ps` rather than a system call because the two Unixes this ships
// on answer the question in entirely different ways — /proc/<pid>/cmdline on
// Linux, a KERN_PROCARGS2 sysctl on macOS — and `ps -o args=` is the one
// spelling both of them understand.
//
// A pid `ps` cannot find is not a failure to read: it is the answer
// "nothing", and what that means is the caller's liveness check to decide.
func commandLine(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandLineTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
