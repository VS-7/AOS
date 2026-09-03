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
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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

	// PickSavePath opens the system save dialog and returns the path chosen.
	// A cancelled dialog returns "" and no error, for the same reason.
	PickSavePath(ctx context.Context, opts SaveOptions) (string, error)

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

// SaveOptions describes the save dialog.
type SaveOptions struct {
	Title    string `json:"title,omitempty"`
	Filename string `json:"filename,omitempty"`
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

	// supervisor is what actually restarts the daemon, when this window has
	// one. The daemon refuses to restart itself — Stop would signal its own
	// pid — so the button in Settings › Daemon had nothing behind it: it
	// asked the daemon to terminate itself, got a dropped connection, and
	// left the window with no daemon at all.
	//
	// Nil in a build with no supervisor (a browser tab against a server), and
	// the method says so rather than pretending.
	supervisor Supervisor

	// root is the workspace the window is looking at. Paths handed to the
	// operating system are resolved inside it, because a renderer that could
	// ask the shell to open any path is a renderer with a filesystem browser
	// nobody designed.
	//
	// It is empty until a workspace is known, and can be set once one is —
	// see SetWorkspaceRoot. The mutex is because that happens on the
	// goroutine that resolves the workspace over HTTP, while the reads happen
	// on whichever goroutine Wails dispatches a call from.
	mu   sync.RWMutex
	root string

	// launchDir is the directory the window was started in, when it was
	// started in one — a terminal inside a repository, or a shortcut with a
	// working directory. It is not the workspace: nothing is registered here,
	// and it is empty for an application opened from the dock.
	//
	// The onboarding wizard reads it to offer a default folder for the first
	// workspace. Until the wizard existed the desktop registered this
	// directory itself, behind the person's back and before they had named
	// anything; offering it instead keeps the useful half of that behaviour
	// and drops the part that overrode a choice nobody had made yet.
	launchDir string
}

// NewSystem builds the system service.
//
// An empty root is allowed and is what an installed application starts with:
// the workspace is resolved over HTTP after sign-in, long after this service
// has to exist. Until then nothing is inside the workspace and every path is
// refused, which is the safe end of that gap rather than the convenient one.
// Supervisor is the slice of gateway.Service this window needs: bring the
// daemon back, and say what state it ended in.
type Supervisor interface {
	Restart(ctx context.Context) error
}

// SetSupervisor installs what can restart the daemon. Called once, at wiring,
// by cmd/aos-desktop — which is the process that launched it.
func (s *SystemService) SetSupervisor(sup Supervisor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.supervisor = sup
}

// RestartDaemon stops the daemon and starts it again.
//
// It lives here rather than being a command because of who is allowed to do
// it: supervision belongs to whatever launched the daemon, and inside the
// daemon `gateway_restart` now refuses (AOS_GATEWAY_SELF_RESTART) instead of
// signalling its own pid and dying mid-request. This window is that
// supervisor, so this is the one path that can honour the button.
func (s *SystemService) RestartDaemon(ctx context.Context) error {
	s.mu.RLock()
	sup := s.supervisor
	s.mu.RUnlock()
	if sup == nil {
		return errNoSupervisor()
	}
	return sup.Restart(ctx)
}

func NewSystem(platform Platform, health Health, root string) *SystemService {
	svc := &SystemService{platform: platform, health: health, root: cleanRoot(root)}
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

// SetLaunchDirectory records where the window was started. Called once, at
// wiring, by cmd/aos-desktop.
func (s *SystemService) SetLaunchDirectory(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launchDir = strings.TrimSpace(dir)
}

// LaunchDirectory is where the window was started, or empty when it was
// started nowhere in particular.
//
// The wizard uses it as the default folder for the first workspace. Empty is a
// normal answer and means "let AOS pick one", which is what the field's own
// placeholder already promises.
func (s *SystemService) LaunchDirectory(context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.launchDir, nil
}

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

// Platform names the operating system this window is running on, as Go spells
// it: "darwin", "windows", "linux".
//
// The interface needs it to decide whether to draw its own minimise, maximise
// and close controls. It draws them where the window is frameless and there is
// nothing else to close it with, and must not draw them on macOS, where the
// window keeps its native traffic lights over full-size content — two sets of
// window controls in one corner is worse than none.
//
// It answers for the process, which is the window: a build runs on one
// platform.
func (s *SystemService) Platform(context.Context) (string, error) {
	return runtime.GOOS, nil
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

// SaveFile writes bytes the interface produced to a path the person chooses.
//
// It replaces `<a download>`, which is what the interface was ported with and
// what an Electron renderer supports. A WebView does not: downloading needs a
// download delegate on every platform (WKDownloadDelegate, WebView2's
// DownloadStarting, WebKitGTK's decide-destination) and Wails implements none
// of them, so the anchor click was accepted and nothing was ever written.
// Exporting a table, saving an image and downloading mcp.json all did nothing
// and said nothing.
//
// Content is base64 because it crosses the bridge as JSON and most of these
// are not text — an image saved from a conversation, above all.
//
// The workspace confinement that guards OpenPath deliberately does not apply.
// The path here is not one the interface chose: it comes back from the
// operating system's own save panel, which the person just used. Refusing to
// write where they said would make the feature useless — nobody saves an
// export into the workspace they are working in.
func (s *SystemService) SaveFile(ctx context.Context, filename, contentBase64 string) (string, error) {
	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return "", errUndecodableContent(filename, err)
	}
	if len(content) > maxSaveBytes {
		return "", errContentTooLarge(filename, len(content))
	}

	chosen, err := s.platform.PickSavePath(ctx, SaveOptions{Filename: filepath.Base(filename)})
	if err != nil {
		return "", err
	}
	if chosen == "" {
		// Cancelled. An empty answer, not a failure — the caller reads "" as
		// "nothing to report", exactly as PickFiles' empty slice is read.
		return "", nil
	}

	if err := os.WriteFile(chosen, content, 0o644); err != nil {
		return "", errUnwritable(chosen, err)
	}
	return chosen, nil
}

// maxSaveBytes caps what may cross the bridge in one call. Base64 inflates by
// a third and the whole thing is held in memory three times over — the string,
// the decoded bytes, and the JSON frame — so a cap is what keeps a mistyped
// export from taking the window down.
const maxSaveBytes = 64 << 20

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

// DroppedFile is one path dragged onto the window, resolved for the interface.
//
// The interface reads a file through the daemon, which addresses everything
// relative to the workspace root — so a bare absolute path from a drag is not
// something it can do anything with. Resolving it here rather than there keeps
// one answer to "where is the workspace", on the side that already has it.
type DroppedFile struct {
	// Name is the file's own name, for showing while it loads.
	Name string `json:"name"`
	// Path is relative to the workspace root, and empty when Inside is false.
	Path string `json:"path"`
	// Inside reports whether the file is in the workspace at all. A drag from
	// the Desktop is not, and the interface says so rather than failing.
	Inside bool `json:"inside"`
}

// ResolveDropped maps paths dragged onto the window to workspace-relative ones.
//
// Not a bound method: nothing in the interface calls it, and it would be a
// path-probing oracle if anything could. `cmd/aos-desktop` calls it on the way
// out, when Wails reports a drop.
func (s *SystemService) ResolveDropped(paths []string) []DroppedFile {
	resolved := make([]DroppedFile, 0, len(paths))
	for _, path := range paths {
		file := DroppedFile{Name: filepath.Base(path)}
		if abs, err := s.inside(path); err == nil {
			s.mu.RLock()
			root := s.root
			s.mu.RUnlock()
			if rel, err := filepath.Rel(root, abs); err == nil {
				file.Path = filepath.ToSlash(rel)
				file.Inside = true
			}
		}
		resolved = append(resolved, file)
	}
	return resolved
}

// SetWorkspaceRoot names the workspace this window is looking at, once it is
// known.
//
// The window opens before it can know: an installed application is launched
// with no directory in mind, and the answer comes back from the daemon after
// somebody has signed in. Passing "" to NewSystem and calling this later is
// the honest shape of that, and it is safe in between — see inside.
func (s *SystemService) SetWorkspaceRoot(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = cleanRoot(root)
}

// cleanRoot keeps an unset root unset. filepath.Clean("") is ".", the
// process's working directory — which for an application launched from Finder
// is "/", so confining paths "inside the workspace" would have confined them
// to the entire disk.
func cleanRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return filepath.Clean(root)
}

// inside resolves a path against the workspace and refuses anything outside it.
func (s *SystemService) inside(path string) (string, error) {
	s.mu.RLock()
	root := s.root
	s.mu.RUnlock()

	// No workspace means no inside. Every path is refused until one is known,
	// which is a few seconds at startup and never again.
	if root == "" {
		return "", errPathOutside(path)
	}

	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errPathOutside(path)
	}
	candidate := trimmed
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(root, candidate)
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

func errUndecodableContent(filename string, cause error) error {
	return apperr.New("SYSTEM_UNDECODABLE_CONTENT").
		Causer("wailsvc.SystemService.SaveFile").
		Msgf("the content to save is not valid base64").
		Issue("filename", filename).
		Issue("cause", cause.Error()).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "nothing was written and the save panel never opened; this is a fault in the interface, not the file — retry the save",
		})
}

func errContentTooLarge(filename string, size int) error {
	return apperr.New("SYSTEM_CONTENT_TOO_LARGE").
		Causer("wailsvc.SystemService.SaveFile").
		Msgf("this file is too large to save through the window").
		Issue("filename", filename).
		Issue("bytes", size).
		Status(apperr.StatusPayloadTooLarge).
		CTA(apperr.CallToAction{
			Label: "files this size belong in the workspace, where the daemon writes them directly",
		})
}

func errUnwritable(path string, cause error) error {
	return apperr.New("SYSTEM_PATH_UNWRITABLE").
		Causer("wailsvc.SystemService.SaveFile").
		Msgf("the chosen location could not be written to").
		Issue("path", path).
		Issue("cause", cause.Error()).
		Status(apperr.StatusInternalServerError)
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

// errNoSupervisor is what a window with nothing behind it answers.
//
// A browser tab against a server daemon is the honest case: it is a client of
// a process it did not launch, and restarting belongs to whatever did — a
// systemd unit, or a terminal.
func errNoSupervisor() error {
	return apperr.New("DESKTOP_NO_SUPERVISOR").
		Causer("wailsvc.SystemService.RestartDaemon").
		Msgf("this client did not launch the daemon and cannot restart it").
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label:   "restart it where it was started, or from a terminal",
			Command: build.Name + " gateway restart",
		})
}
