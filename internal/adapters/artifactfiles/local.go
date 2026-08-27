// Package artifactfiles is the filesystem implementation of artifact.Files:
// it scaffolds an artifact's entrypoint on Create and removes its directory
// on Delete. Serving the files back out over HTTP is a transport concern —
// internal/transport/artifactapi — which calls Resolve below for the same
// containment Ensure uses, rather than a second implementation of it.
package artifactfiles

import (
	"context"
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/pathx"
)

// recordMode matches fscollections' own: an artifact's files are meant to be
// read by a browser and edited by the user, not secrets.
const recordMode os.FileMode = 0o644

// scaffoldHTML is written at the entrypoint when Ensure finds nothing there
// yet — enough to prove the artifact is reachable before anything real is
// written into it.
const scaffoldHTML = `<!doctype html>
<html>
<head><meta charset="utf-8"><title>New artifact</title></head>
<body><p>This artifact has no content yet.</p></body>
</html>
`

// Files places and removes one artifact's directory under the workspace
// root, the same root every fscollections.Repo in this process is bound to.
type Files struct{ root string }

// New builds the adapter over one workspace root.
//
// The root is resolved through symlinks once, here, which is what pathx.Root's
// own doc asks of every caller: Resolve returns a real path, and comparing one
// against a root that is still a symlink rejects everything underneath it.
// That was invisible while an artifact's own directory did not exist yet —
// Resolve gave up and handed back the uncollapsed path, which happened to
// compare equal — and became visible the moment Resolve started resolving
// through the nearest ancestor that does exist. Doing it at construction is
// both the documented shape and the cheap one: once per workspace, not once
// per file.
func New(root string) *Files {
	cleaned := filepath.Clean(root)
	if real, err := pathx.Root(cleaned); err == nil {
		cleaned = real
	}
	return &Files{root: cleaned}
}

// artifactRoot resolves the directory one artifact's files live under,
// through any symlink in the path. It does not require the directory to
// already exist — see pathx.Root's own doc.
func (f *Files) artifactRoot(id string) (string, error) {
	// The id is a path segment, and it is checked as one. `filepath.Join`
	// cleans, so an id of ".." resolved to the workspace's whole .aos
	// directory and Remove took it — no symlink involved, nothing for Root or
	// ResolveInside to catch, because the result was a perfectly ordinary
	// path that simply was not the artifact's.
	dir, err := pathx.JoinSegment(filepath.Join(f.root, collections.Root, "artifacts"), id)
	if err != nil {
		return "", err
	}
	return pathx.Root(dir)
}

// Ensure makes the artifact's directory and, when entrypoint does not
// already exist there, writes scaffoldHTML at that relative path. It returns
// the entrypoint actually in place.
func (f *Files) Ensure(_ context.Context, id, entrypoint string) (string, error) {
	root, err := f.artifactRoot(id)
	if err != nil {
		return "", errResolveFailed(id, err)
	}
	target, err := pathx.ResolveInside(root, entrypoint)
	if err != nil {
		return "", errPathOutside(id, entrypoint, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", errWriteFailed(id, entrypoint, err)
	}
	if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
		if err := os.WriteFile(target, []byte(scaffoldHTML), recordMode); err != nil {
			return "", errWriteFailed(id, entrypoint, err)
		}
	}
	return entrypoint, nil
}

// Resolve confines path inside artifact id's own directory, exactly the way
// Ensure confines entrypoint — the same helper, so the transport that serves
// an artifact's files back out over HTTP (internal/transport/artifactapi)
// cannot drift from the containment Ensure already enforces at write time.
func (f *Files) Resolve(id, path string) (string, error) {
	root, err := f.artifactRoot(id)
	if err != nil {
		return "", errResolveFailed(id, err)
	}
	target, err := pathx.ResolveInside(root, path)
	if err != nil {
		return "", errPathOutside(id, path, err)
	}
	return target, nil
}

// Remove deletes the artifact's directory and everything under it. It is
// idempotent: removing what is already gone succeeds.
func (f *Files) Remove(_ context.Context, id string) error {
	root, err := f.artifactRoot(id)
	if err != nil {
		return errResolveFailed(id, err)
	}
	if err := os.RemoveAll(root); err != nil {
		return errRemoveFailed(id, err)
	}
	return nil
}

func errResolveFailed(id string, cause error) error {
	return apperr.New("ARTIFACTFILES_RESOLVE_FAILED").
		Causer("artifactfiles.Files").
		Msgf("could not resolve the directory of artifact %q: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace root is readable"})
}

func errPathOutside(id, path string, cause error) error {
	return apperr.New("ARTIFACTFILES_PATH_OUTSIDE").
		Causer("artifactfiles.Files").
		Msgf("%q resolves outside artifact %q's own directory", path, id).
		Issue("id", id).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the entrypoint must live inside the artifact's own directory"})
}

func errWriteFailed(id, path string, cause error) error {
	return apperr.New("ARTIFACTFILES_WRITE_FAILED").
		Causer("artifactfiles.Files.Ensure").
		Msgf("writing %q for artifact %q failed: %v", path, id, cause).
		Issue("id", id).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace is writable and has space"})
}

func errRemoveFailed(id string, cause error) error {
	return apperr.New("ARTIFACTFILES_REMOVE_FAILED").
		Causer("artifactfiles.Files.Remove").
		Msgf("removing artifact %q failed: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the workspace is writable"})
}
