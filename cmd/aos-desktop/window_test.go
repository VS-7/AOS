package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TestTheWindowHasControlsToCloseItWith is the defect this file exists for.
//
// The application shipped with `Frameless: true` on every platform, paired
// with `Mac.TitleBar: MacTitleBarHiddenInset`. Read together those say "no
// frame, but keep the inset traffic lights", and Wails resolves them the other
// way round on both counts:
//
//   - effectiveMacWindowButtonStates (webview_window_options.go) hides the
//     minimise, close and zoom buttons outright for a frameless macOS window
//     with default corner options, which is what this window had.
//   - the whole title-bar block in webview_window_darwin.go is guarded by
//     `if !w.parent.options.Frameless`, so the preset was never applied at all.
//
// The result was a window with no title bar, no traffic lights, and — since
// the interface drew no controls of its own and marked no drag regions Wails
// understands — no way to move, resize, minimise or close it with the mouse.
//
// This asserts the combination, because nothing else does: neither of those
// Wails behaviours is an error, and the window opens either way.
func TestTheWindowHasControlsToCloseItWith(t *testing.T) {
	opts := windowOptions("http://127.0.0.1:5326")

	if runtime.GOOS == "darwin" {
		if opts.Frameless {
			t.Error("the macOS window is frameless, which is what makes Wails hide the traffic lights")
		}
		if opts.Mac.TitleBar != application.MacTitleBarHiddenInset {
			t.Errorf("title bar = %+v, want the hidden-inset preset", opts.Mac.TitleBar)
		}
		// The preset's own contract: transparent, no title text, content
		// filling the window, and the buttons still drawn.
		if !opts.Mac.TitleBar.AppearsTransparent || !opts.Mac.TitleBar.FullSizeContent {
			t.Errorf("the preset no longer means what this window relies on: %+v", opts.Mac.TitleBar)
		}
		if opts.Mac.TitleBar.Hide {
			t.Error("the title bar is hidden outright, which takes the traffic lights with it")
		}
		return
	}

	// Windows and Linux have no equivalent of a hidden title bar that keeps
	// its buttons: the frame is all or nothing. The window is frameless there
	// and the interface draws the three controls itself — see the frontend's
	// components/ui/window-controls.
	if !opts.Frameless {
		t.Error("the window is not frameless, so the interface would draw a second set of controls")
	}
}

// TestTheWindowCanBeDraggedByItsTitleArea. On macOS the invisible title bar is
// what makes the top strip drag the window even where the interface has drawn
// something over it; the interface marks its own regions with
// --wails-draggable for everywhere else.
func TestTheWindowCanBeDraggedByItsTitleArea(t *testing.T) {
	opts := windowOptions("http://127.0.0.1:5326")
	if runtime.GOOS != "darwin" {
		t.Skip("the invisible title bar is a macOS option")
	}
	if opts.Mac.InvisibleTitleBarHeight == 0 {
		t.Fatal("no invisible title bar: the window cannot be dragged by its top strip")
	}
}

// TestTheWindowTellsTheInterfaceWhereTheDaemonIs. The realtime channel is the
// one connection the webview opens itself, and it cannot derive the daemon's
// address from a page served off the application's own asset scheme.
func TestTheWindowTellsTheInterfaceWhereTheDaemonIs(t *testing.T) {
	opts := windowOptions("http://127.0.0.1:5326")
	if !strings.Contains(opts.URL, "daemon=") {
		t.Fatalf("URL = %q, want the daemon address in it", opts.URL)
	}
	if !strings.Contains(opts.URL, "127.0.0.1%3A5326") {
		t.Fatalf("URL = %q, want the address escaped into the query", opts.URL)
	}
}

// TestAFilesystemRootIsNotHandedToTheDaemon guards the other half of the same
// launch: an application bundle opened from Finder starts in "/", and passing
// that on as the workspace made every collection-backed command in the
// application answer with a refusal for the whole session.
func TestAFilesystemRootIsNotHandedToTheDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`"/" is not a filesystem root on Windows`)
	}
	t.Chdir("/")

	if got := daemonEnv(""); got != nil {
		t.Fatalf("env = %v, want the parent's own when no workspace is named", got)
	}
}

// TestTheDaemonIsToldWhichDirectoryToServe: when this window does name a
// directory, the daemon is told, rather than left to infer it from a working
// directory it inherited.
func TestTheDaemonIsToldWhichDirectoryToServe(t *testing.T) {
	dir := t.TempDir()
	env := daemonEnv(dir)

	var found string
	for _, kv := range env {
		if strings.HasPrefix(kv, "AOS_WORKSPACE_PATH=") {
			found = strings.TrimPrefix(kv, "AOS_WORKSPACE_PATH=")
		}
	}
	if found != dir {
		t.Fatalf("AOS_WORKSPACE_PATH = %q, want %q", found, dir)
	}

	// Exactly one: a duplicated variable is resolved differently by different
	// libraries, and the process's own value is the one that would be stale.
	count := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, "AOS_WORKSPACE_PATH=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("AOS_WORKSPACE_PATH appears %d times", count)
	}
}

// TestTheWindowTellsTheInterfaceWhichPlatformItIsOn.
//
// The interface decides its own layout from this on its very first render: on
// macOS it insets the tab strip to clear the traffic lights and draws no
// window controls of its own; elsewhere it draws all three, because the window
// is frameless there and they are the only way to close it with a mouse.
//
// Wails answers the same question through System.IsMac(), which reads
// `window._wails.environment` — injected from the host's
// WebViewDidFinishNavigation hook, i.e. after the interface bundle has run and
// already decided. The interface used to bridge that gap by matching
// /Macintosh/ against the user agent; this is the same fact, stated by the
// process that actually knows it.
func TestTheWindowTellsTheInterfaceWhichPlatformItIsOn(t *testing.T) {
	opts := windowOptions("http://127.0.0.1:5326")

	if !strings.Contains(opts.URL, "platform="+runtime.GOOS) {
		t.Fatalf("URL = %q, want platform=%s in it", opts.URL, runtime.GOOS)
	}
	// Both parameters, not one replacing the other: the daemon address is what
	// gives the interface an API origin at all.
	if !strings.Contains(opts.URL, "daemon=") {
		t.Fatalf("URL = %q, want the daemon address kept alongside the platform", opts.URL)
	}
}

// TestFilesDraggedOntoTheWindowReachTheInterface.
//
// Every platform routes a file drag to the native window before the WebView
// sees it — on macOS the window registers itself as the dragging destination
// outright — so the HTML5 `drop` handlers the interface was ported with never
// fire. Without this option Wails' own HandlePlatformFileDrop returns early
// too, and the drop is accepted by the window and then discarded: dropping a
// file on the composer did nothing, and said nothing.
func TestFilesDraggedOntoTheWindowReachTheInterface(t *testing.T) {
	if !windowOptions("http://127.0.0.1:5326").EnableFileDrop {
		t.Fatal("file drops are disabled, so a dropped file is swallowed by the window")
	}
}

// TestTheLinuxWindowIsOpaque is the ghosting fix.
//
// A translucent window makes Wails call setTransparent() on Linux
// (webview_window_linux.go), which composites the whole window with an alpha
// channel. WebKitGTK's accelerated compositor latches its clear colour at the
// first composite and never refreshes it — Wails' own comment in that file
// says so — and with an alpha channel there is no opaque clear at all, so a
// re-tiled region keeps whatever was drawn there. What people saw was the
// previous screen showing through the new one at low opacity.
//
// Solid alone is not enough: Wails only pins the clear colour when the colour
// is *explicitly* opaque (`BackgroundColour.Alpha == 255`), leaving the
// WebKitGTK default — white — behind a dark page otherwise. Both halves are
// asserted here because either one alone leaves a visible defect.
func TestTheLinuxWindowIsOpaque(t *testing.T) {
	opts := windowOptions("http://127.0.0.1:5326")

	if runtime.GOOS == "linux" {
		if opts.BackgroundType != application.BackgroundTypeSolid {
			t.Error("the Linux window is translucent — WebKitGTK will not clear what was on it before")
		}
		if opts.BackgroundColour.Alpha != 255 {
			t.Errorf("background alpha = %d, want 255 so Wails pins the compositor's clear colour",
				opts.BackgroundColour.Alpha)
		}
		return
	}

	// Everywhere else the translucency is the point: macOS draws a real
	// NSVisualEffectView behind the page, and WebView2 composites correctly.
	if opts.BackgroundType != application.BackgroundTypeTranslucent {
		t.Errorf("background type = %v, want translucent on %s", opts.BackgroundType, runtime.GOOS)
	}
}

// TestTranslucencyIsDecidedPerPlatform pins the predicate itself, so the
// decision survives on the two platforms this test binary never runs on.
func TestTranslucencyIsDecidedPerPlatform(t *testing.T) {
	if got, want := translucentHere(), runtime.GOOS != "linux"; got != want {
		t.Errorf("translucentHere() = %v on %s, want %v", got, runtime.GOOS, want)
	}

	// The solid answer must be a colour, not a zero value: Wails treats a
	// non-opaque colour on a solid window as "no explicit background" and
	// leaves WebKitGTK's white default latched.
	if !translucentHere() && backgroundHere().Alpha != 255 {
		t.Error("a solid window was given a background Wails will ignore")
	}
	if translucentHere() && backgroundHere().Alpha != 0 {
		t.Error("a translucent window was given an opaque background, which defeats the backdrop")
	}
}
