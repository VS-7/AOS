package main

import (
	"context"
	"net/url"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// wailsPlatform is the operating system, as the system service reaches it.
//
// It is the only file in this binary that calls Wails directly. Everything the
// window can ask for is decided in internal/transport/wailsvc, where it can be
// tested without opening a Finder window.
type wailsPlatform struct{ window *application.WebviewWindow }

func (p *wailsPlatform) OpenPath(_ context.Context, path string) error {
	return application.Get().Browser.OpenFile(path)
}

func (p *wailsPlatform) RevealInFolder(_ context.Context, path string) error {
	// Wails has no reveal-in-folder of its own. Opening the containing
	// directory is the closest thing every platform agrees on, and it is what
	// the person meant: show me where this is.
	return application.Get().Browser.OpenFile(directoryOf(path))
}

func (p *wailsPlatform) OpenExternal(_ context.Context, rawURL string) error {
	// The scheme was already checked by the service. This is the call, not the
	// decision — putting the check here too would be two places to keep right.
	return application.Get().Browser.OpenURL(rawURL)
}

func (p *wailsPlatform) PickFiles(_ context.Context, opts wailsvc.PickOptions) ([]string, error) {
	dialog := application.Get().Dialog.OpenFile()
	if opts.Title != "" {
		dialog.SetTitle(opts.Title)
	}
	if opts.Directory != "" {
		dialog.SetDirectory(opts.Directory)
	}
	dialog.CanChooseDirectories(opts.Directories)
	dialog.CanChooseFiles(!opts.Directories)

	if opts.Multiple {
		picked, err := dialog.PromptForMultipleSelection()
		if err != nil {
			return nil, err
		}
		return picked, nil
	}
	picked, err := dialog.PromptForSingleSelection()
	if err != nil {
		return nil, err
	}
	if picked == "" {
		// Cancelled. An empty answer, not a failure.
		return nil, nil
	}
	return []string{picked}, nil
}

// PickSavePath opens the system save panel and reports the path chosen.
//
// The panel is what authorises the write: nothing else in this binary decides
// where an export lands. A cancelled panel comes back as "" with no error —
// changing your mind is not a failure.
func (p *wailsPlatform) PickSavePath(_ context.Context, opts wailsvc.SaveOptions) (string, error) {
	dialog := application.Get().Dialog.SaveFile()
	if opts.Filename != "" {
		dialog.SetFilename(opts.Filename)
	}
	if opts.Title != "" {
		dialog.SetMessage(opts.Title)
	}
	dialog.CanCreateDirectories(true)
	if p.window != nil {
		dialog.AttachToWindow(p.window)
	}
	return dialog.PromptForSingleSelection()
}

func (p *wailsPlatform) SetAppearance(_ context.Context, appearance, windows string) error {
	if p.window == nil {
		return nil
	}
	// A translucent window needs a transparent background behind the page for
	// the operating system's blur to show through; a solid one needs the page
	// to paint its own.
	//
	// Never transparent on Linux, whatever the theme asks for — see
	// translucentHere for what a transparent window does there.
	if windows == "blur" && translucentHere() {
		p.window.SetBackgroundColour(application.NewRGBA(0, 0, 0, 0))
	} else {
		p.window.SetBackgroundColour(opaqueFor(appearance))
	}
	return nil
}

// backgroundTypeHere is the window's compositing mode for this platform.
func backgroundTypeHere() application.BackgroundType {
	if translucentHere() {
		return application.BackgroundTypeTranslucent
	}
	return application.BackgroundTypeSolid
}

// backgroundHere is the colour the window starts with.
//
// Transparent where the platform draws its own backdrop; a real, fully opaque
// colour where it does not. Dark is the right guess for the first frame: the
// interface's own default appearance is dark, and the alternative — a flash of
// white before the page paints — is the more noticeable of the two mistakes.
// The page corrects it through SetAppearance as soon as the theme is known.
func backgroundHere() application.RGBA {
	if translucentHere() {
		return application.NewRGBA(0, 0, 0, 0)
	}
	return opaqueFor("dark")
}

// opaqueFor is the colour behind the page while it has not painted yet. It is
// near-black for a dark appearance and near-white for a light one, so the
// moment before the first paint is not a flash of the opposite.
func opaqueFor(appearance string) application.RGBA {
	if appearance == "light" {
		return application.NewRGBA(250, 250, 250, 255)
	}
	return application.NewRGBA(16, 16, 18, 255)
}

func directoryOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			if i == 0 {
				return path[:1]
			}
			return path[:i]
		}
	}
	return "."
}

// framelessHere decides whether this window draws its own frame.
//
// It is not the same answer on every platform, and the previous unconditional
// `true` is why the application had no window controls at all on macOS.
//
// Wails hides the traffic lights outright for a frameless macOS window with
// default corner options (effectiveMacWindowButtonStates in
// webview_window_options.go), and skips every Mac.TitleBar option while it is
// set (webview_window_darwin.go). So `Frameless: true` paired with
// MacTitleBarHiddenInset — the configuration this application shipped — asked
// for inset traffic lights and got no traffic lights, no title bar, and no way
// to close, minimise or zoom the window except the menu bar and the keyboard.
// On macOS the right answer is a hidden-inset title bar, which is exactly the
// native chrome over full-size content, and is what the original did too.
//
// Windows and Linux have no equivalent: their frame is all or nothing, so the
// window is frameless there and the interface draws the three controls itself
// (see the frontend's window-controls component).
func framelessHere() bool { return runtime.GOOS != "darwin" }

// translucentHere decides whether this window may be see-through.
//
// Not on Linux, and this is the fix for the ghosting people reported there:
// screens leaving a faint residue of whatever was on them before.
//
// A translucent window on Linux makes Wails call setTransparent()
// (webview_window_linux.go), which composites the whole window with an alpha
// channel. WebKitGTK's accelerated compositor latches its clear colour at the
// first composite and does not refresh it — Wails' own comment in that file
// says so — and with an alpha channel there is no opaque clear at all, so a
// region the compositor re-tiles keeps whatever was drawn there. The result is
// the previous screen showing through the new one at low opacity, which is
// exactly what was seen.
//
// macOS is the platform this was for: NSVisualEffectView draws a real backdrop
// behind the page. Windows composites correctly through WebView2. Linux has
// neither, and pays for the option with a rendering defect.
func translucentHere() bool { return runtime.GOOS != "linux" }

// windowOptions is the window this application opens.
//
// It is a function rather than a literal at the call site so the choices in it
// can be checked: the previous configuration asked macOS for inset traffic
// lights and got no window controls at all, and nothing failed — the window
// simply opened with no way to close, minimise or zoom it with the mouse. A
// test can assert the combination; only a person can see the window.
func windowOptions(address string) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Title: build.DisplayName,

		// The daemon's address, handed to the interface in the one place it
		// can read synchronously and cannot fail to reach: its own URL.
		//
		// Everything else the window asks for goes through the Wails bridge,
		// which needs no address. The realtime channel is the exception — it
		// is a WebSocket the webview opens itself, and in this window
		// `window.location` is `wails://localhost`, the application's own
		// asset scheme. A socket URL derived from it names something that
		// serves no `/ws` and is not even a valid WebSocket scheme, which is
		// why the desktop had no live updates at all: not a failing
		// connection, no connection.
		// `platform` rides along for the same reason. Wails answers this
		// through System.IsMac()/IsWindows()/IsLinux(), which read
		// `window._wails.environment` — injected from the host's
		// WebViewDidFinishNavigation hook, i.e. *after* the interface bundle
		// has run and decided how to lay itself out. The interface used to
		// match /Macintosh/ against the user agent to bridge that gap, which
		// is a guess about a fact this process already knows for certain.
		URL: "/?daemon=" + url.QueryEscape(address) +
			"&platform=" + url.QueryEscape(runtime.GOOS),

		Width:     1440,
		Height:    900,
		MinWidth:  960,
		MinHeight: 600,

		// The window's own chrome, per platform — see framelessHere for why
		// this is not simply `true` everywhere.
		Frameless: framelessHere(),

		// Files dragged onto the window reach the interface.
		//
		// This is not optional the way it looks. The macOS window registers
		// itself as the dragging destination unconditionally
		// (webview_window_darwin.m's setDelegate), so a file drag never
		// reaches the WebView at all and the interface's HTML5 `drop`
		// handlers — which is what the port brought over from Electron, where
		// they worked — see nothing. With this off, Wails' own
		// HandlePlatformFileDrop returns early too, and the drop is simply
		// swallowed. With it on, the paths arrive as a window event, which
		// main.go forwards to the interface.
		EnableFileDrop: true,

		// Opaque on Linux, translucent elsewhere — see translucentHere.
		//
		// The colour matters even in the solid case, and it has to be right
		// here rather than set later: Wails pins it as WebKitGTK's
		// accelerated-compositing clear colour before the first paint, and a
		// colour left at zero would be treated as "not explicitly opaque" and
		// leave the WebKitGTK default (white) latched behind a dark page.
		BackgroundColour: backgroundHere(),
		BackgroundType:   backgroundTypeHere(),
		Mac: application.MacWindow{
			// Hidden-inset: no title bar, full-size content, and the traffic
			// lights still drawn by macOS in the top left, slightly inset —
			// the same shape the original used. The sidebar leaves room for
			// them.
			//
			// This is applied only because Frameless is false here: Wails
			// skips every title-bar option on a frameless window
			// (webview_window_darwin.go), so the previous pairing of
			// Frameless:true with this preset silently discarded the preset.
			TitleBar: application.MacTitleBarHiddenInset,
			Backdrop: application.MacBackdropTranslucent,
			// The strip along the top that drags the window even where the
			// interface has drawn something. The interface marks its own drag
			// regions too (--wails-draggable), and this covers the rest.
			InvisibleTitleBarHeight: 40,
		},
		Windows: application.WindowsWindow{
			// A frameless window on Windows still needs the system to know
			// which regions are caption, or the custom controls the interface
			// draws get none of the snap, aero-shake and double-click-to-
			// maximise behaviour people expect from a title bar.
			NonClientRegionSupport:     true,
			WebView2CompositionHosting: true,
		},
	}
}
