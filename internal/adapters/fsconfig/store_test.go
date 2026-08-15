package fsconfig_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fsconfig"
	"github.com/OWNER/aos/internal/domain/config"
)

func TestLoadOnAFreshInstallationReturnsDefaults(t *testing.T) {
	store := fsconfig.New(filepath.Join(t.TempDir(), "config.json"))
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Security.Enabled || got.Telemetry.Enabled {
		t.Fatalf("fresh installation did not get the safe defaults: %+v", got)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := fsconfig.New(path)
	ctx := context.Background()

	want := config.Default()
	want.User.Name = "Vitor"
	want.Region.Timezone = "America/Sao_Paulo"
	want.Agents.Providers = []config.Provider{{ID: "openai", Key: "sk-secret"}}

	if err := store.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.User.Name != "Vitor" || got.Region.Timezone != "America/Sao_Paulo" {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if len(got.Agents.Providers) != 1 || got.Agents.Providers[0].Key != "sk-secret" {
		t.Fatalf("providers = %+v", got.Agents.Providers)
	}
}

func TestSaveWrites0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes are advisory on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := fsconfig.New(path).Save(context.Background(), config.Default()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %04o, want 0600", perm)
	}
}

// TestHandEditIsPickedUpWithoutRestart reproduces the behaviour of the
// original's stateless config service: the file is the source of truth and may
// be edited with the daemon running.
func TestHandEditIsPickedUpWithoutRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := fsconfig.New(path)
	ctx := context.Background()

	if err := store.Save(ctx, config.Default()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx); err != nil {
		t.Fatal(err)
	}

	// Some filesystems have coarse modification-time resolution; the size
	// check in the store covers that, and this edit changes both.
	edited := []byte(`{"user":{"name":"Edited By Hand"},"region":{"timezone":"UTC"}}`)
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	got, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.User.Name != "Edited By Hand" {
		t.Fatalf("hand edit was not picked up: %+v", got.User)
	}
	// Fields the hand edit omitted keep their defaults rather than zeroing.
	if !got.Security.Enabled {
		t.Error("an omitted field must fall back to the default, not to the zero value")
	}
}

func TestLoadIsCachedBetweenIdenticalReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := fsconfig.New(path)
	ctx := context.Background()
	if err := store.Save(ctx, config.Default()); err != nil {
		t.Fatal(err)
	}
	first, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skip("cannot make the file unreadable on this filesystem")
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	second, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("a cached read should not touch the file: %v", err)
	}
	if first.Region.Timezone != second.Region.Timezone {
		t.Fatal("cached value differs from the first read")
	}
}

func TestCancelledContextIsRespected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := fsconfig.New(filepath.Join(t.TempDir(), "config.json"))
	if _, err := store.Load(ctx); err == nil {
		t.Error("Load ignored a cancelled context")
	}
	if err := store.Save(ctx, config.Default()); err == nil {
		t.Error("Save ignored a cancelled context")
	}
}
