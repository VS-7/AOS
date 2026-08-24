package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// RealtimeEventName is what the window listens for. One name for every kind
// of event: the payload is the daemon's own envelope, and the interface
// already knows how to read it.
const RealtimeEventName = "aos:realtime"

// forwardRealtime keeps the daemon's event channel open and hands what
// arrives to the window.
//
// The window cannot open that channel itself. Its page is served from the
// application's own `wails://` scheme, and a WebView will not let a page on a
// custom scheme open a ws:// connection to another origin — the socket is
// refused locally, so nothing ever reaches the daemon to even be logged. That
// is why the desktop had no live updates of any kind: not a failing
// connection, no connection at all.
//
// This process has no such restriction, and it is already the window's client
// for everything else. So it holds the socket and relays.
func forwardRealtime(ctx context.Context, address, workspaceID string, emit func(any), log *slog.Logger) {
	backoff := []time.Duration{250 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	attempt := 0
	// Whether this has ever worked. A channel that drops and reconnects is
	// ordinary; one that has never opened is a broken installation, and it
	// used to say nothing at all — which is how the desktop ran without live
	// updates for as long as it did.
	opened := false

	for ctx.Err() == nil {
		err := streamRealtime(ctx, address, workspaceID, func() {
			if !opened {
				opened = true
				log.Info("the event channel is open", "workspace", workspaceID)
			}
			attempt = 0
		}, emit)
		if err != nil && ctx.Err() == nil {
			if opened {
				log.Debug("the event channel dropped; reconnecting", "err", err)
			} else {
				log.Warn("the event channel could not be opened; the interface will not update on its own",
					"address", address, "workspace", workspaceID, "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff[min(attempt, len(backoff)-1)]):
		}
		attempt++
	}
}

func streamRealtime(ctx context.Context, address, workspaceID string, onOpen func(), emit func(any)) error {
	endpoint := strings.Replace(strings.TrimRight(address, "/"), "http", "ws", 1) +
		"/ws?workspace=" + url.QueryEscape(workspaceID)

	// No credential: the channel authorises by workspace, not by user
	// (internal/transport/realtime.Upgrade), and a token nothing reads would
	// be a second credential path to keep in step for no gain.
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()

	// No ceiling worth guessing at: a message snapshot carries a whole
	// answer, tool results included.
	conn.SetReadLimit(-1)
	onOpen()

	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var event any
		if err := json.Unmarshal(raw, &event); err != nil {
			// A frame this build does not understand is not a reason to drop
			// the channel.
			continue
		}
		emit(event)
	}
}
