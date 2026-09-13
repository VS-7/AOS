package main

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/domain/gateway"
)

// supervision is the slice of the gateway service the version check needs.
type supervision interface {
	Status(ctx context.Context, in gateway.StatusInput) (gateway.State, error)
	Restart(ctx context.Context, in gateway.RestartInput) (gateway.State, error)
}

// versionGuard replaces a daemon that is not the version this window was built
// with, when this window's supervisor is the one that started it.
//
// The daemon is detached — it outlives the window by design — and an install
// replaces the application bundle underneath it. The new window found it
// healthy and adopted it, so the fixes the update carried for the daemon never
// ran until somebody killed aosd by hand or rebooted; nothing even said the two
// disagreed. The comparison is exact rather than build.Compatible's same-minor
// rule: a patch release is exactly the case, and the point is the fixes, not
// protocol compatibility.
//
// The version compared is the daemon's own (/api/health), never the gateway
// record's, which is the version of whatever spawned it.
type versionGuard struct {
	window string

	mu    sync.Mutex
	tried map[string]bool
}

func newVersionGuard(window string) *versionGuard {
	return &versionGuard{window: window, tried: map[string]bool{}}
}

// releaseVersion is a comparable release version, or "" for a developer build
// ("dev") or anything else that does not look like one.
var releasePattern = regexp.MustCompile(`^\d+\.\d+`)

func releaseVersion(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if !releasePattern.MatchString(v) {
		return ""
	}
	return v
}

// check looks at the daemon's version and restarts it when it is not this
// window's and this window's supervisor owns it. It tries once per daemon
// version, so a restart that brings back the same version is not repeated in a
// loop. It reports whether it restarted.
func (g *versionGuard) check(ctx context.Context, sup supervision, daemonVersion string, log *slog.Logger) bool {
	window, daemon := releaseVersion(g.window), releaseVersion(daemonVersion)
	if window == "" || daemon == "" || window == daemon {
		return false
	}

	g.mu.Lock()
	seen := g.tried[daemon]
	g.tried[daemon] = true
	g.mu.Unlock()
	if seen {
		return false
	}

	state, err := sup.Status(ctx, gateway.StatusInput{})
	if err != nil || state.Status != gateway.Running {
		// Started some other way — a terminal, a service manager — so not
		// this window's to stop. Saying so is what is left.
		log.Warn("the daemon serving is a different version than this window; restart it to run this one",
			"daemon", daemonVersion, "window", g.window)
		return false
	}

	log.Info("replacing a daemon from another version of the application",
		"daemon", daemonVersion, "window", g.window, "pid", state.Meta.PID)
	if _, err := sup.Restart(ctx, gateway.RestartInput{}); err != nil {
		log.Error("the daemon from another version could not be replaced", "err", err)
		return false
	}
	return true
}
