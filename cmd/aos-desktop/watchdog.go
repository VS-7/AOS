package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/transport/daemonclient"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// DaemonEventName is the window event carrying the daemon's health.
//
// The interface listens for it to say it is waiting rather than reporting
// every action as broken — see lib/realtime.ts.
const DaemonEventName = "aos:daemon"

// daemonPollInterval is how often the window asks whether the daemon is still
// there.
//
// Slow enough to cost nothing on an idle machine, fast enough that somebody
// who quit the daemon in a terminal sees the window notice before they wonder
// whether it has.
const daemonPollInterval = 5 * time.Second

// daemonFailuresBeforeRestart is how many misses it takes before the window
// tries to bring the daemon back.
//
// One miss is a busy machine or a request that lost a race with a restart
// somebody else asked for. Three in a row, fifteen seconds apart, is a daemon
// that is gone.
const daemonFailuresBeforeRestart = 3

// watchDaemon keeps the daemon alive underneath the window, and tells the
// interface which of the two states it is in.
//
// Supervision used to be a one-shot boot step: `ensureDaemon` ran once, in a
// goroutine, and nothing looked again. A daemon that crashed, or that
// somebody stopped from a terminal, left the window rendering its screens
// while every action failed — "Load failed" as an untranslated toast — with
// no way back short of relaunching the application. The window owns the
// supervisor; it may as well use it.
//
// Restarting is deliberately not the first move: the window reports the loss
// immediately (so the interface can say so) and only tries to start a daemon
// after several consecutive misses, because a restart while one is already
// coming back is how two daemons end up fighting over one port.
func watchDaemon(
	ctx context.Context,
	supervisor *gateway.Service,
	client *daemonclient.Client,
	adopt func(workspaceRef),
	root string,
	emit func(event any),
	log *slog.Logger,
) {
	misses := 0
	healthy := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(daemonPollInterval):
		}

		probe, cancel := context.WithTimeout(ctx, 3*time.Second)
		ready, err := client.Ready(probe)
		cancel()

		if err == nil && ready {
			if !healthy {
				log.Info("the daemon is answering again")
				// The workspace, the file root and the event relay all
				// followed the daemon that went away; a new one has none of
				// them until it is adopted again.
				reopen(ctx, client, root, adopt, log)
			}
			misses, healthy = 0, true
			emitDaemonState(emit, true)
			continue
		}

		misses++
		if healthy {
			log.Warn("the daemon stopped answering", "err", err)
		}
		healthy = false
		emitDaemonState(emit, false)

		if misses < daemonFailuresBeforeRestart {
			continue
		}
		misses = 0

		start, cancelStart := context.WithTimeout(ctx, 30*time.Second)
		if _, err := supervisor.Start(start, gateway.StartInput{}); err != nil {
			log.Error("the daemon could not be started again", "err", err)
		}
		cancelStart()
	}
}

// emitDaemonState tells the interface, when there is a window to tell.
func emitDaemonState(emit func(event any), healthy bool) {
	if emit == nil {
		return
	}
	emit(map[string]any{"healthy": healthy})
}

// reopen re-adopts the workspace after a daemon came back.
//
// A restarted daemon is a different process with none of this window's
// per-session state: the client's workspace, the file root and the event
// relay were all bound to the one that died.
func reopen(ctx context.Context, client *daemonclient.Client, root string, adopt func(workspaceRef), log *slog.Logger) {
	open, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opened, err := openWorkspace(open, client, root, wailsvc.AuthLogin)
	if err != nil {
		log.Debug("no workspace to re-adopt after the daemon came back", "err", err)
		return
	}
	adopt(opened)
}
