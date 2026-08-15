// Package atomicfs writes files atomically.
//
// ADR-0012 places WriteFileAtomic in internal/core/collections. It lives here
// instead because the secret writer of ADR-0010 needs the same guarantee in
// phase 0, before the collections engine exists, and two implementations of an
// atomic write is exactly the kind of duplication that ends with one of them
// being wrong. The collections engine consumes this package.
package atomicfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// tempFile is the slice of *os.File this package uses. It exists so that the
// failure paths — a short write, a failing fsync, a failing rename — can be
// exercised by a test, which is the only way to prove the promise of the
// package: an interrupted write leaves the previous file intact.
type tempFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

var (
	createTemp = func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) }
	rename     = os.Rename
)

// fsyncEnabled allows tests to skip the two fsyncs per write (AOS_FSYNC=off).
// It is process-wide on purpose: it is a machine-level trade-off, not a
// per-call one.
var fsyncEnabled atomic.Bool

func init() { fsyncEnabled.Store(true) }

// SetFsync enables or disables fsync on write. Production never calls it with
// false; the test harness does, from AOS_FSYNC.
func SetFsync(on bool) { fsyncEnabled.Store(on) }

// WriteFile writes data to path atomically: it creates a temp file in the same
// directory (same filesystem, so rename is atomic), fsyncs it, renames over the
// target, then fsyncs the directory so the rename itself is durable.
//
// A crash mid-write therefore leaves the previous file intact rather than a
// truncated one with invalid YAML front matter.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := createTemp(dir, tempPattern(path))
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }() // no-op once the rename succeeds

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if fsyncEnabled.Load() {
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := rename(tmp, path); err != nil {
		return err
	}
	return fsyncDir(dir)
}

// tempPattern keeps the temp name recognisable in a directory listing and out
// of the way of the collection scanner, which ignores dotfiles.
func tempPattern(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if len(base) > 24 {
		base = base[:24]
	}
	return ".tmp-" + base + "-*"
}

func fsyncDir(dir string) error {
	if !fsyncEnabled.Load() {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		// Directory fsync is not supported on every filesystem (notably some
		// Windows and network mounts). The rename already happened; refusing
		// the write here would be worse than accepting weaker durability.
		return nil //nolint:nilerr // documented degradation
	}
	return nil
}
