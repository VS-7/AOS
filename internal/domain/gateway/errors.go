package gateway

import (
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errLockUnavailable(cause error) error {
	return apperr.New("GATEWAY_LOCK_UNAVAILABLE").
		Causer("gateway.Service").
		Msgf("another process is supervising the daemon right now").
		Status(apperr.StatusConflict).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label:   "wait for the other command to finish, then ask what the state is",
			Command: build.Name + " gateway status",
			Tool:    "gateway_status",
		})
}

func errStateUnreadable(cause error) error {
	return apperr.New("GATEWAY_STATE_UNREADABLE").
		Causer("gateway.Service.read").
		Msgf("the daemon's record could not be read").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errStateUnwritable(cause error) error {
	return apperr.New("GATEWAY_STATE_UNWRITABLE").
		Causer("gateway.Service").
		Msgf("the daemon's record could not be written").
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errSpawnFailed(path string, cause error) error {
	return apperr.New("GATEWAY_SPAWN_FAILED").
		Causer("gateway.Service.Start").
		Msgf("could not start %q", path).
		Issue("command", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

// errDaemonExited is the case the health check exists to distinguish from a
// slow boot: the process is gone, so waiting longer changes nothing.
func errDaemonExited(pid int) error {
	return apperr.New("GATEWAY_DAEMON_EXITED").
		Causer("gateway.Service.waitHealthy").
		Msgf("the daemon exited before it started serving").
		Issue("pid", pid).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			// `aos gateway logs` is not a command — this group is start, stop,
			// restart and status. Naming the file is what the reader can
			// actually act on (defect #5).
			Label:   "read the daemon log — the reason it exited is the last thing in it",
			Command: "tail -n 50 ~/" + build.StateDir + "/runtime/gateway/gateway.log",
		})
}

// errNeverBecameHealthy is the other half: the process is alive and not
// serving. The original waits for liveness only, and would call this a success.
func errNeverBecameHealthy(host string, port int, waited time.Duration) error {
	return apperr.New("GATEWAY_NOT_HEALTHY").
		Causer("gateway.Service.waitHealthy").
		Msgf("the daemon started but did not answer on %s:%d within %s", host, port, waited).
		Issue("host", host).
		Issue("port", port).
		Status(apperr.StatusGatewayTimeout).
		CTA(
			apperr.CallToAction{
				Label: "something else may be holding the port; check what is listening on it",
			},
			apperr.CallToAction{
				Label:   "read the daemon log",
				Command: "tail -n 50 ~/" + build.StateDir + "/runtime/gateway/gateway.log",
			},
		)
}

func errKillFailed(pid int, cause error) error {
	return apperr.New("GATEWAY_KILL_FAILED").
		Causer("gateway.Service.Stop").
		Msgf("the daemon ignored the shutdown request and could not be killed").
		Issue("pid", pid).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{
			Label: "stop the process by hand — it is holding the port",
		})
}

// errSelfRestart is what the daemon answers when asked to restart itself.
//
// The alternative was not "it works": Stop signals the pid in the record,
// which is this process, so the daemon terminated itself mid-request and the
// caller received an unclassified 500 as the connection dropped — the shape
// AGENTS.md calls a defect. Whoever launched the daemon has a working copy of
// this same service; the call to action points there.
func errSelfRestart() error {
	return apperr.New("GATEWAY_SELF_RESTART").
		Causer("gateway.Service.Restart").
		Msgf("the daemon cannot restart itself; ask whatever supervises it").
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "restart it from the application, or from a terminal",
			Command: build.Name + " gateway restart",
		})
}
