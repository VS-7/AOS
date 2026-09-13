package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/OWNER/aos/internal/core/build"
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
//
// Every answer is also compared with the release this window was built from
// (see skewBetween). An update installed from a terminal restarts the daemon
// in about a second — often between two polls, so no miss is ever seen — and
// leaves this window running the previous release against it.
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
	reported := ""

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(daemonPollInterval):
		}

		probe, cancel := context.WithTimeout(ctx, 3*time.Second)
		health, err := client.Health(probe)
		cancel()

		if err == nil && health.Ready {
			if !healthy {
				log.Info("the daemon is answering again")
				// The workspace, the file root and the event relay all
				// followed the daemon that went away; a new one has none of
				// them until it is adopted again.
				reopen(ctx, client, root, adopt, log)
			}
			misses, healthy = 0, true
			skew := skewBetween(build.Current().Version, health.Version)
			if skew == nil {
				reported = ""
			} else if health.Version != reported {
				log.Warn("the daemon is a different release from this window", "window", skew.Window, "daemon", skew.Daemon, "compatible", skew.Compatible)
				reported = health.Version
			}
			emitDaemonState(emit, true, skew)
			continue
		}

		misses++
		if healthy {
			log.Warn("the daemon stopped answering", "err", err)
		}
		healthy = false
		emitDaemonState(emit, false, nil)

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

// emitDaemonState tells the interface, when there is a window to tell. The
// skew rides along only while the daemon is answering: an event without one
// is what takes the interface's banner down.
func emitDaemonState(emit func(event any), healthy bool, skew *versionSkew) {
	if emit == nil {
		return
	}
	event := map[string]any{"healthy": healthy}
	if healthy && skew != nil {
		event["skew"] = skew
	}
	emit(event)
}

// versionSkew is a daemon answering from a different release than the one
// this window was built from.
type versionSkew struct {
	Window string `json:"window"`
	Daemon string `json:"daemon"`
	// WindowOlder: the daemon was updated under this window, and the window
	// is the one to restart. Otherwise the daemon is older — a new window
	// adopted the daemon a previous install left running — and restarting
	// the daemon is what brings it level.
	WindowOlder bool `json:"windowOlder"`
	// Compatible is build.Compatible's answer: same minor release, so the
	// two can still talk until one of them is restarted. Different minors
	// cannot, and the interface says so more loudly.
	Compatible bool `json:"compatible"`
}

// skewBetween compares the window's release with the daemon's.
//
// Nil when they are one release — suffixes aside, as build.SemVer.Compare
// has it, so a locally packaged -dirty tree matches its tag — and when either
// carries no release to compare ("dev", or a daemon that did not say), which
// is build.Compatible's own stance on development builds.
func skewBetween(window, daemon string) *versionSkew {
	w, wok := build.ParseVersion(window)
	d, dok := build.ParseVersion(daemon)
	if !wok || !dok || w.Compare(d) == 0 {
		return nil
	}
	return &versionSkew{
		Window:      window,
		Daemon:      daemon,
		WindowOlder: w.Compare(d) < 0,
		Compatible:  build.Compatible(window, daemon) == nil,
	}
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
