package main

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ReloadEventName is the window event the menu's Reload sends. The interface
// answers it with its own reload (lib/native.ts), which keeps the parameters
// the window was opened with.
const ReloadEventName = "aos:reload"

// applicationMenu is the menu bar this application installs in place of the
// one Wails would.
//
// Wails' default View menu reloads the webview itself (Reload, Cmd+R; Force
// Reload, Cmd+Shift+R), at the URL as it is by then — and the router strips
// `?daemon=` from it on the first navigation. The bundle came back as a browser
// tab, with no daemon address and no event channel, and the window stayed
// broken until the application was restarted. Everything else is the default,
// Edit above all: without it Cmd+C and Cmd+V do nothing in a macOS webview.
func applicationMenu(reload func()) *application.Menu {
	menu := application.NewMenu()
	menu.AddRole(application.AppMenu)
	menu.AddRole(application.FileMenu)
	menu.AddRole(application.EditMenu)
	menu.AddSubmenu("View").Append(viewMenu(reload))
	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)
	return menu
}

// hasMenuBar is whether applicationMenu is installed on goos: macOS only,
// which is also where Wails installs its default (App.Run sets the
// application menu on darwin alone).
//
// Linux is the reason it matters. A GTK window with no menu of its own takes
// the application menu as its menubar (webview_window_linux.go), and the window
// there is frameless and draws its own chrome, so a File/Edit/View strip
// appeared above the tab bar.
func hasMenuBar(goos string) bool {
	return goos == "darwin"
}

// viewMenu is Wails' View menu with the reload handed to the page.
func viewMenu(reload func()) *application.Menu {
	view := application.NewMenu()
	view.Add("Reload").
		SetAccelerator("CmdOrCtrl+r").
		OnClick(func(*application.Context) { reload() })
	addDevTools(view)
	view.AddSeparator()
	view.AddRole(application.ResetZoom)
	view.AddRole(application.ZoomIn)
	view.AddRole(application.ZoomOut)
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)
	return view
}

// spaFallback serves the interface for a route the router owns.
//
// The embedded asset server answers index.html for "/" and a bare 404 for
// anything it has no file for — so a reload on /goals, or any navigation the
// webview makes to a deep route, drew a blank page. A navigation (it asks for
// HTML) to a path the bundle has no file for is the router's, and gets the
// interface; a missing script or stylesheet still 404s as it should, and the
// daemon paths this window forwards are never touched.
func spaFallback(dist fs.FS, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isRouterNavigation(dist, r) {
			r = r.Clone(r.Context())
			r.URL.Path, r.URL.RawPath = "/", ""
		}
		next.ServeHTTP(w, r)
	})
}

func isRouterNavigation(dist fs.FS, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	p := r.URL.Path
	if p == "" || p == "/" || !strings.Contains(r.Header.Get("Accept"), "text/html") {
		return false
	}
	for _, prefix := range []string{"/api/", "/wails/", "/v/"} {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	_, err := fs.Stat(dist, strings.TrimPrefix(path.Clean(p), "/"))
	return err != nil
}
