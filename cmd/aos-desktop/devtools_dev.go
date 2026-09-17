//go:build !production || devtools

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// addDevTools keeps the View menu's Open DevTools in the builds Wails' own
// default menu has it in: development builds, and production ones built with
// the devtools tag. The replacement View menu dropped it along with Reload.
func addDevTools(view *application.Menu) {
	view.AddRole(application.OpenDevTools)
}
