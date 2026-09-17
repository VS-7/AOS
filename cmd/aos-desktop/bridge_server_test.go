package main

import (
	"testing"
	"time"
)

// A `-tags server` build serves the window over HTTP to a browser, and Wails
// gives that server a thirty-second write deadline by default. A bridge call
// that took longer — a download, an install, a daemon that stalled — had its
// answer refused by the deadline, the connection closed with nothing written,
// and the browser, which resends a request whose kept-alive connection closed
// before any answer, sent the same POST /wails/runtime again: one
// todos_create, measured with the daemon stalled for 34 seconds, made two
// rows. How long a call may take is daemonclient's to decide, while the
// daemon is alive, not the asset server's.
func TestTheWindowsServerDoesNotCutASlowAnswer(t *testing.T) {
	if got := serverOptions().WriteTimeout; got < time.Hour {
		t.Errorf("WriteTimeout = %s; a bridge call that outlasts it is sent twice by the browser", got)
	}
}
