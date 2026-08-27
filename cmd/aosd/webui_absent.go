//go:build !webui

package main

import "io/fs"

// webInterface reports that this build carries no interface.
//
// The default. This is the daemon aos-desktop supervises, and that window
// carries its own copy of the bundle and loads it off its own scheme — a
// second copy here would be 14 MB of the same files in the same tarball. See
// webui_embed.go for the build that does carry it.
func webInterface() fs.FS { return nil }
