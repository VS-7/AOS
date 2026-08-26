package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/core/build"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// workspaceRootSource names where the root this process serves came from, so
// the daemon can say it in one line at boot instead of leaving whoever is
// debugging to infer it from behaviour.
type workspaceRootSource string

const (
	rootFromOption    workspaceRootSource = "option"
	rootFromEnv       workspaceRootSource = "environment"
	rootFromWorkingIn workspaceRootSource = "working directory"
	rootFromRegistry  workspaceRootSource = "registered workspace"
	rootFromDefault   workspaceRootSource = "default"
)

// resolveWorkspaceRoot decides which directory this process serves as its
// workspace, in the order a caller expects to win: what the embedder passed,
// what the environment names, the working directory, the workspace this
// installation already has registered, and finally a directory made for the
// purpose under the state directory.
//
// The working directory is what makes the CLI behave like git — run `aos tasks
// list` inside a project and it operates on that project — but it is a bad
// default for a process nobody started from a shell. A macOS application
// bundle opened from Finder, a launchd job and a Windows service are all
// started with a working directory that has nothing to do with the user's
// work: on macOS that is "/", the filesystem root.
//
// Serving "/" is not a degraded mode, it is a broken one. Every collection
// path resolved under it named a directory at the top of the disk that does
// not exist and cannot be created, so the daemon answered 403 or 500 to
// agents_list, tasks_list, goals_list, chats_list and workspace_introspect —
// the whole application, for the whole session, with the only symptom being
// AOS_COLLECTION_PATH_ESCAPES_ROOT lines in a log file nobody opens. So a
// working directory that cannot be a workspace is skipped rather than used,
// and the search moves on to what this installation already knows.
func resolveWorkspaceRoot(
	explicit, fromEnv string,
	paths corecfg.Paths,
	store workspace.Store,
) (root string, source workspaceRootSource) {
	if cleaned := cleanRoot(explicit); cleaned != "" {
		return cleaned, rootFromOption
	}
	if cleaned := cleanRoot(fromEnv); cleaned != "" {
		return cleaned, rootFromEnv
	}
	if cwd, err := os.Getwd(); err == nil {
		if cleaned := cleanRoot(cwd); cleaned != "" && corecfg.CanHoldWorkspace(cleaned) {
			return cleaned, rootFromWorkingIn
		}
	}
	if p := registeredWorkspacePath(store); p != "" {
		return p, rootFromRegistry
	}
	return filepath.Join(paths.Workspaces(), build.Name, "workspace"), rootFromDefault
}

func cleanRoot(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

// registeredWorkspacePath returns the path of a workspace this installation
// already registered, preferring one whose directory is still there. It is
// how a desktop application reopens what its user was working on instead of
// starting somewhere new every launch.
func registeredWorkspacePath(store workspace.Store) string {
	if store == nil {
		return ""
	}
	found, err := store.List(context.Background())
	if err != nil || len(found) == 0 {
		return ""
	}
	var fallback string
	for i := range found {
		p := cleanRoot(found[i].Path)
		if p == "" || found[i].Archived {
			continue
		}
		if fallback == "" {
			fallback = p
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return fallback
}
