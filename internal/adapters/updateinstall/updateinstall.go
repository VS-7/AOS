// Package updateinstall implements update.Stager, update.Installer,
// update.Store and update.Lock over the local filesystem: writing staged
// binaries, swapping them into place with a backup that makes Rollback exact,
// keeping the record of what was checked and staged, and one update at a
// time across the processes that share it.
package updateinstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/domain/update"
)

// backupSuffix marks the binary SwapIn displaced, so Rollback knows exactly
// what to restore without any state kept in memory between the two calls —
// update.Service itself may retry Apply from a fresh process. incomingSuffix
// is the copy SwapIn writes beside the target before renaming it over.
const (
	backupSuffix   = ".prev"
	incomingSuffix = ".new"
)

// Installer is update.Stager and update.Installer over one machine's own
// filesystem.
//
// Every method takes a binary name and resolves the path itself, and refuses
// any name update.IsBinary does not know. Paths used to arrive from the
// caller — the request body of update_apply — and "../" in a binary name put
// a file outside BinDir.
type Installer struct {
	// StageDir is where Stage writes verified binaries before Apply swaps
	// them in — a directory of its own under config.Paths.UpdateDir().
	StageDir string
	// BinDir is where the live binaries this installation runs are
	// resolved from — Target joins BinDir and the binary's own filename.
	BinDir string
}

// New builds an Installer over stageDir and binDir.
func New(stageDir, binDir string) *Installer {
	return &Installer{StageDir: stageDir, BinDir: binDir}
}

var (
	_ update.Stager    = (*Installer)(nil)
	_ update.Installer = (*Installer)(nil)
)

// Stage writes data as binary's staged copy.
func (i *Installer) Stage(_ context.Context, binary string, data []byte) (string, error) {
	path, err := i.staged(binary)
	if err != nil {
		return "", err
	}
	if err := atomicfs.WriteFile(path, data, 0o755); err != nil {
		return "", errStageFailed(binary, err)
	}
	return path, nil
}

// Digest reads binary's staged copy now and returns its hex SHA-256.
func (i *Installer) Digest(_ context.Context, binary string) (string, error) {
	path, err := i.staged(binary)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errStageFailed(binary, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", errStageFailed(binary, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Discard removes every binary's staged copy. Only the names this package
// stages are removed, never the directory: StageDir is configured, and a
// RemoveAll on a misconfigured one is not a mistake worth making possible.
func (i *Installer) Discard(context.Context) error {
	var failed []error
	for _, binary := range update.Binaries() {
		path, err := i.staged(binary)
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failed = append(failed, err)
		}
	}
	if err := errors.Join(failed...); err != nil {
		return errStageFailed("the staged release", err)
	}
	return nil
}

// Target resolves binary's live path, and whether a regular file is there.
func (i *Installer) Target(_ context.Context, binary string) (string, bool, error) {
	path, err := i.live(binary)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return path, false, nil
	case err != nil:
		return "", false, errSwapFailed(path, err)
	}
	return path, info.Mode().IsRegular(), nil
}

// SwapIn puts binary's staged copy in place.
//
// The staged copy is copied next to the target first and renamed over it, so
// the target is always either the old binary or the whole new one, and the
// staged copy stays until Discard — a rolled-back update used to have lost it
// to the rename, and could not be retried without downloading it again.
//
// Only a binary that is already installed is replaced. Creating one that was
// not there — an aos inside a macOS bundle that never carried one — changes
// the installation's layout, which is the installer's decision, not an
// update's.
//
// The copy is hashed as it is written, and it replaces the target only when
// it hashes to digest, the value Apply verified the staged copy against.
// Apply verifies and then waits for work in flight before it swaps; without
// this, a staged file changed during that wait was put in place unchecked.
func (i *Installer) SwapIn(ctx context.Context, binary, digest string) error {
	staged, err := i.staged(binary)
	if err != nil {
		return err
	}
	if digest == "" {
		return errSwapFailed(staged, errors.New("no verified digest to check the staged copy against"))
	}
	target, installed, err := i.Target(ctx, binary)
	if err != nil {
		return err
	}
	if !installed {
		return errSwapFailed(target, errors.New("it is not installed here, and an update does not add binaries"))
	}

	incoming := target + incomingSuffix
	got, err := copyFile(staged, incoming)
	if err != nil {
		_ = os.Remove(incoming)
		return errSwapFailed(target, err)
	}
	if !strings.EqualFold(got, digest) {
		_ = os.Remove(incoming)
		return errSwapFailed(target, fmt.Errorf("%w: %s hashes to %s, not the verified %s", update.ErrStagedChanged, staged, got, digest))
	}
	backup := target + backupSuffix
	if err := renameOver(target, backup); err != nil {
		_ = os.Remove(incoming)
		return errSwapFailed(target, err)
	}
	if err := renameOver(incoming, target); err != nil {
		_ = renameOver(backup, target)
		_ = os.Remove(incoming)
		return errSwapFailed(target, err)
	}
	return nil
}

// Rollback restores binary from the backup SwapIn made. A binary with no
// backup — Rollback called without a prior SwapIn for it — is a no-op, not
// an error: update.Service's own Apply rolls back every binary it may have
// swapped, whether or not this particular one got that far before the
// failure that triggered the rollback.
func (i *Installer) Rollback(_ context.Context, binary string) error {
	target, err := i.live(binary)
	if err != nil {
		return err
	}
	backup := target + backupSuffix
	if _, err := os.Stat(backup); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := renameOver(backup, target); err != nil {
		return errRollbackFailed(target, err)
	}
	return nil
}

// Commit removes the backup SwapIn kept, once the new binary is proven.
func (i *Installer) Commit(_ context.Context, binary string) error {
	target, err := i.live(binary)
	if err != nil {
		return err
	}
	if err := os.Remove(target + backupSuffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errSwapFailed(target, err)
	}
	return nil
}

// Tidy removes the backups a Commit could not remove and the copies a SwapIn
// that was killed before its rename left, for the binaries an update manages.
//
// On Windows a running program's file can be renamed but not removed, and
// `aosd.exe update apply` runs from the very aosd.exe its SwapIn renames to
// aosd.exe.prev — so its own Commit cannot remove that backup, nor the
// window's while the window is open. Once those processes have ended, this
// can. update.Service calls it only when no install is under way.
func (i *Installer) Tidy(context.Context) error {
	var failed []error
	for _, binary := range update.Binaries() {
		target, err := i.live(binary)
		if err != nil {
			continue
		}
		for _, leftover := range []string{target + backupSuffix, target + incomingSuffix} {
			if err := os.Remove(leftover); err != nil && !errors.Is(err, fs.ErrNotExist) {
				failed = append(failed, err)
			}
		}
	}
	if err := errors.Join(failed...); err != nil {
		return errSwapFailed(i.BinDir, err)
	}
	return nil
}

// Reinstall says why the binaries in BinDir cannot be replaced one at a time.
//
// Inside a macOS application bundle: codesign seals every file of a bundle,
// so replacing Contents/MacOS/aosd — or leaving an aosd.prev beside it —
// makes `codesign --verify` fail for the whole application, the very check
// install.sh refuses a download over.
//
// In a directory this account cannot write: an AppImage's binaries live on a
// read-only mount, and an install for every account (Program Files, /usr)
// needs an administrator. SwapIn renames and creates files in BinDir, so
// such an installation used to be offered a terminal command that could only
// fail at the swap. Writing is asked of the directory itself, by creating a
// file and removing it: permission bits, ACLs, a read-only mount and a
// process without elevation all answer that one question the same way.
func (i *Installer) Reinstall(context.Context) update.ReinstallReason {
	if inAppBundle(i.BinDir) {
		return update.ReinstallBundle
	}
	if !writable(i.BinDir) {
		return update.ReinstallReadOnly
	}
	return ""
}

// writable reports whether this process can create a file in dir.
func writable(dir string) bool {
	probe, err := os.CreateTemp(dir, ".aos-update-probe-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	return os.Remove(name) == nil
}

// inAppBundle reports whether dir is inside "<name>.app/Contents".
func inAppBundle(dir string) bool {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/")
	for n := 0; n+1 < len(parts); n++ {
		if strings.HasSuffix(parts[n], ".app") && parts[n+1] == "Contents" {
			return true
		}
	}
	return false
}

func (i *Installer) staged(binary string) (string, error) {
	name, err := filename(binary)
	if err != nil {
		return "", err
	}
	return filepath.Join(i.StageDir, name), nil
}

func (i *Installer) live(binary string) (string, error) {
	name, err := filename(binary)
	if err != nil {
		return "", err
	}
	return filepath.Join(i.BinDir, name), nil
}

// filename is binary's file name on this platform, for the three names an
// update manages and no other. The domain refuses other names first; this
// refuses them again, because a filesystem adapter that joins whatever it is
// handed is one caller away from writing anywhere.
func filename(binary string) (string, error) {
	if !update.IsBinary(binary) {
		return "", errUnknownBinary(binary)
	}
	if runtime.GOOS == "windows" {
		return binary + ".exe", nil
	}
	return binary, nil
}

// copyFile copies src to dst and returns the hex SHA-256 of the bytes it
// wrote — the bytes dst now holds, not whatever src holds by the time anybody
// asks again.
func copyFile(src, dst string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), in); err != nil {
		_ = out.Close()
		return "", err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// renameOver renames src to dst, removing dst first: os.Rename overwrites
// silently on POSIX but refuses when dst already exists on Windows, and a
// stale dst here is always meant to be replaced (a leftover ".prev" from an
// earlier update, or the target SwapIn is about to occupy). Both paths are
// in BinDir, so the rename never crosses a filesystem.
func renameOver(src, dst string) error {
	if runtime.GOOS == "windows" {
		_ = os.Remove(dst)
	}
	return os.Rename(src, dst)
}

// Store is update.Store over one JSON file.
type Store struct {
	Path string
}

// recordLockWait bounds how long Update waits for another writer. A writer
// holds the record for one read and one write, so anything near this is a
// process that is stuck, not busy.
const recordLockWait = 10 * time.Second

// NewStore builds a Store over path.
func NewStore(path string) *Store { return &Store{Path: path} }

var _ update.Store = (*Store)(nil)

// Load reads the record. No file yet is an empty record.
func (s *Store) Load(context.Context) (update.Record, error) {
	raw, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return update.Record{}, nil
	}
	if err != nil {
		return update.Record{}, errRecordFailed(s.Path, err)
	}
	var record update.Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return update.Record{}, errRecordFailed(s.Path, err)
	}
	return record, nil
}

// Update reads the record, applies change and writes it back while holding a
// lock file beside it, so no other Update — another workspace's service in
// the daemon, or a terminal — reads or writes in between. The lock is an
// operating-system file lock: every open of it contends, in this process or
// another, and a process that dies holding it releases it.
func (s *Store) Update(ctx context.Context, change func(*update.Record) error) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return errRecordFailed(s.Path, err)
	}
	lock := flock.New(s.Path + ".lock")
	wait, cancel := context.WithTimeout(ctx, recordLockWait)
	defer cancel()
	locked, err := lock.TryLockContext(wait, 10*time.Millisecond)
	if err == nil && !locked {
		err = errors.New("another process kept it locked")
	}
	if err != nil {
		return errRecordFailed(s.Path, err)
	}
	defer func() { _ = lock.Unlock() }()

	record, err := s.Load(ctx)
	if err != nil {
		return err
	}
	if err := change(&record); err != nil {
		return err
	}
	return s.save(record)
}

// save writes the record atomically, readable by this account only: it
// names the files an update would put in place.
func (s *Store) save(record update.Record) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return errRecordFailed(s.Path, err)
	}
	if err := atomicfs.WriteFile(s.Path, raw, 0o600); err != nil {
		return errRecordFailed(s.Path, err)
	}
	return nil
}

// Lock is update.Lock over one lock file.
type Lock struct {
	Path string
}

// NewLock builds a Lock over path.
func NewLock(path string) *Lock { return &Lock{Path: path} }

var _ update.Lock = (*Lock)(nil)

// TryLock takes the file lock without waiting. The lock belongs to the open
// file, so it is released by unlock or by the process ending — an install
// that was killed does not keep the installation locked.
func (l *Lock) TryLock(context.Context) (func(), bool, error) {
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return nil, false, errRecordFailed(l.Path, err)
	}
	lock := flock.New(l.Path)
	ok, err := lock.TryLock()
	if err != nil {
		return nil, false, errRecordFailed(l.Path, err)
	}
	if !ok {
		return nil, false, nil
	}
	return func() { _ = lock.Unlock() }, true, nil
}

func errStageFailed(name string, cause error) error {
	return apperr.New("UPDATEINSTALL_STAGE_FAILED").
		Causer("updateinstall.Installer.Stage").
		Msgf("could not stage %s: %v", name, cause).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errSwapFailed(target string, cause error) error {
	return apperr.New("UPDATEINSTALL_SWAP_FAILED").
		Causer("updateinstall.Installer.SwapIn").
		Msgf("could not put the new binary at %q: %v", target, cause).
		Issue("target", target).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errRollbackFailed(target string, cause error) error {
	return apperr.New("UPDATEINSTALL_ROLLBACK_FAILED").
		Causer("updateinstall.Installer.Rollback").
		Msgf("could not restore the previous binary at %q: %v", target, cause).
		Issue("target", target).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}

func errUnknownBinary(binary string) error {
	return apperr.New("UPDATEINSTALL_UNKNOWN_BINARY").
		Causer("updateinstall.Installer").
		Msgf("%q is not a binary an update installs", binary).
		Issue("binary", binary).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "an update manages aos, aosd and aos-desktop, by name"})
}

func errRecordFailed(path string, cause error) error {
	return apperr.New("UPDATEINSTALL_RECORD_FAILED").
		Causer("updateinstall.Store").
		Msgf("could not read or write the update record at %q: %v", path, cause).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
