package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/core/env"
	"github.com/OWNER/aos/internal/domain/workspace"
)

func testPaths(t *testing.T) corecfg.Paths {
	t.Helper()
	p, err := corecfg.Resolve(env.New(env.Map(map[string]string{env.KeyHome: t.TempDir()})))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// stubStore is the registry, without a filesystem: these tests are about which
// candidate wins, not about how a workspace is stored.
type stubStore struct{ workspaces []workspace.Workspace }

func (s stubStore) List(context.Context) ([]workspace.Workspace, error) { return s.workspaces, nil }
func (s stubStore) Get(context.Context, string) (*workspace.Workspace, error) {
	return nil, os.ErrNotExist
}
func (s stubStore) Save(context.Context, *workspace.Workspace) error { return nil }
func (s stubStore) Delete(context.Context, string) error             { return nil }

func TestTheExplicitRootWinsOverEverything(t *testing.T) {
	dir := t.TempDir()
	root, source := resolveWorkspaceRoot(dir, filepath.Join(dir, "other"), testPaths(t), stubStore{})
	if root != dir || source != rootFromOption {
		t.Fatalf("root = %q from %q, want %q from the option", root, source, dir)
	}
}

func TestTheEnvironmentWinsOverTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	root, source := resolveWorkspaceRoot("", dir, testPaths(t), stubStore{})
	if root != dir || source != rootFromEnv {
		t.Fatalf("root = %q from %q, want %q from the environment", root, source, dir)
	}
}

// TestTheWorkingDirectoryIsUsedWhenItCanBeAWorkspace is the CLI's behaviour:
// run a command inside a project and it operates on that project.
func TestTheWorkingDirectoryIsUsedWhenItCanBeAWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	root, source := resolveWorkspaceRoot("", "", testPaths(t), stubStore{})
	// macOS puts t.TempDir() under /var, a symlink to /private/var, and the
	// working directory comes back resolved. Comparing the base is enough to
	// say the working directory is what was chosen.
	if filepath.Base(root) != filepath.Base(dir) || source != rootFromWorkingIn {
		t.Fatalf("root = %q from %q, want the working directory %q", root, source, dir)
	}
}

// TestAFilesystemRootIsNeverTheWorkspace is the defect this file exists for.
//
// A macOS application bundle launched from Finder is started with "/" as its
// working directory, and the daemon the desktop spawns inherits it. Taking it
// meant every collection path resolved under the top of the disk, and the
// whole application answered 403 or 500 for the session.
func TestAFilesystemRootIsNeverTheWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`"/" is not a filesystem root on Windows`)
	}
	t.Chdir("/")

	paths := testPaths(t)
	root, source := resolveWorkspaceRoot("", "", paths, stubStore{})
	if root == "/" {
		t.Fatal(`the filesystem root was accepted as a workspace`)
	}
	if source == rootFromWorkingIn {
		t.Fatalf("root = %q was taken from the working directory anyway", root)
	}
	if want := filepath.Join(paths.Workspaces(), "aos", "workspace"); root != want {
		t.Fatalf("root = %q, want the default %q", root, want)
	}
}

// TestAnAlreadyRegisteredWorkspaceIsReopened: an installation that has been
// used before knows where the user works, and a launcher that provides no
// working directory should land there rather than somewhere new.
func TestAnAlreadyRegisteredWorkspaceIsReopened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`"/" is not a filesystem root on Windows`)
	}
	t.Chdir("/")

	existing := t.TempDir()
	store := stubStore{workspaces: []workspace.Workspace{{ID: "one", Path: existing}}}

	root, source := resolveWorkspaceRoot("", "", testPaths(t), store)
	if filepath.Base(root) != filepath.Base(existing) || source != rootFromRegistry {
		t.Fatalf("root = %q from %q, want the registered %q", root, source, existing)
	}
}

// TestAnArchivedWorkspaceIsNotReopened — archiving is how somebody says they
// are done with a workspace, and reopening it would undo that with no way to
// see why.
func TestAnArchivedWorkspaceIsNotReopened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`"/" is not a filesystem root on Windows`)
	}
	t.Chdir("/")

	paths := testPaths(t)
	store := stubStore{workspaces: []workspace.Workspace{
		{ID: "old", Path: t.TempDir(), Archived: true},
	}}

	root, source := resolveWorkspaceRoot("", "", paths, store)
	if source != rootFromDefault {
		t.Fatalf("root = %q from %q, want the default", root, source)
	}
}

// TestARegisteredWorkspaceThatIsStillOnDiskIsPreferred: a directory somebody
// deleted is a worse answer than one that is still there, even if it was
// registered first.
func TestARegisteredWorkspaceThatIsStillOnDiskIsPreferred(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`"/" is not a filesystem root on Windows`)
	}
	t.Chdir("/")

	present := t.TempDir()
	store := stubStore{workspaces: []workspace.Workspace{
		{ID: "gone", Path: filepath.Join(t.TempDir(), "deleted-by-hand")},
		{ID: "here", Path: present},
	}}

	root, _ := resolveWorkspaceRoot("", "", testPaths(t), store)
	if filepath.Base(root) != filepath.Base(present) {
		t.Fatalf("root = %q, want the one that still exists, %q", root, present)
	}
}

func TestTheHomeDirectoryIsNotScaffoldedIntoByAccident(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory on this machine")
	}
	if corecfg.CanHoldWorkspace(filepath.Clean(home)) {
		// It is allowed when the layout is already there — somebody meant it.
		if _, err := os.Stat(filepath.Join(home, ".aos")); err == nil {
			t.Skip("this machine's home directory is already a workspace")
		}
		t.Fatal("the home directory was accepted as a workspace to scaffold into")
	}
}
