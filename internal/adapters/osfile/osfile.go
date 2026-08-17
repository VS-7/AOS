// Package osfile is the plain-disk implementation of the file domain's FS
// port.
package osfile

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/OWNER/aos/internal/core/pathx"
	"github.com/OWNER/aos/internal/domain/file"
)

// FS reads and writes the real filesystem, confining every path through
// pathx — the same containment the sandbox uses, so the two cannot drift
// apart.
type FS struct{}

// New builds an FS adapter.
func New() *FS { return &FS{} }

func (FS) Resolve(_ context.Context, root, p string) (string, error) {
	real, err := pathx.ResolveInside(root, p)
	if err != nil {
		if errors.Is(err, pathx.ErrOutside) {
			return "", file.ErrOutside
		}
		return "", err
	}
	return real, nil
}

func (FS) Stat(_ context.Context, path string) (file.Info, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return file.Info{}, wrapNotExist(err)
	}
	return file.Info{Name: fi.Name(), Dir: fi.IsDir(), Size: fi.Size(), ModTime: fi.ModTime()}, nil
}

func (FS) ReadDir(_ context.Context, path string) ([]file.Info, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, wrapNotExist(err)
	}
	out := make([]file.Info, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue // a file that disappeared mid-listing is skipped, not fatal
		}
		out = append(out, file.Info{Name: e.Name(), Dir: e.IsDir(), Size: info.Size(), ModTime: info.ModTime()})
	}
	return out, nil
}

// ReadFile reads at most limit+1 bytes so a single read call answers both
// "what's in the file" and "was there more than we took" without a second
// stat.
func (FS) ReadFile(_ context.Context, path string, limit int64) ([]byte, bool, error) {
	f, err := os.Open(path) //nolint:gosec // path was resolved and confined by pathx before this call
	if err != nil {
		return nil, false, wrapNotExist(err)
	}
	defer f.Close()

	buf := make([]byte, limit+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if int64(n) > limit {
		return buf[:limit], true, nil
	}
	return buf[:n], false, nil
}

func (FS) WriteFile(_ context.Context, path string, data []byte) error {
	return os.WriteFile(path, data, 0o644) //nolint:gosec // workspace files are not secrets
}

func (FS) MkdirAll(_ context.Context, path string) error {
	return os.MkdirAll(path, 0o755) //nolint:gosec // matches the permissions a person creating the directory by hand would get
}

func (FS) Rename(_ context.Context, from, to string) error {
	return os.Rename(from, to)
}

func (FS) Remove(_ context.Context, path string) error {
	return os.RemoveAll(path)
}

func wrapNotExist(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return file.ErrNotExist
	}
	return err
}
