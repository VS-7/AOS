package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
)

func TestResolveUsesHomeOverride(t *testing.T) {
	dir := t.TempDir()
	p, err := config.Resolve(env.New(env.Map(map[string]string{env.KeyHome: dir})))
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != filepath.Clean(dir) {
		t.Fatalf("root = %q, want %q", p.Root, dir)
	}
	if got, want := p.Config(), filepath.Join(dir, "config.json"); got != want {
		t.Errorf("config path = %q, want %q", got, want)
	}
	if got, want := p.JobsDB(), filepath.Join(dir, "data", "jobs.sqlite"); got != want {
		t.Errorf("jobs db = %q, want %q", got, want)
	}
	if got, want := p.GatewayLock(), filepath.Join(dir, "runtime", "gateway", "gateway.lock"); got != want {
		t.Errorf("gateway lock = %q, want %q", got, want)
	}
}

func TestResolveDefaultsToBrandedStateDir(t *testing.T) {
	p, err := config.Resolve(env.New())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p.Root) != build.StateDir {
		t.Fatalf("root = %q, want a directory named %q", p.Root, build.StateDir)
	}
}

// TestResolveRefusesForeignStateDirectory guards the installation of the
// original product, which is a live reference on the development machine.
func TestResolveRefusesForeignStateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".fractal")
	_, err := config.Resolve(env.New(env.Map(map[string]string{env.KeyHome: dir})))
	if err == nil {
		t.Fatal("expected a refusal for a .fractal root")
	}
	e, ok := apperr.As(err)
	if !ok || e.Code != build.ErrorPrefix+"_STATE_FOREIGN_ROOT" {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Error("refusal should unwrap to ErrInvalid")
	}
}

func TestEnsureCreatesTheSkeleton(t *testing.T) {
	dir := t.TempDir()
	p, err := config.Resolve(env.New(env.Map(map[string]string{env.KeyHome: dir})))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{p.Data(), p.GatewayDir(), p.Outputs(), p.Workspaces()} {
		info, err := os.Stat(d)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", d)
		}
	}
}

func TestWriteSecretUses0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes are advisory on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.WriteSecret(path, []byte(`{"security":{}}`)); err != nil {
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

// TestAuditSecretsRepairsLoosePermissions reproduces what is on the machine
// running the original: config.json and users.json at 0644, readable by every
// process of the user.
func TestAuditSecretsRepairsLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes are advisory on Windows")
	}
	dir := t.TempDir()
	loose := filepath.Join(dir, "config.json")
	if err := os.WriteFile(loose, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(dir, "users.json")

	repairs := config.AuditSecrets(loose, absent)
	if len(repairs) != 1 {
		t.Fatalf("expected exactly one repair, got %d: %+v", len(repairs), repairs)
	}
	r := repairs[0]
	if r.Was != 0o644 || r.Now != 0o600 || r.Err != "" || !r.Enforced {
		t.Fatalf("unexpected repair: %+v", r)
	}
	info, err := os.Stat(loose)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode after repair = %04o", perm)
	}
	// A second audit finds nothing left to do.
	if again := config.AuditSecrets(loose); len(again) != 0 {
		t.Fatalf("second audit reported %+v", again)
	}
}

func TestSecretFilesAreTheOnesAudited(t *testing.T) {
	p, err := config.Resolve(env.New(env.Map(map[string]string{env.KeyHome: t.TempDir()})))
	if err != nil {
		t.Fatal(err)
	}
	files := p.SecretFiles()
	want := []string{p.Config(), p.Users(), p.LocalToken()}
	if len(files) != len(want) {
		t.Fatalf("expected %v, got %v", want, files)
	}
	for i, f := range files {
		if f != want[i] {
			t.Errorf("expected %q at position %d, got %q", want[i], i, f)
		}
	}
}

func TestEnsureFailsWhenTheRootIsAFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "state")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := config.Resolve(env.New(env.Map(map[string]string{env.KeyHome: blocker})))
	if err != nil {
		t.Fatal(err)
	}
	err = p.Ensure()
	if err == nil {
		t.Fatal("expected Ensure to fail")
	}
	e, ok := apperr.As(err)
	if !ok || e.Code != build.ErrorPrefix+"_STATE_DIR_UNWRITABLE" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteSecretFailsOnAnUnwritablePath(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := config.WriteSecret(filepath.Join(blocker, "config.json"), []byte("{}"))
	if err == nil {
		t.Fatal("expected a write failure")
	}
	e, ok := apperr.As(err)
	if !ok || e.Code != build.ErrorPrefix+"_SECRET_WRITE_FAILED" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteSecretTightensAPreexistingLooseFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes are advisory on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteSecret(path, []byte(`{"a":1}`)); err != nil {
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

func TestAuditSecretsReportsAStatFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(locked, "config.json")
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skip("cannot drop directory permissions on this filesystem")
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	repairs := config.AuditSecrets(target)
	if len(repairs) != 1 || repairs[0].Err == "" {
		t.Fatalf("expected a reported stat failure, got %+v", repairs)
	}
}

// TestAFilesystemRootCannotHoldAWorkspace is the defect this predicate exists
// for.
//
// A process nobody started from a shell inherits a working directory that has
// nothing to do with the person using it: an application bundle opened from
// Finder gets "/". Taking that as the workspace made every workspace-relative
// path resolve at the top of the disk, which the collection repositories then
// refused as escaping the root — every workspace-scoped command in the
// application answered 403 or 500 for the whole session, with nothing on
// screen saying why.
func TestAFilesystemRootCannotHoldAWorkspace(t *testing.T) {
	root := "/"
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(os.Getenv("SystemDrive")) + string(filepath.Separator)
		if root == string(filepath.Separator) {
			root = `C:\`
		}
	}
	if config.CanHoldWorkspace(root) {
		t.Errorf("%q was accepted as a workspace root", root)
	}
}

// TestAnEmptyDirectoryNameIsNotAWorkspace. filepath.Clean("") is ".", the
// process's own working directory — so an unset value that reached this
// unchecked would be answered for a directory nobody named.
func TestAnEmptyDirectoryNameIsNotAWorkspace(t *testing.T) {
	for _, dir := range []string{"", "   ", "\t"} {
		if config.CanHoldWorkspace(dir) {
			t.Errorf("%q was accepted as a workspace root", dir)
		}
	}
}

// TestTheBareHomeDirectoryIsRefusedUntilItHasBeenUsedBefore.
//
// Laying a workspace skeleton into someone's home directory because of a
// working directory they did not choose is not a change to make to a machine
// uninvited. A home that already holds the state directory was meant.
func TestTheBareHomeDirectoryIsRefusedUntilItHasBeenUsedBefore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.UserHomeDir reads USERPROFILE on Windows; HOME does not move it")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	if config.CanHoldWorkspace(home) {
		t.Error("a bare home directory was accepted as a workspace root")
	}

	if err := os.MkdirAll(filepath.Join(home, build.StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if !config.CanHoldWorkspace(home) {
		t.Errorf("a home directory already holding %s was refused", build.StateDir)
	}
}

// TestAnOrdinaryDirectoryCanHoldAWorkspace — the answer that has to stay yes,
// or the check would have replaced one broken default with another.
func TestAnOrdinaryDirectoryCanHoldAWorkspace(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Setenv("HOME", t.TempDir())
	}
	dir := t.TempDir()
	if !config.CanHoldWorkspace(dir) {
		t.Errorf("%q was refused as a workspace root", dir)
	}
	// A directory under the home directory is not the home directory.
	nested := filepath.Join(dir, "projects", "thing")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if !config.CanHoldWorkspace(nested) {
		t.Errorf("%q was refused as a workspace root", nested)
	}
}
