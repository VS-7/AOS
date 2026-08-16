package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

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

func (p *wailsPlatform) SetAppearance(_ context.Context, appearance, windows string) error {
	if p.window == nil {
		return nil
	}
	// A translucent window needs a transparent background behind the page for
	// the operating system's blur to show through; a solid one needs the page
	// to paint its own.
	if windows == "blur" {
		p.window.SetBackgroundColour(application.NewRGBA(0, 0, 0, 0))
	} else {
		p.window.SetBackgroundColour(opaqueFor(appearance))
	}
	return nil
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
