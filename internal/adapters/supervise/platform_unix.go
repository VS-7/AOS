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

	"github.com/OWNER/aos/internal/domain/gateway"
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

// describeTimeout bounds the one process this asks the question through. It
// is asked once per supervision call, not in a loop, and a `ps` that has not
// answered in a second is not going to say anything the caller can use.
const describeTimeout = time.Second

// describe asks the operating system what a process is running and how long
// it has been running.
//
// Through `ps` rather than a system call because the two Unixes this ships on
// answer those questions in entirely different ways — /proc/<pid> on Linux, a
// pair of sysctls on macOS — and `ps -o etime=,args=` is the one spelling
// both of them understand.
//
// A pid `ps` cannot find is not a failure to read: it is the answer
// "nothing", and what that means is the caller's liveness check to decide.
func describe(pid int) (gateway.ProcessInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), describeTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ps", "-o", "etime=,args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return gateway.ProcessInfo{}, nil
		}
		return gateway.ProcessInfo{}, err
	}
	etime, line, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	age, known := parseElapsed(etime)
	return gateway.ProcessInfo{CommandLine: strings.TrimSpace(line), Elapsed: age, ElapsedKnown: known}, nil
}

// parseElapsed reads ps's elapsed time — [[dd-]hh:]mm:ss — and reports whether
// it understood the field at all.
//
// The second answer matters: 00:00 is what ps prints for a process in its
// first second, and a zero duration on its own is indistinguishable from a
// field this could not read. The caller treats "could not read" as no
// evidence, so conflating them let a just-reused pid pass as the daemon.
func parseElapsed(etime string) (time.Duration, bool) {
	days := 0
	if before, after, found := strings.Cut(etime, "-"); found {
		n, err := strconv.Atoi(before)
		if err != nil {
			return 0, false
		}
		days, etime = n, after
	}
	parts := strings.Split(etime, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	total := time.Duration(days) * 24 * time.Hour
	for _, unit := range []time.Duration{time.Hour, time.Minute, time.Second}[3-len(parts):] {
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || n < 0 {
			return 0, false
		}
		total, parts = total+time.Duration(n)*unit, parts[1:]
	}
	return total, true
}
