//go:build webui

package main

import (
	"embed"
	"io/fs"
)

// The web interface, compiled into this daemon.
//
// Behind a build tag because the bundle belongs in exactly one binary per
// deployment. On a server this daemon *is* the interface, and one file is most
// of what makes it deployable to a machine nobody wants to keep a bundle in
// sync on. Beside aos-desktop it would be a second copy of the same 14 MB, in
// a tarball that already carries the first one inside the window — which is
// why `task build:cli` produces a daemon without it and `task build:server`
// produces one with it.
//
// The files here are gzipped: 53 MB of bundle for 14 MB of binary, and a page
// fetched over a network rather than off the local disk wants them compressed
// anyway. httpapi serves them as they are — see webui.go there.
//
//go:embed all:dist
var bundle embed.FS

// webInterface is the bundle, rooted where the files actually are.
//
// A dist directory holding only .gitkeep is a build that never had the
// frontend copied in. Answering nil for that is what makes the daemon say so
// on startup instead of serving a blank page.
func webInterface() fs.FS {
	sub, err := fs.Sub(bundle, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html.gz"); err != nil {
		if _, err := fs.Stat(sub, "index.html"); err != nil {
			return nil
		}
	}
	return sub
}
