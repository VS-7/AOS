// Package wailsvc is the desktop's surface: the Go methods the React frontend
// calls through the Wails binding.
//
// It replaces the original's nine Electron IPC channels. What it does not
// replace is any domain logic: the domain service here delegates to the very
// same command registry the CLI, MCP and HTTP surfaces use, so the desktop
// cannot grow a second set of validation rules that drifts from the first.
//
// The platform calls live behind ports rather than being made directly. A test
// that has to open a Finder window to check that a path was refused is a test
// nobody runs.
package wailsvc

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// Platform is what the operating system is asked to do.
type Platform interface {
	OpenPath(ctx context.Context, path string) error
	RevealInFolder(ctx context.Context, path string) error
	OpenExternal(ctx context.Context, rawURL string) error

	// PickFiles opens the system file dialog. A cancelled dialog returns no
	// paths and no error: the person changed their mind, which is not a
	// failure and must not be reported as one.
	PickFiles(ctx context.Context, opts PickOptions) ([]string, error)

	// SetAppearance sets the window's native material, which is the desktop
	// half of a theme change.
	SetAppearance(ctx context.Context, appearance, windows string) error
}

// PickOptions describes the file dialog.
type PickOptions struct {
	Title       string   `json:"title,omitempty"`
	Directory   string   `json:"directory,omitempty"`
	Multiple    bool     `json:"multiple,omitempty"`
	Directories bool     `json:"directories,omitempty"`
	Extensions  []string `json:"extensions,omitempty"`
}

// Health reports whether the daemon behind the window is answering.
type Health interface {
	Ready(ctx context.Context) (bool, error)
}

// Addressable knows where the daemon is. Health is usually the same object;
// this is a second, narrower port so a caller that only wants the address
// does not have to supply a health check.
type Addressable interface {
	BaseURL() string
}

// SystemService is the equivalent of the original's IPC surface.
//
// Every exported method becomes a TypeScript function in the generated
// bindings, which is why the set is small and each one does one thing.
type SystemService struct {
	platform Platform
	health   Health
	address  Addressable

	// root is the workspace the window is looking at. Paths handed to the
	// operating system are resolved inside it, because a renderer that could
	// ask the shell to open any path is a renderer with a filesystem browser
	// nobody designed.
	root string
}

// NewSystem builds the system service.
func NewSystem(platform Platform, health Health, root string) *SystemService {
	svc := &SystemService{platform: platform, health: health, root: filepath.Clean(root)}
	// The health port is the daemon client in every real wiring, and it is
	// the thing that knows the address. Asked for rather than required, so a
	// test can pass a bare health check.
	if addr, ok := health.(Addressable); ok {
		svc.address = addr
	}
	return svc
}

// ServiceName is what Wails calls this service in the generated bindings.
func (s *SystemService) ServiceName() string { return "SystemService" }

// DaemonAddress reports where the daemon is, as an http(s) origin.
//
// The window needs this to open the realtime channel. Everything else it
// asks for goes through the Wails bridge, which needs no address — but the
// event channel is a WebSocket the webview opens itself, and a URL built
// from window.location there points at whatever is serving the interface,
// which is the application binary, not the daemon. Empty when this build
// has no daemon client, which is the browser case: there the page really is
// served by the daemon, and a same-origin URL is correct.
func (s *SystemService) DaemonAddress(context.Context) (string, error) {
	if s.address == nil {
		return "", nil
	}
	return s.address.BaseURL(), nil
}

// Ping reports whether the daemon is answering, which is what the splash
// window waits on.
func (s *SystemService) Ping(ctx context.Context) (bool, error) {
	if s.health == nil {
		return true, nil
	}
	return s.health.Ready(ctx)
}

// Version reports what this build is, for the about panel and for a bug report
// that would otherwise say "the latest one".
func (s *SystemService) Version(context.Context) (map[string]string, error) {
	return map[string]string{
		"name":    build.DisplayName,
		"version": build.Version,
		"commit":  build.Commit,
		"date":    build.Date,
	}, nil
}

// OpenPath asks the operating system to open a file with its default handler.
func (s *SystemService) OpenPath(ctx context.Context, path string) error {
	resolved, err := s.inside(path)
	if err != nil {
		return err
	}
	return s.platform.OpenPath(ctx, resolved)
}

// RevealInFolder shows a file in the system file browser.
func (s *SystemService) RevealInFolder(ctx context.Context, path string) error {
	resolved, err := s.inside(path)
	if err != nil {
		return err
	}
	return s.platform.RevealInFolder(ctx, resolved)
}

// OpenExternal opens a URL in the user's browser.
//
// Anything that is not http or https is refused. Without the check, a link in
// a model's answer could invoke a local handler — file://, or a scheme another
// application registered — and the renderer would have handed the machine to
// whatever wrote that text.
func (s *SystemService) OpenExternal(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return errUnsafeURL(rawURL, "")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errUnsafeURL(rawURL, parsed.Scheme)
	}
	if parsed.Host == "" {
		return errUnsafeURL(rawURL, parsed.Scheme)
	}
	return s.platform.OpenExternal(ctx, parsed.String())
}

// PickFiles opens the system file dialog.
func (s *SystemService) PickFiles(ctx context.Context, opts PickOptions) ([]string, error) {
	picked, err := s.platform.PickFiles(ctx, opts)
	if err != nil {
		return nil, err
	}
	// A cancelled dialog is an empty answer, and the caller reads len() rather
	// than distinguishing nil from empty.
	if picked == nil {
		return []string{}, nil
	}
	return picked, nil
}

// SetAppearance syncs the native window material with the theme the interface
// switched to.
func (s *SystemService) SetAppearance(ctx context.Context, appearance, windows string) error {
	switch appearance {
	case "light", "dark", "auto":
	default:
		return errUnknownAppearance(appearance)
	}
	switch windows {
	case "", "solid", "blur":
	default:
		return errUnknownWindows(windows)
	}
	return s.platform.SetAppearance(ctx, appearance, windows)
}

// inside resolves a path against the workspace and refuses anything outside it.
func (s *SystemService) inside(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errPathOutside(path)
	}
	candidate := trimmed
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(s.root, candidate)
	}
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(s.root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errPathOutside(path)
	}
	return candidate, nil
}

func errUnsafeURL(raw, scheme string) error {
	return apperr.New("SYSTEM_UNSAFE_URL").
		Causer("wailsvc.SystemService.OpenExternal").
		Msgf("only http and https links are opened outside the application").
		Issue("url", raw).
		Issue("scheme", scheme).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "a link with another scheme would hand the machine to whatever wrote it; open the file with the file explorer instead",
		})
}

func errPathOutside(path string) error {
	return apperr.New("SYSTEM_PATH_OUTSIDE_WORKSPACE").
		Causer("wailsvc.SystemService").
		Msgf("this path is outside the workspace the window is looking at").
		Issue("path", path).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "the desktop opens files of the active workspace; switch workspace to reach another tree",
		})
}

func errUnknownAppearance(appearance string) error {
	return apperr.New("SYSTEM_UNKNOWN_APPEARANCE").
		Causer("wailsvc.SystemService.SetAppearance").
		Msgf("%q is not an appearance", appearance).
		Issue("appearance", appearance).
		Issue("valid", []string{"light", "dark", "auto"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use light, dark or auto"})
}

func errUnknownWindows(windows string) error {
	return apperr.New("SYSTEM_UNKNOWN_WINDOW_MATERIAL").
		Causer("wailsvc.SystemService.SetAppearance").
		Msgf("%q is not a window material", windows).
		Issue("windows", windows).
		Issue("valid", []string{"solid", "blur"}).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use solid or blur"})
}
