package fsauth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/adapters/fsauth"
	"github.com/OWNER/aos/internal/domain/auth"
)

func ctx() context.Context { return context.Background() }

func TestAFreshInstallationHasNoAccounts(t *testing.T) {
	s := fsauth.New(filepath.Join(t.TempDir(), "users.json"))
	got, err := s.Load(ctx())
	if err != nil {
		t.Fatalf("a missing file is the state onboarding exists for: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("users = %+v", got)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	s := fsauth.New(path)
	want := []auth.User{{
		ID: "u-1", Name: "Vitor", Username: "vitor", Email: "v@example.test",
		PasswordHash: "$argon2id$...", Role: auth.Super,
		Tokens:    []auth.Token{{ID: "t-1", Name: "initial", Hash: "abc", Prefix: "aos_abcd"}},
		CreatedAt: time.Unix(0, 0).UTC(),
	}}
	if err := s.Save(ctx(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Username != "vitor" || len(got[0].Tokens) != 1 {
		t.Fatalf("users = %+v", got)
	}
}

// TestThePermissionIsRestrictive is defect #10: the original ships this file at
// 0644, and it holds every password hash and token hash on the machine.
func TestThePermissionIsRestrictive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	s := fsauth.New(path)
	if err := s.Save(ctx(), []auth.User{{ID: "u-1"}}); err != nil {
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

func TestAMalformedFileIsReportedRatherThanIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Silently treating this as "no accounts" would reopen onboarding on an
	// installation that already has an administrator.
	if _, err := fsauth.New(path).Load(ctx()); err == nil {
		t.Fatal("a corrupt account file must not read as an empty one")
	}
}

func TestSavingNothingWritesAnEmptyList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	s := fsauth.New(path)
	if err := s.Save(ctx(), nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]\n" {
		t.Fatalf("file = %q, want an empty list rather than null", raw)
	}
}

func TestACancelledContextIsRefused(t *testing.T) {
	s := fsauth.New(filepath.Join(t.TempDir(), "users.json"))
	cancelled, cancel := context.WithCancel(ctx())
	cancel()
	if _, err := s.Load(cancelled); err == nil {
		t.Error("Load ignored a cancelled context")
	}
	if err := s.Save(cancelled, nil); err == nil {
		t.Error("Save ignored a cancelled context")
	}
}
