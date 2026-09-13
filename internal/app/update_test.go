package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/releasesource"
	"github.com/OWNER/aos/internal/adapters/supervise"
	"github.com/OWNER/aos/internal/adapters/updateinstall"
	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/clockx"
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
	return gateway.NewService(gateway.Deps{
		Processes: supervise.NewProcesses(),
		Health:    supervise.NewHealth(),
		Store:     supervise.NewStore(filepath.Join(dir, "gateway.json")),
		Locker:    supervise.NewLock(filepath.Join(dir, "gateway.lock")),
		Resolver:  supervise.Resolver{Explicit: filepath.Join(dir, "no-such-aosd"), Args: []string{"serve"}},
		Clock:     clockx.System{},
		Sleeper:   supervise.Sleeper{},
		Host:      "127.0.0.1",
		Port:      1,
		Inside:    inside,
	})
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
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	aosd := filepath.Join(binDir, "aosd")
	if err := os.WriteFile(aosd, []byte("the running daemon"), 0o755); err != nil {
		t.Fatal(err)
	}

	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	const platform = "linux/amd64"
	asset := []byte("aosd v0.10.0")
	sum := sha256.Sum256(asset)
	checksums := hex.EncodeToString(sum[:]) + "  aosd_v0.10.0_linux_amd64\n"
	sig, err := relsig.Sign(priv, []byte(checksums))
	if err != nil {
		t.Fatal(err)
	}
	feed := http.NewServeMux()
	var base string
	feed.HandleFunc("/stable.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(update.Release{
			Version: "v0.10.0", ChecksumsURL: base + "/checksums.txt", SignatureURL: base + "/checksums.txt.sig",
			Assets: []update.Asset{{Binary: "aosd", Platform: platform, URL: base + "/aosd", Filename: "aosd_v0.10.0_linux_amd64"}},
		})
	})
	feed.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(checksums)) })
	feed.HandleFunc("/checksums.txt.sig", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(sig)) })
	feed.HandleFunc("/aosd", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(asset) })
	srv := httptest.NewServer(feed)
	t.Cleanup(srv.Close)
	base = srv.URL

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
		Platform:   platform,
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

func TestUpdateSupervisorRestartsFromOutsideOnlyOutsideTheDaemon(t *testing.T) {
	dir := t.TempDir()
	serving := false
	sup := updateSupervisor{
		inside:  realGateway(t, dir, true),
		outside: realGateway(t, dir, false),
		serving: func() bool { return serving },
	}
	ctx := context.Background()
	if !sup.CanRestart(ctx) {
		t.Fatal("a process that is not the daemon can restart it")
	}
	// Nothing is running and the daemon binary does not exist: the outside
	// gateway tries, and fails to start it — which is not a self-restart
	// refusal.
	if err := sup.Restart(ctx); err == nil {
		t.Fatal("starting a daemon that does not exist should fail")
	} else if e, ok := apperr.As(err); ok && e.Code == "AOS_GATEWAY_SELF_RESTART" {
		t.Fatal("outside the daemon the restart must go through the outside gateway")
	}
	if sup.Healthy(ctx) {
		t.Fatal("nothing is running, so nothing is healthy")
	}

	serving = true
	if sup.CanRestart(ctx) {
		t.Fatal("the daemon cannot restart itself")
	}
	if (updateSupervisor{inside: sup.inside, serving: func() bool { return false }}).CanRestart(ctx) {
		t.Fatal("with no outside gateway there is nothing to restart with")
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
		{"an agent, even on a super account's token", identity.Identity{UserID: "u-super", AgentID: "atlas"}, false},
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
