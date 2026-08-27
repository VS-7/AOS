package wailsvc_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/transport/wailsvc"
)

// platform records what the operating system was asked to do. Every test here
// runs on it: what is being checked is what reaches the platform, not what the
// platform then does with it.
type platform struct {
	opened     []string
	revealed   []string
	external   []string
	appearance [2]string
	picked     []string
	// savePath is what the save panel answers with. Empty means the person
	// cancelled, which every caller has to treat as an ordinary outcome.
	savePath string
	saveOpts wailsvc.SaveOptions
	failWith error
}

func (p *platform) OpenPath(_ context.Context, path string) error {
	p.opened = append(p.opened, path)
	return p.failWith
}

func (p *platform) RevealInFolder(_ context.Context, path string) error {
	p.revealed = append(p.revealed, path)
	return p.failWith
}

func (p *platform) OpenExternal(_ context.Context, rawURL string) error {
	p.external = append(p.external, rawURL)
	return p.failWith
}

func (p *platform) PickSavePath(_ context.Context, opts wailsvc.SaveOptions) (string, error) {
	p.saveOpts = opts
	return p.savePath, p.failWith
}

func (p *platform) PickFiles(context.Context, wailsvc.PickOptions) ([]string, error) {
	return p.picked, p.failWith
}

func (p *platform) SetAppearance(_ context.Context, appearance, windows string) error {
	p.appearance = [2]string{appearance, windows}
	return p.failWith
}

type health struct {
	ready bool
	err   error
}

func (h health) Ready(context.Context) (bool, error) { return h.ready, h.err }

func ctx() context.Context { return context.Background() }

// authCaller is what AuthService talks to instead of the real daemonclient.
type authCaller struct {
	result   wailsvc.AuthResult
	status   wailsvc.AuthStatus
	session  wailsvc.PublicUser
	failWith error
}

func (a *authCaller) Status(context.Context) (wailsvc.AuthStatus, error) {
	return a.status, a.failWith
}

func (a *authCaller) Login(context.Context, string, string) (wailsvc.AuthResult, error) {
	return a.result, a.failWith
}

func (a *authCaller) Onboarding(context.Context, string, string, string) (wailsvc.AuthResult, error) {
	return a.result, a.failWith
}

func (a *authCaller) Logout(context.Context) error { return a.failWith }

func (a *authCaller) Session(context.Context) (wailsvc.PublicUser, error) {
	return a.session, a.failWith
}

// TestOpenExternalRefusesEverythingThatIsNotTheWeb. A link in a model's answer
// with a local scheme would hand the machine to whatever wrote that text.
func TestOpenExternalRefusesEverythingThatIsNotTheWeb(t *testing.T) {
	p := &platform{}
	svc := wailsvc.NewSystem(p, nil, t.TempDir())

	refused := []string{
		"file:///etc/passwd",
		"file://localhost/etc/passwd",
		"aos://task/123",
		"vscode://file/etc/passwd",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"ftp://example.invalid/x",
		"mailto:someone@example.invalid",
		"//example.invalid",
		"example.invalid",
		"",
		"   ",
	}
	for _, raw := range refused {
		err := svc.OpenExternal(ctx(), raw)
		if err == nil {
			t.Fatalf("%q was opened", raw)
		}
		got, ok := apperr.As(err)
		if !ok || !strings.HasSuffix(got.Code, "SYSTEM_UNSAFE_URL") {
			t.Fatalf("%q: error = %v", raw, err)
		}
	}
	if len(p.external) != 0 {
		t.Fatalf("the platform was asked to open %v", p.external)
	}

	for _, raw := range []string{"https://example.invalid/docs", "http://localhost:5326/"} {
		if err := svc.OpenExternal(ctx(), raw); err != nil {
			t.Fatalf("%q was refused: %v", raw, err)
		}
	}
	if len(p.external) != 2 {
		t.Fatalf("the platform opened %v", p.external)
	}
}

// TestAPathOutsideTheWorkspaceIsNotOpened. A renderer that could ask the shell
// to open any path is a filesystem browser nobody designed.
func TestAPathOutsideTheWorkspaceIsNotOpened(t *testing.T) {
	root := t.TempDir()
	p := &platform{}
	svc := wailsvc.NewSystem(p, nil, root)

	outside := []string{
		"/etc/passwd",
		"../../../etc/passwd",
		filepath.Join(root, "..", "elsewhere"),
		"",
	}
	for _, path := range outside {
		if err := svc.OpenPath(ctx(), path); err == nil {
			t.Fatalf("%q was opened", path)
		}
		if err := svc.RevealInFolder(ctx(), path); err == nil {
			t.Fatalf("%q was revealed", path)
		}
	}
	if len(p.opened)+len(p.revealed) != 0 {
		t.Fatal("the platform was asked about a path outside the workspace")
	}

	// A relative path is resolved against the workspace, which is how the
	// interface addresses a file it is showing.
	if err := svc.OpenPath(ctx(), "docs/README.md"); err != nil {
		t.Fatal(err)
	}
	if len(p.opened) != 1 || p.opened[0] != filepath.Join(root, "docs/README.md") {
		t.Fatalf("opened %v", p.opened)
	}
	// A path that merely starts with the root's name is not inside it.
	if err := svc.OpenPath(ctx(), root+"-elsewhere/x"); err == nil {
		t.Fatal("a sibling directory with a longer name passed as inside")
	}
}

// TestACancelledDialogIsAnEmptyAnswerAndNotAnError. The person changed their
// mind, which is not a failure.
func TestACancelledDialogIsAnEmptyAnswerAndNotAnError(t *testing.T) {
	svc := wailsvc.NewSystem(&platform{picked: nil}, nil, t.TempDir())

	got, err := svc.PickFiles(ctx(), wailsvc.PickOptions{Title: "Pick"})
	if err != nil {
		t.Fatalf("a cancelled dialog reported an error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty slice", got)
	}

	failing := wailsvc.NewSystem(&platform{failWith: errors.New("no display")}, nil, t.TempDir())
	if _, err := failing.PickFiles(ctx(), wailsvc.PickOptions{}); err == nil {
		t.Fatal("a dialog that could not open reported success")
	}
}

// TestSetAppearanceRefusesWhatTheWindowCannotBe.
func TestSetAppearanceRefusesWhatTheWindowCannotBe(t *testing.T) {
	p := &platform{}
	svc := wailsvc.NewSystem(p, nil, t.TempDir())

	if err := svc.SetAppearance(ctx(), "sepia", "solid"); err == nil {
		t.Fatal("an appearance that is not one was accepted")
	}
	if err := svc.SetAppearance(ctx(), "dark", "frosted"); err == nil {
		t.Fatal("a window material that is not one was accepted")
	}
	if p.appearance != [2]string{"", ""} {
		t.Fatalf("the platform was told %v", p.appearance)
	}

	if err := svc.SetAppearance(ctx(), "dark", "blur"); err != nil {
		t.Fatal(err)
	}
	if p.appearance != [2]string{"dark", "blur"} {
		t.Fatalf("the platform was told %v", p.appearance)
	}
	// An empty material means "whatever the theme said", which the window
	// resolves; it is not an error.
	if err := svc.SetAppearance(ctx(), "auto", ""); err != nil {
		t.Fatal(err)
	}
}

// TestPingIsWhatTheSplashWaitsOn.
func TestPingIsWhatTheSplashWaitsOn(t *testing.T) {
	waiting := wailsvc.NewSystem(&platform{}, health{ready: false}, t.TempDir())
	if ready, err := waiting.Ping(ctx()); err != nil || ready {
		t.Fatalf("ready = %v, %v", ready, err)
	}

	up := wailsvc.NewSystem(&platform{}, health{ready: true}, t.TempDir())
	if ready, err := up.Ping(ctx()); err != nil || !ready {
		t.Fatalf("ready = %v, %v", ready, err)
	}

	// With no health port the window is not left staring at a splash forever.
	none := wailsvc.NewSystem(&platform{}, nil, t.TempDir())
	if ready, err := none.Ping(ctx()); err != nil || !ready {
		t.Fatalf("ready = %v, %v", ready, err)
	}
}

// TestVersionSaysWhatThisBuildIs, so a bug report does not say "the latest one".
func TestVersionSaysWhatThisBuildIs(t *testing.T) {
	svc := wailsvc.NewSystem(&platform{}, nil, t.TempDir())
	got, err := svc.Version(ctx())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "version", "commit", "date"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("version is missing %q: %v", key, got)
		}
	}
}

// caller stands in for the daemon. What the desktop must prove is that it
// hands the call through unchanged and reports a daemon that is not answering —
// the validation itself belongs to the one registry, on the other side.
type caller struct {
	keys     []string
	payloads []string
	answer   string
	failWith error
	commands []wailsvc.CommandInfo
}

func (c *caller) Invoke(_ context.Context, key string, input json.RawMessage) (json.RawMessage, error) {
	c.keys = append(c.keys, key)
	c.payloads = append(c.payloads, string(input))
	if c.failWith != nil {
		return nil, c.failWith
	}
	if c.answer == "" {
		c.answer = `{"data":{"ok":true}}`
	}
	return json.RawMessage(c.answer), nil
}

func (c *caller) Commands(context.Context) ([]wailsvc.CommandInfo, error) {
	if c.failWith != nil {
		return nil, c.failWith
	}
	return c.commands, nil
}

// TestTheDesktopHandsTheCallThroughUnchanged. It is a client: the payload it
// received is the payload the daemon gets, because anything else would be a
// second place where an input is shaped.
func TestTheDesktopHandsTheCallThroughUnchanged(t *testing.T) {
	daemon := &caller{answer: `{"data":{"echo":"hello"}}`}
	svc := wailsvc.NewDomain(daemon)

	raw, err := svc.Invoke(ctx(), "  probe_echo  ", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"data":{"echo":"hello"}}` {
		t.Fatalf("the answer was rewritten: %s", raw)
	}
	if len(daemon.keys) != 1 || daemon.keys[0] != "probe_echo" {
		t.Fatalf("keys = %v — the name was not trimmed", daemon.keys)
	}
	if daemon.payloads[0] != `{"text":"hello"}` {
		t.Fatalf("the payload was rewritten: %s", daemon.payloads[0])
	}
}

// TestAnEmptyPayloadIsAnEmptyObject, because a command that takes nothing is
// called with nothing from the interface, and the daemon expects an object.
func TestAnEmptyPayloadIsAnEmptyObject(t *testing.T) {
	daemon := &caller{}
	svc := wailsvc.NewDomain(daemon)

	if _, err := svc.Invoke(ctx(), "probe_nothing", nil); err != nil {
		t.Fatal(err)
	}
	if daemon.payloads[0] != "{}" {
		t.Fatalf("payload = %q", daemon.payloads[0])
	}
}

// TestAWindowWithNoDaemonSaysSo, rather than failing somewhere deeper with an
// error that does not name the cause.
func TestAWindowWithNoDaemonSaysSo(t *testing.T) {
	svc := wailsvc.NewDomain(nil)

	_, err := svc.Invoke(ctx(), "probe_echo", nil)
	if err == nil {
		t.Fatal("a call with no daemon behind it reported success")
	}
	if got, ok := apperr.As(err); !ok || !strings.HasSuffix(got.Code, "DESKTOP_NO_DAEMON") {
		t.Fatalf("error = %v", err)
	}
	if _, err := svc.Commands(ctx()); err == nil {
		t.Fatal("the command list came back with no daemon behind it")
	}

	withDaemon := wailsvc.NewDomain(&caller{})
	if _, err := withDaemon.Invoke(ctx(), "   ", nil); err == nil {
		t.Fatal("a call with no command name was passed on")
	}
}

// TestTheFrontendCanCheckWhatTheDaemonHas, which is how a version mismatch
// becomes a message rather than a call that fails halfway through a screen.
func TestTheFrontendCanCheckWhatTheDaemonHas(t *testing.T) {
	daemon := &caller{commands: []wailsvc.CommandInfo{
		{Key: "tasks_list", Group: "tasks", Name: "list", ReadOnly: true},
		{Key: "agents_list", Group: "agents", Name: "list", ReadOnly: true},
	}}
	svc := wailsvc.NewDomain(daemon)

	got, err := svc.Commands(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Key != "agents_list" {
		t.Fatalf("commands = %+v — the list is not sorted", got)
	}
}

// TestAFailureFromTheDaemonReachesTheWindow rather than being swallowed.
func TestAFailureFromTheDaemonReachesTheWindow(t *testing.T) {
	svc := wailsvc.NewDomain(&caller{failWith: errors.New("the daemon is down")})

	if _, err := svc.Invoke(ctx(), "probe_echo", nil); err == nil {
		t.Fatal("a failed call reported success")
	}
	if _, err := svc.Commands(ctx()); err == nil {
		t.Fatal("a failed listing reported success")
	}
}

// TestTheServicesNameThemselves, because the generated bindings are keyed by it
// and a rename would silently move every call in the frontend.
func TestTheServicesNameThemselves(t *testing.T) {
	cases := map[string]interface{ ServiceName() string }{
		"SystemService": wailsvc.NewSystem(&platform{}, nil, t.TempDir()),
		"DomainService": wailsvc.NewDomain(nil),
		"AuthService":   wailsvc.NewAuth(nil, nil),
	}
	for want, svc := range cases {
		if got := svc.ServiceName(); got != want {
			t.Errorf("service is named %q, want %q", got, want)
		}
	}
}

// TestAuthWithNoDaemonSaysSo, the same shape as DomainService's equivalent:
// a window that has not found its daemon yet gets a named cause, not a
// generic failure three layers down.
func TestAuthWithNoDaemonSaysSo(t *testing.T) {
	svc := wailsvc.NewAuth(nil, nil)

	if _, err := svc.Status(ctx()); err == nil {
		t.Fatal("a status check with no daemon behind it reported success")
	}
	if _, err := svc.Login(ctx(), "vitor", "whatever"); err == nil {
		t.Fatal("a login with no daemon behind it reported success")
	}
	if _, err := svc.Onboarding(ctx(), "Vitor", "vitor@example.test", "whatever"); err == nil {
		t.Fatal("an onboarding with no daemon behind it reported success")
	}
	if err := svc.Logout(ctx()); err == nil {
		t.Fatal("a logout with no daemon behind it reported success")
	}
	if _, err := svc.Session(ctx()); err == nil {
		t.Fatal("a session check with no daemon behind it reported success")
	}
}

// TestAuthRunsAfterAuthOnlyOnSuccess: the workspace registration this hook
// exists for needs a token Login/Onboarding only just minted — running it
// after a failure would register a workspace nobody is signed in to see.
func TestAuthRunsAfterAuthOnlyOnSuccess(t *testing.T) {
	var hookRuns int
	hook := func(context.Context) { hookRuns++ }

	ok := wailsvc.NewAuth(&authCaller{result: wailsvc.AuthResult{User: wailsvc.PublicUser{ID: "u1"}}}, hook)
	if _, err := ok.Login(ctx(), "vitor", "whatever"); err != nil {
		t.Fatal(err)
	}
	if hookRuns != 1 {
		t.Fatalf("hookRuns after a successful login = %d, want 1", hookRuns)
	}

	failing := wailsvc.NewAuth(&authCaller{failWith: errors.New("wrong password")}, hook)
	if _, err := failing.Login(ctx(), "vitor", "wrong"); err == nil {
		t.Fatal("a failed login reported success")
	}
	if hookRuns != 1 {
		t.Fatalf("hookRuns after a failed login = %d, want still 1", hookRuns)
	}

	if _, err := ok.Onboarding(ctx(), "Vitor", "vitor@example.test", "whatever"); err != nil {
		t.Fatal(err)
	}
	if hookRuns != 2 {
		t.Fatalf("hookRuns after a successful onboarding = %d, want 2", hookRuns)
	}
}

// TestAnUnknownWorkspaceConfinesNothingToTheWorkingDirectory.
//
// An installed application is opened without a directory in mind, so the
// window starts before it knows which workspace it is for and hands this
// service an empty root. filepath.Clean("") is ".", the process's working
// directory — which for an application launched from Finder is "/", the whole
// disk. Confining "inside the workspace" to "inside /" confines nothing, and
// this is the boundary that keeps the interface from asking the shell to
// reveal any file on the machine.
//
// Until a workspace is known, nothing is inside it.
func TestAnUnknownWorkspaceConfinesNothingToTheWorkingDirectory(t *testing.T) {
	svc := wailsvc.NewSystem(&platform{}, nil, "")

	for _, path := range []string{"/etc/passwd", "some/file.txt", "."} {
		if err := svc.OpenPath(ctx(), path); err == nil {
			t.Errorf("%q was opened while no workspace was known", path)
		}
		if err := svc.RevealInFolder(ctx(), path); err == nil {
			t.Errorf("%q was revealed while no workspace was known", path)
		}
	}
}

// TestTheWorkspaceIsLearnedAfterTheWindowOpens: an installed application
// resolves its workspace over HTTP, after sign-in, which is later than this
// service is constructed.
func TestTheWorkspaceIsLearnedAfterTheWindowOpens(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(inside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := wailsvc.NewSystem(&platform{}, nil, "")
	svc.SetWorkspaceRoot(dir)

	if err := svc.OpenPath(ctx(), inside); err != nil {
		t.Fatalf("a path inside the workspace was refused after it was set: %v", err)
	}
	if err := svc.OpenPath(ctx(), "/etc/passwd"); err == nil {
		t.Fatal("a path outside the workspace was opened")
	}
}

// TestPlatformNamesTheOperatingSystem. The interface draws its own minimise,
// maximise and close controls on the platforms whose window is frameless, and
// must not draw a second set next to the ones macOS draws itself. A user-agent
// guess is what it has until this answers.
func TestPlatformNamesTheOperatingSystem(t *testing.T) {
	svc := wailsvc.NewSystem(&platform{}, nil, t.TempDir())

	got, err := svc.Platform(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if got != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", got, runtime.GOOS)
	}
}

// TestADroppedFileIsResolvedAgainstTheWorkspace.
//
// A file dragged onto the window arrives as an absolute path, and the
// interface reads files through the daemon, which addresses them relative to
// the workspace root. Nothing in the window knows that root — so an
// unresolved path is a path the interface can do nothing with, which is the
// state a drag was in: accepted by the operating system and then dropped.
func TestADroppedFileIsResolvedAgainstTheWorkspace(t *testing.T) {
	root := t.TempDir()
	svc := wailsvc.NewSystem(&platform{}, health{ready: true}, root)

	resolved := svc.ResolveDropped([]string{
		filepath.Join(root, "notes", "todo.md"),
	})

	if len(resolved) != 1 {
		t.Fatalf("resolved %d files, want 1", len(resolved))
	}
	if !resolved[0].Inside {
		t.Error("a file in the workspace was reported as outside it")
	}
	if resolved[0].Path != "notes/todo.md" {
		t.Errorf("path = %q, want the workspace-relative one", resolved[0].Path)
	}
	if resolved[0].Name != "todo.md" {
		t.Errorf("name = %q, want the file's own name", resolved[0].Name)
	}
}

// TestADroppedFileOutsideTheWorkspaceIsReportedNotRead.
//
// The desktop's file access is confined to the workspace the window is looking
// at, and a drag is not a reason to widen it. The interface needs to be able to
// say so, which means the refusal has to survive as an answer rather than
// vanish from the list.
func TestADroppedFileOutsideTheWorkspaceIsReportedNotRead(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "secrets.env")
	svc := wailsvc.NewSystem(&platform{}, health{ready: true}, root)

	resolved := svc.ResolveDropped([]string{elsewhere})

	if len(resolved) != 1 {
		t.Fatalf("resolved %d files, want the refusal to still be listed", len(resolved))
	}
	if resolved[0].Inside {
		t.Error("a file outside the workspace was reported as inside it")
	}
	if resolved[0].Path != "" {
		t.Errorf("path = %q, want nothing the interface could read", resolved[0].Path)
	}
	if resolved[0].Name != "secrets.env" {
		t.Errorf("name = %q, want the name so the refusal can say which file", resolved[0].Name)
	}
}

// TestNoWorkspaceMeansNoDroppedFileIsInsideIt. The window opens before it
// knows which workspace it is for, and "" as a root would otherwise clean to
// the process's working directory — "/" for an application launched from
// Finder, which would make the whole disk the workspace.
func TestNoWorkspaceMeansNoDroppedFileIsInsideIt(t *testing.T) {
	svc := wailsvc.NewSystem(&platform{}, health{ready: true}, "")

	for _, file := range svc.ResolveDropped([]string{"/etc/hosts", "relative.txt"}) {
		if file.Inside {
			t.Errorf("%q was reported as inside a workspace that is not known yet", file.Name)
		}
	}
}

// TestSavingAFileWritesWhereThePersonChose.
//
// `<a download>` is what the interface was ported with, and what an Electron
// renderer supports. A WebView does not: every platform needs a download
// delegate and Wails implements none, so seven export and save actions —
// a table to CSV, an image from a conversation, mcp.json — clicked an anchor
// that wrote nothing and reported nothing.
func TestSavingAFileWritesWhereThePersonChose(t *testing.T) {
	chosen := filepath.Join(t.TempDir(), "export.csv")
	p := &platform{savePath: chosen}
	svc := wailsvc.NewSystem(p, health{ready: true}, t.TempDir())

	written, err := svc.SaveFile(ctx(), "export.csv", base64.StdEncoding.EncodeToString([]byte("a,b\n1,2\n")))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if written != chosen {
		t.Errorf("wrote to %q, want %q", written, chosen)
	}

	content, err := os.ReadFile(chosen)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(content) != "a,b\n1,2\n" {
		t.Errorf("content = %q, want what the interface produced", content)
	}
	if p.saveOpts.Filename != "export.csv" {
		t.Errorf("the panel opened on %q, want the suggested name", p.saveOpts.Filename)
	}
}

// TestSavingOutsideTheWorkspaceIsAllowed.
//
// The confinement that guards OpenPath deliberately does not apply here: this
// path did not come from the interface, it came back from the operating
// system's own save panel, which the person just used. Nobody saves an export
// into the workspace they are working in, and refusing would make the whole
// feature useless.
func TestSavingOutsideTheWorkspaceIsAllowed(t *testing.T) {
	workspace := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "downloaded.png")
	svc := wailsvc.NewSystem(&platform{savePath: elsewhere}, health{ready: true}, workspace)

	if _, err := svc.SaveFile(ctx(), "downloaded.png", base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G'})); err != nil {
		t.Fatalf("a path the person chose in the save panel was refused: %v", err)
	}
	if _, err := os.Stat(elsewhere); err != nil {
		t.Fatalf("nothing was written: %v", err)
	}
}

// TestACancelledSavePanelWritesNothingAndIsNotAnError. Same contract as
// PickFiles: changing your mind is an empty answer, not a failure the person
// needs a dialog about.
func TestACancelledSavePanelWritesNothingAndIsNotAnError(t *testing.T) {
	svc := wailsvc.NewSystem(&platform{savePath: ""}, health{ready: true}, t.TempDir())

	written, err := svc.SaveFile(ctx(), "export.csv", base64.StdEncoding.EncodeToString([]byte("x")))
	if err != nil {
		t.Fatalf("a cancelled panel was reported as a failure: %v", err)
	}
	if written != "" {
		t.Errorf("path = %q, want nothing", written)
	}
}

// TestUndecodableContentIsRefusedBeforeAnyPanelOpens. Opening a save panel and
// then failing to write is the worst order to do this in: the person picks a
// location, waits, and is told the content was never valid.
func TestUndecodableContentIsRefusedBeforeAnyPanelOpens(t *testing.T) {
	p := &platform{savePath: filepath.Join(t.TempDir(), "x")}
	svc := wailsvc.NewSystem(p, health{ready: true}, t.TempDir())

	_, err := svc.SaveFile(ctx(), "export.csv", "not base64 at all!!")
	if err == nil {
		t.Fatal("undecodable content was accepted")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "AOS_SYSTEM_UNDECODABLE_CONTENT" {
		t.Errorf("error = %v, want AOS_SYSTEM_UNDECODABLE_CONTENT", err)
	}
	if p.saveOpts.Filename != "" {
		t.Error("the save panel was opened before the content was checked")
	}
}

// TestSessionAnswersTheSameShapeAsHTTP is the bug that made the desktop window
// look signed out while it was signed in.
//
// lib/auth.ts tries the bridge and falls back to the daemon's own
// GET /api/auth/session, and every caller writes `const { user } = await
// session()`. HTTP answers {"data":{"user":{...}}}; this method used to answer
// the bare user, so inside the desktop that destructuring produced undefined —
// no name in the sidebar, an empty account form, and `user.role === "super"`
// false, which is the condition that renders the button for creating a
// workspace. Signed in, and invisible.
func TestSessionAnswersTheSameShapeAsHTTP(t *testing.T) {
	who := wailsvc.PublicUser{ID: "u-1", Name: "Vitor", Username: "vitor", Email: "vitor@example.test", Role: "super"}
	svc := wailsvc.NewAuth(&authCaller{session: who}, nil)

	got, err := svc.Session(ctx())
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if got.User != who {
		t.Errorf("Session().User = %+v, want %+v", got.User, who)
	}

	// The wrapper is the contract, not an implementation detail: the JSON the
	// window receives has to carry a `user` key for the caller to destructure.
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["user"]; !ok {
		t.Errorf("Session answered %s — the interface reads .user from this", encoded)
	}
}

// TestSessionReportsTheFailureRatherThanAnEmptyUser. Wrapping must not turn a
// refusal into a successful answer carrying a blank account, which would read
// to the interface as "signed in as nobody".
func TestSessionReportsTheFailureRatherThanAnEmptyUser(t *testing.T) {
	svc := wailsvc.NewAuth(&authCaller{failWith: errors.New("no session")}, nil)

	got, err := svc.Session(ctx())
	if err == nil {
		t.Fatal("a failed session check reported success")
	}
	if got.User != (wailsvc.PublicUser{}) {
		t.Errorf("a failed session check carried a user: %+v", got.User)
	}
}

func TestInstallSkillWritesIntoTheNamedAgentAndListsIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	svc := wailsvc.NewSystem(&platform{}, health{ready: true}, "")

	result, err := svc.InstallSkill(ctx(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".codex", "skills", "aos")
	if len(result.Installed) != 1 || result.Installed[0] != want {
		t.Fatalf("installed = %v, want %s", result.Installed, want)
	}
	if _, err := os.Stat(filepath.Join(want, "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	targets, err := svc.SkillTargets(ctx())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, target := range targets {
		if target.ID == "codex" {
			found = true
			if !target.Present || !target.Installed {
				t.Fatalf("codex = %+v, want present and installed", target)
			}
		}
	}
	if !found {
		t.Fatal("codex is not among the targets")
	}
}

func TestInstallSkillRefusesAnUnknownAgentAndAnEmptyMachine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := wailsvc.NewSystem(&platform{}, health{ready: true}, "")

	if _, err := svc.InstallSkill(ctx(), "vim"); err == nil {
		t.Fatal("an unknown agent was accepted")
	} else if e, ok := apperr.As(err); !ok || e.Code != "AOS_SKILL_UNKNOWN_TARGET" {
		t.Fatalf("err = %v", err)
	}
	if _, err := svc.InstallSkill(ctx(), "all"); err == nil {
		t.Fatal("a machine with no agents installed somewhere")
	}
}
