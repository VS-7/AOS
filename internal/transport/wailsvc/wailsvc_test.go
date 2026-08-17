package wailsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
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
	failWith   error
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
