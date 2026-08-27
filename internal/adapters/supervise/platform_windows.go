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

// stillActive is STILL_ACTIVE: what GetExitCodeProcess reports for a process
// that has not exited.
const stillActive = 259

// alive asks the kernel directly.
//
// os.FindProcess always succeeds here — it does not look anything up — and
// os.Process.Signal refuses every signal but Kill, so the Unix "send signal 0"
// probe answered false for every live process. Opening a handle and reading
// the exit code is the question that has a real answer on this platform.
//
// A pid the caller may not open is treated as alive rather than dead: it
// exists, and reporting it dead is what makes a supervisor start a second
// daemon beside the first.
func alive(pid int) bool {
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == stillActive
}
