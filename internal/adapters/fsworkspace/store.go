// Package fsworkspace stores workspace records under the installation state
// directory, and lays out the managed skeleton inside the user's repository.
//
// Two ports, one package, because they are two halves of the same fact: a
// workspace is a record here and a directory there, and separating them would
// only mean two packages that must be changed together.
package fsworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/core/build"
	corecfg "github.com/OWNER/aos/internal/core/config"
	"github.com/OWNER/aos/internal/domain/workspace"
)

// recordName is the file inside ~/.aos/workspaces/{id}/ that holds the record.
// The name is the original's, so a person who has both installed finds the same
// file in the same place.
const recordName = "config.json"

// dirMode and fileMode keep the state directory closed. The workspace record is
// not a secret, but it lives beside files that are.
const (
	dirMode  os.FileMode = 0o700
	fileMode os.FileMode = 0o600
)

// Store is the filesystem implementation of workspace.Store.
type Store struct{ root string }

// NewStore builds a store over the directory that holds one directory per
// workspace.
func NewStore(root string) *Store { return &Store{root: filepath.Clean(root)} }

// FromPaths builds a store over the installation layout.
func FromPaths(p corecfg.Paths) *Store { return NewStore(p.Workspaces()) }

func (s *Store) fileFor(id string) string { return filepath.Join(s.root, id, recordName) }

// Get reads one workspace record.
func (s *Store) Get(ctx context.Context, id string) (*workspace.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(s.fileFor(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errRecordMissing(id)
	}
	if err != nil {
		return nil, errUnreadable(s.fileFor(id), err)
	}
	var w workspace.Workspace
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, errMalformed(s.fileFor(id), err)
	}
	// The directory name is the identity. A record whose id field drifted from
	// its location — hand-edited, or copied from another installation — is
	// read under the name it is filed as, because that is what every reference
	// to it resolves through.
	w.ID = id
	out := normalise(w)
	return &out, nil
}

// List reads every workspace record, skipping the ones that cannot be parsed.
//
// A single corrupt file must not make the whole installation unusable: the
// person still needs to reach their other workspaces in order to fix it.
func (s *Store) List(ctx context.Context) ([]workspace.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errUnreadable(s.root, err)
	}
	out := make([]workspace.Workspace, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		w, err := s.Get(ctx, e.Name())
		if err != nil {
			continue
		}
		out = append(out, *w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Save writes the record atomically, so a crash mid-write cannot leave a
// workspace registry that no longer parses.
func (s *Store) Save(ctx context.Context, w *workspace.Workspace) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Join(s.root, w.ID)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return errUnwritable(dir, err)
	}
	raw, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := atomicfs.WriteFile(s.fileFor(w.ID), raw, fileMode); err != nil {
		return errUnwritable(s.fileFor(w.ID), err)
	}
	return nil
}

// Delete removes the workspace's directory under the state root: the record and
// the derived data beside it, such as the search index.
//
// It does not touch the user's repository. That is the aggregate's rule, and it
// holds here too because this store only ever addresses paths under its root.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Join(s.root, id)
	if err := os.RemoveAll(dir); err != nil {
		return errUnwritable(dir, err)
	}
	return nil
}

// normalise fills the fields an older record predates, so that a workspace
// written by a previous build keeps working rather than coming back with a zero
// taxonomy and no Git policy.
func normalise(w workspace.Workspace) workspace.Workspace {
	if len(w.Tasks) == 0 {
		w.Tasks = append([]workspace.TaskType(nil), workspace.DefaultTaskTypes...)
	}
	if w.Labels == nil {
		w.Labels = []workspace.Label{}
	}
	if w.Git.BranchPrefix == "" {
		w.Git.BranchPrefix = workspace.DefaultGit().BranchPrefix
	}
	if w.Worktrees.WorktreeLimit == 0 {
		w.Worktrees = workspace.DefaultWorktrees()
	}
	if w.Color == "" {
		w.Color = workspace.DefaultColor
	}
	return w
}

func errRecordMissing(id string) error {
	return apperr.New("WORKSPACE_RECORD_MISSING").
		Causer("fsworkspace.Store.Get").
		Msgf("no workspace record for %q", id).
		Issue("workspace", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the registered workspaces",
			Command: build.Name + " workspace list",
			Tool:    "workspace_list",
		})
}

func errUnreadable(path string, cause error) error {
	return apperr.New("WORKSPACE_RECORD_UNREADABLE").
		Causer("fsworkspace.Store").
		Msgf("cannot read %q", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errMalformed(path string, cause error) error {
	return apperr.New("WORKSPACE_RECORD_MALFORMED").
		Causer("fsworkspace.Store.Get").
		Msgf("%q is not a valid workspace record", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnwritable(path string, cause error) error {
	return apperr.New("WORKSPACE_RECORD_UNWRITABLE").
		Causer("fsworkspace.Store").
		Msgf("cannot write %q", path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
