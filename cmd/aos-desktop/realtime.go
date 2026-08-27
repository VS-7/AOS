package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// RealtimeEventName is what the window listens for. One name for every kind
// of event: the payload is the daemon's own envelope, and the interface
// already knows how to read it.
const RealtimeEventName = "aos:realtime"

// FilesDroppedEventName carries the paths of files dragged onto the window.
//
// A separate name from the realtime channel because it is a separate thing:
// realtime events are the daemon's, relayed; this one originates in the window
// itself and never leaves the machine.
const FilesDroppedEventName = "aos:files-dropped"

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
func forwardRealtime(ctx context.Context, address, workspaceID string, token func() string, emit func(any), log *slog.Logger) {
	backoff := []time.Duration{250 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}
	attempt := 0
	// Whether this has ever worked. A channel that drops and reconnects is
	// ordinary; one that has never opened is a broken installation, and it
	// used to say nothing at all — which is how the desktop ran without live
	// updates for as long as it did.
	opened := false

	for ctx.Err() == nil {
		err := streamRealtime(ctx, address, workspaceID, token, func() {
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

func streamRealtime(ctx context.Context, address, workspaceID string, token func() string, onOpen func(), emit func(any)) error {
	endpoint := strings.Replace(strings.TrimRight(address, "/"), "http", "ws", 1) +
		"/ws?workspace=" + url.QueryEscape(workspaceID)

	// The credential the rest of this window's calls already carry. The
	// channel used to be dialled without one, on the reasoning that it
	// authorises by workspace rather than by user — but a workspace with no
	// explicit members admits everybody, so that made the whole event stream
	// readable by anything that could reach the port. The daemon now requires
	// a credential here like it does everywhere else, and this is where the
	// window presents it. It never reaches the page.
	var opts *websocket.DialOptions
	if token != nil {
		if t := token(); t != "" {
			opts = &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + t}}}
		}
	}
	conn, res, err := websocket.Dial(ctx, endpoint, opts)
	// The handshake response is only interesting when the dial failed — a
	// 401 from the daemon arrives here rather than as a socket. Closed either
	// way: a rejected upgrade leaves a body nobody else will.
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
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
