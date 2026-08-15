package fsworkspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/OWNER/aos/internal/core/atomicfs"
)

// Files is the filesystem implementation of workspace.Scaffolder.
//
// Everything it writes lands in the user's own repository, so the permissions
// are the ordinary ones for source files: these are records a person edits and
// commits, not secrets.
type Files struct{}

// NewFiles builds the scaffolder.
func NewFiles() Files { return Files{} }

const (
	repoDirMode  os.FileMode = 0o755
	repoFileMode os.FileMode = 0o644
)

// EnsureDir creates a directory if it is missing and reports whether it had to.
//
// The report is what makes the scaffold observable: os.MkdirAll succeeds either
// way, so without the prior check a second run would be indistinguishable from
// the first, and "this repository was already a workspace" is exactly what the
// caller needs to know.
func (Files) EnsureDir(ctx context.Context, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	switch info, err := os.Stat(path); {
	case err == nil && info.IsDir():
		return false, nil
	case err == nil:
		return false, errNotADirectory(path)
	case !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	if err := os.MkdirAll(path, repoDirMode); err != nil {
		return false, err
	}
	return true, nil
}

// ReadFile returns the contents of a file, or "" when it does not exist. A
// missing .env is the normal case, not a failure.
func (Files) ReadFile(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// WriteFile replaces a file's contents atomically.
//
// Atomically because the file being replaced is often the user's .env: an
// interrupted write that truncated it would take their database URL with it.
func (Files) WriteFile(ctx context.Context, path, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), repoDirMode); err != nil {
		return err
	}
	return atomicfs.WriteFile(path, []byte(content), repoFileMode)
}

func errNotADirectory(path string) error {
	return &fs.PathError{Op: "mkdir", Path: path, Err: errors.New("exists and is not a directory")}
}
