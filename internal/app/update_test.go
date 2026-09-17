package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/releasesource"
	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/adapters/updateinstall"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/core/identity"
	"github.com/OWNER/aos/internal/core/relsig"
	"github.com/OWNER/aos/internal/domain/auth"
	"github.com/OWNER/aos/internal/domain/gateway"
	"github.com/OWNER/aos/internal/domain/update"
)

// realGateway is the gateway exactly as wire.go builds it, over a scratch
// state directory, inside the daemon or outside it.
func realGateway(t *testing.T, dir string, inside bool) *gateway.Service {
	t.Helper()
	return gatewayOn(t, dir, inside, 1)
}

// gatewayOn is realGateway for a daemon configured to answer on port.
func gatewayOn(t *testing.T, dir string, inside bool, port int) *gateway.Service {
	t.Helper()
	return gateway.NewService(gateway.Deps{
		Processes: supervise.NewProcesses(),
		Health:    supervise.NewHealth(),
		Store:     supervise.NewStore(filepath.Join(dir, "gateway.json")),
		Locker:    supervise.NewLock(filepath.Join(dir, "gateway.lock")),
		Resolver:  supervise.Resolver{Explicit: filepath.Join(dir, "no-such-aosd"), Args: []string{"serve"}},
		Clock:     clockx.System{},
		Sleeper:   supervise.Sleeper{},
		Host:      "127.0.0.1",
		Port:      port,
		Inside:    inside,
	})
}

// releasePlatform is the platform the test release is published for, which
// the services under test are told they run on.
const releasePlatform = "linux/amd64"

// installDaemon puts a stand-in daemon binary in binDir and returns its path.
func installDaemon(t *testing.T, binDir string) string {
	t.Helper()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	aosd := filepath.Join(binDir, "aosd")
	if err := os.WriteFile(aosd, []byte("the running daemon"), 0o755); err != nil {
		t.Fatal(err)
	}
	return aosd
}

// publishRelease serves a feed publishing aosd v0.10.0 for releasePlatform,
// signed with a key of its own, and returns the feed, that key and the asset.
func publishRelease(t *testing.T) (base, pub string, asset []byte) {
	t.Helper()
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	asset = []byte("aosd v0.10.0")
	sum := sha256.Sum256(asset)
	checksums := hex.EncodeToString(sum[:]) + "  aosd_v0.10.0_linux_amd64\n"
	sig, err := relsig.Sign(priv, []byte(checksums))
	if err != nil {
		t.Fatal(err)
	}
	feed := http.NewServeMux()
	feed.HandleFunc("/stable.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(update.Release{
			Version: "v0.10.0", ChecksumsURL: base + "/checksums.txt", SignatureURL: base + "/checksums.txt.sig",
			Assets: []update.Asset{{Binary: "aosd", Platform: releasePlatform, URL: base + "/aosd", Filename: "aosd_v0.10.0_linux_amd64"}},
		})
	})
	feed.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(checksums)) })
	feed.HandleFunc("/checksums.txt.sig", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(sig)) })
	feed.HandleFunc("/aosd", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(asset) })
	srv := httptest.NewServer(feed)
	t.Cleanup(srv.Close)
	base = srv.URL
	return base, pub, asset
}

// TestApplyInsideTheDaemonRefusesBeforeTouchingAnything wires the real
// pieces — the gateway as the daemon builds it, the filesystem installer, the
// HTTP release source and a real signature — because the defect lived in how
// they fit together: every unit test ran Apply over a fake supervisor that
// restarted happily, while the real one inside the daemon refuses, so Apply
// swapped the binary, failed to restart, failed to restart again on rollback,
// and reported "the daemon may be down" from a daemon that was answering.
func TestApplyInsideTheDaemonRefusesBeforeTouchingAnything(t *testing.T) {
	root := t.TempDir()
	binDir, stateDir := filepath.Join(root, "bin"), filepath.Join(root, "state")
	aosd := installDaemon(t, binDir)
	base, pub, asset := publishRelease(t)

	inside := realGateway(t, stateDir, true)
	installer := updateinstall.New(filepath.Join(stateDir, "staged"), binDir)
	svc := update.NewService(update.Deps{
		Source:    releasesource.New(base),
		Stager:    installer,
		Installer: installer,
		Store:     updateinstall.NewStore(filepath.Join(stateDir, "state.json")),
		Supervisor: updateSupervisor{
			inside:  inside,
			outside: realGateway(t, stateDir, false),
			serving: func() bool { return true },
		},
		Operators:  updateOperators{},
		ActiveWork: updateActiveWork{},
		Clock:      clockx.System{},
		Sleeper:    supervise.Sleeper{},
		PublicKey:  pub,
		Platform:   releasePlatform,
		Version:    "v0.9.0",
	})
	ctx := context.Background()

	checked, err := svc.Check(ctx, update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if checked.State != update.StateAvailable || checked.Install == nil || checked.Install.Method != update.InstallFromTerminal {
		t.Fatalf("inside the daemon an update is installed from a terminal, got %+v", checked)
	}
	if _, err := svc.Download(ctx, update.DownloadInput{Release: checked.Release}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.Apply(ctx, update.ApplyInput{Version: "v0.10.0"})
	e, ok := apperr.As(err)
	if !ok || e.Code != "AOS_UPDATE_RESTART_UNAVAILABLE" {
		t.Fatalf("expected AOS_UPDATE_RESTART_UNAVAILABLE, got %v", err)
	}
	if got, _ := os.ReadFile(aosd); string(got) != "the running daemon" {
		t.Fatalf("the live daemon binary was replaced: %q", got)
	}
	if _, err := os.Stat(aosd + ".prev"); !os.IsNotExist(err) {
		t.Fatal("no backup should exist: nothing was swapped")
	}
	if got, _ := os.ReadFile(filepath.Join(stateDir, "staged", "aosd")); string(got) != string(asset) {
		t.Fatal("the staged release must survive the refusal")
	}

	// And the reason it refused is the one the gateway really gives inside
	// the daemon — not a fake's.
	err = updateSupervisor{inside: inside, outside: realGateway(t, stateDir, false), serving: func() bool { return true }}.Restart(ctx)
	if e, ok := apperr.As(err); !ok || e.Code != "AOS_GATEWAY_SELF_RESTART" {
		t.Fatalf("inside the daemon a restart is refused, got %v", err)
	}
}

// fakeDaemon answers the health check the way a daemon does, as process pid
// on version, and returns the port it answers on.
func fakeDaemon(t *testing.T, version string, pid int) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "aos", "status": "ok", "version": version, "pid": pid})
	}))
	t.Cleanup(srv.Close)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// A terminal `aosd update apply` next to a daemon somebody started by hand —
// `aosd serve`, a service manager — has no process of its own to restart. It
// used to swap the binaries anyway: the version-blind health probe found the
// old daemon answering and reported the install done, deleting the backup,
// while the old release went on serving; once the gateway refused to start a
// second daemon beside it, the rollback's restart was refused the same way
// and the answer was "the daemon did not come back up", about a daemon that
// never went down.
func TestTerminalApplyRefusesADaemonItsSupervisorDidNotStart(t *testing.T) {
	root := t.TempDir()
	binDir, stateDir := filepath.Join(root, "bin"), filepath.Join(root, "state")
	aosd := installDaemon(t, binDir)
	base, pub, asset := publishRelease(t)
	// Serving v0.9.0 as a process no gateway record names.
	port := fakeDaemon(t, "v0.9.0", 424242)

	installer := updateinstall.New(filepath.Join(stateDir, "staged"), binDir)
	svc := update.NewService(update.Deps{
		Source:    releasesource.New(base),
		Stager:    installer,
		Installer: installer,
		Store:     updateinstall.NewStore(filepath.Join(stateDir, "state.json")),
		Supervisor: updateSupervisor{
			inside:   gatewayOn(t, stateDir, true, port),
			outside:  gatewayOn(t, stateDir, false, port),
			serving:  func() bool { return false },
			host:     "127.0.0.1",
			port:     port,
			identify: supervise.NewHealth().Identify,
		},
		Operators:     updateOperators{},
		ActiveWork:    updateActiveWork{},
		Clock:         clockx.System{},
		Sleeper:       supervise.Sleeper{},
		PublicKey:     pub,
		Platform:      releasePlatform,
		Version:       "v0.9.0",
		HealthTimeout: time.Second,
	})
	ctx := context.Background()

	checked, err := svc.Check(ctx, update.CheckInput{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Download(ctx, update.DownloadInput{Release: checked.Release}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.Apply(ctx, update.ApplyInput{Version: "v0.10.0"})
	e, ok := apperr.As(err)
	if !ok || e.Code != "AOS_UPDATE_DAEMON_NOT_SUPERVISED" {
		t.Fatalf("expected AOS_UPDATE_DAEMON_NOT_SUPERVISED, got %v", err)
	}
	if got, _ := os.ReadFile(aosd); string(got) != "the running daemon" {
		t.Fatalf("the live daemon binary was replaced: %q", got)
	}
	if _, err := os.Stat(aosd + ".prev"); !os.IsNotExist(err) {
		t.Fatal("no backup should exist: nothing was swapped")
	}
	if got, _ := os.ReadFile(filepath.Join(stateDir, "staged", "aosd")); string(got) != string(asset) {
		t.Fatal("the staged release must survive the refusal")
	}
}

// Which daemon answers is read from the outside gateway's record and from the
// daemon's own health answer together: the record says which process the
// supervisor started, the answer says which process is serving.
func TestUpdateSupervisorObservesWhichDaemonAnswers(t *testing.T) {
	ctx := context.Background()
	observe := func(t *testing.T, dir string, port int, serving bool) update.Daemon {
		t.Helper()
		sup := updateSupervisor{
			inside:   gatewayOn(t, dir, true, port),
			outside:  gatewayOn(t, dir, false, port),
			serving:  func() bool { return serving },
			host:     "127.0.0.1",
			port:     port,
			identify: supervise.NewHealth().Identify,
		}
		d, err := sup.Observe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	record := func(t *testing.T, dir string, pid, port int) {
		t.Helper()
		if err := supervise.NewStore(filepath.Join(dir, "gateway.json")).Write(ctx, gateway.Meta{PID: pid, Host: "127.0.0.1", Port: port}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("nothing runs", func(t *testing.T) {
		d := observe(t, t.TempDir(), 1, false)
		if d.Self || d.Answering || d.RecordedPID != 0 || d.Address != "127.0.0.1:1" {
			t.Fatalf("daemon = %+v", d)
		}
	})
	t.Run("a daemon started by hand", func(t *testing.T) {
		d := observe(t, t.TempDir(), fakeDaemon(t, "v0.9.0", 424242), false)
		if !d.Answering || d.PID != 424242 || d.Version != "v0.9.0" || d.RecordedPID != 0 {
			t.Fatalf("daemon = %+v", d)
		}
	})
	t.Run("the daemon the supervisor started", func(t *testing.T) {
		dir := t.TempDir()
		// This test's own process stands in for the recorded daemon: it is
		// alive, and nothing here restarts it.
		port := fakeDaemon(t, "v0.10.0", os.Getpid())
		record(t, dir, os.Getpid(), port)
		d := observe(t, dir, 1, false)
		if !d.Answering || d.RecordedPID != os.Getpid() || d.PID != os.Getpid() || d.Version != "v0.10.0" {
			t.Fatalf("daemon = %+v", d)
		}
		if d.Address != net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) {
			t.Fatalf("the recorded daemon is looked for where the record says, got %s", d.Address)
		}
	})
	t.Run("another daemon where the supervisor's should be", func(t *testing.T) {
		dir := t.TempDir()
		port := fakeDaemon(t, "v0.9.0", 424242)
		record(t, dir, os.Getpid(), port)
		d := observe(t, dir, port, false)
		if !d.Answering || d.RecordedPID != os.Getpid() || d.PID != 424242 {
			t.Fatalf("daemon = %+v", d)
		}
	})
	t.Run("the daemon itself", func(t *testing.T) {
		d := observe(t, t.TempDir(), 1, true)
		if !d.Self || !d.Answering || d.PID != os.Getpid() || d.Version != build.Version {
			t.Fatalf("daemon = %+v", d)
		}
	})
}

func TestUpdateSupervisorRestartsFromOutsideOnlyOutsideTheDaemon(t *testing.T) {
	dir := t.TempDir()
	serving := false
	sup := updateSupervisor{
		inside:  realGateway(t, dir, true),
		outside: realGateway(t, dir, false),
		serving: func() bool { return serving },
	}
	ctx := context.Background()
	// Nothing is running and the daemon binary does not exist: the outside
	// gateway tries, and fails to start it — which is not a self-restart
	// refusal.
	if err := sup.Restart(ctx); err == nil {
		t.Fatal("starting a daemon that does not exist should fail")
	} else if e, ok := apperr.As(err); ok && e.Code == "AOS_GATEWAY_SELF_RESTART" {
		t.Fatal("outside the daemon the restart must go through the outside gateway")
	}

	serving = true
	if e, ok := apperr.As(sup.Restart(ctx)); !ok || e.Code != "AOS_GATEWAY_SELF_RESTART" {
		t.Fatal("the daemon cannot restart itself")
	}
}

type usersFile struct{ users []auth.User }

func (u *usersFile) Load(context.Context) ([]auth.User, error) { return u.users, nil }
func (u *usersFile) Save(_ context.Context, users []auth.User) error {
	u.users = users
	return nil
}

type brokenUsers struct{}

func (brokenUsers) Load(context.Context) ([]auth.User, error) {
	return nil, errors.New("users.json unreadable")
}
func (brokenUsers) Save(context.Context, []auth.User) error { return nil }

// update_download and update_apply were open to any valid token. Installing
// replaces the binaries every account runs, so it takes a super account.
func TestOnlyAnAdministratorMayInstallUpdates(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	users := &usersFile{users: []auth.User{
		{ID: "u-super", Username: "owner", Role: auth.Super, CreatedAt: now},
		{ID: "u-member", Username: "guest", Role: auth.Member, CreatedAt: now},
	}}
	ops := updateOperators{auth: auth.NewService(auth.Deps{Store: users, Clock: clockx.Fixed{At: now}})}

	cases := []struct {
		name string
		who  identity.Identity
		want bool
	}{
		{"a super account", identity.Identity{UserID: "u-super"}, true},
		{"a member", identity.Identity{UserID: "u-member"}, false},
		{"no account: authentication off, or a terminal", identity.Identity{}, true},
	}
	for _, c := range cases {
		got, err := ops.MayInstall(identity.With(context.Background(), c.who))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: MayInstall = %v, want %v", c.name, got, c.want)
		}
	}

	// Not a person: an agent on any token, and anything that arrives through
	// MCP. `aosd --mcp` carries no account and no agent, so an MCP client
	// looked like a terminal and was let through to install and restart.
	notPeople := []struct {
		name    string
		who     identity.Identity
		surface command.Surface
	}{
		{"an agent, even on a super account's token", identity.Identity{UserID: "u-super", AgentID: "atlas"}, command.SurfaceHTTP},
		{"an MCP client of aosd --mcp, with no account", identity.Identity{}, command.SurfaceMCP},
		{"an MCP client on a super account's token", identity.Identity{UserID: "u-super"}, command.SurfaceMCP},
		{"the agent's own tool registry", identity.Identity{}, command.SurfaceAgent},
	}
	for _, c := range notPeople {
		ctx := surfaced(identity.With(context.Background(), c.who), c.surface)
		got, err := ops.MayInstall(ctx)
		if got || !errors.Is(err, update.ErrNotAPerson) {
			t.Errorf("%s: MayInstall = %v, %v; want a refusal saying it is not a person", c.name, got, err)
		}
	}
	for _, s := range []command.Surface{command.SurfaceCLI, command.SurfaceHTTP} {
		if got, err := ops.MayInstall(surfaced(identity.With(context.Background(), identity.Identity{UserID: "u-super"}), s)); err != nil || !got {
			t.Errorf("a super account through %s: MayInstall = %v, %v", s, got, err)
		}
	}

	if _, err := ops.MayInstall(identity.With(context.Background(), identity.Identity{UserID: "u-gone"})); err == nil {
		t.Error("an account that does not exist should be an error, not a yes")
	}
	broken := updateOperators{auth: auth.NewService(auth.Deps{Store: brokenUsers{}, Clock: clockx.Fixed{At: now}})}
	if ok, err := broken.MayInstall(identity.With(context.Background(), identity.Identity{UserID: "u-super"})); err == nil || ok {
		t.Error("unreadable accounts must not grant anything")
	}
}

// A feed set on this machine and the feed a build carries fail differently
// when they are empty, so which one this is travels to the update service.
func TestUpdateFeedSaysWhetherItWasSetHere(t *testing.T) {
	saved := build.UpdateBaseURL
	t.Cleanup(func() { build.UpdateBaseURL = saved })
	build.UpdateBaseURL = "https://releases.example.test/latest/download"

	feed, custom := updateFeed(env.New(env.Map(map[string]string{})))
	if feed != build.UpdateBaseURL || custom {
		t.Fatalf("with nothing set: feed %q, custom %v", feed, custom)
	}
	feed, custom = updateFeed(env.New(env.Map(map[string]string{env.KeyUpdateBaseURL: " http://127.0.0.1:7498/feed "})))
	if feed != "http://127.0.0.1:7498/feed" || !custom {
		t.Fatalf("with %s set: feed %q, custom %v", env.KeyUpdateBaseURL, feed, custom)
	}
}

// surfaced is ctx as a handler invoked through surface sees it: SurfaceOf is
// set by Invoke only, so the call goes through a registered command.
func surfaced(ctx context.Context, surface command.Surface) context.Context {
	type in struct{ command.Reasoning }
	var seen context.Context
	reg := command.NewRegistry()
	command.MustRegister(reg, command.Command[in, struct{}]{
		Group: "probe", Name: "context", Summary: "Hand back the handler's context.",
		Handler: func(ctx context.Context, _ in) (struct{}, error) { seen = ctx; return struct{}{}, nil },
	})
	d, _, _ := reg.Lookup("probe_context")
	if _, err := d.Invoke(ctx, surface, json.RawMessage(`{"_reasoning":"probe"}`)); err != nil {
		panic(err)
	}
	return seen
}
