//go:build production && !devtools

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// addDevTools adds nothing to a release build's View menu — see the
// development build's version.
func addDevTools(*application.Menu) {}
