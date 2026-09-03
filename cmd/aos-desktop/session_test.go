package main

import (
	"os"
	"path/filepath"
	"testing"

	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
)

// The application asked for a password on every launch, while `aos` in a
// terminal on the same machine, as the same person, needed none — because the
// CLI reads `~/.aos/local.token` and the window read nothing. There was
// nothing to log *into*: the account already existed and the credential was
// already on disk.
func TestTheWindowUsesTheCredentialTheInstallationAlreadyHolds(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "local.token"), []byte("  aos_written-at-onboarding\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolver := env.New(env.Map(map[string]string{env.KeyHome: home}))
	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		t.Fatal(err)
	}

	if got := localToken(resolver, paths); got != "aos_written-at-onboarding" {
		t.Errorf("token = %q, want the one on disk, trimmed", got)
	}
}

// AOS_TOKEN is for pointing this window at a daemon it did not start, and has
// to win over whatever this machine's own installation holds.
func TestAnExplicitTokenWins(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "local.token"), []byte("on-disk"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolver := env.New(env.Map(map[string]string{env.KeyHome: home, "TOKEN": "from-the-environment"}))
	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		t.Fatal(err)
	}

	if got := localToken(resolver, paths); got != "from-the-environment" {
		t.Errorf("token = %q, want the one the environment named", got)
	}
}

// A fresh installation has no credential yet, and the window opening signed
// out is the correct answer rather than a failure.
func TestNoCredentialYetIsNotAnError(t *testing.T) {
	resolver := env.New(env.Map(map[string]string{env.KeyHome: t.TempDir()}))
	paths, err := corecfg.Resolve(resolver)
	if err != nil {
		t.Fatal(err)
	}

	if got := localToken(resolver, paths); got != "" {
		t.Errorf("token = %q, want empty", got)
	}
}
