// Package updateinstall implements update.Stager, update.Installer and
// update.Store over the local filesystem: writing staged binaries, swapping
// them into place with a backup that makes Rollback exact, and keeping the
// record of what was checked and staged.
package updateinstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/domain/update"
)

// backupSuffix marks the binary SwapIn displaced, so Rollback knows exactly
// what to restore without any state kept in memory between the two calls —
// update.Service itself may retry Apply from a fresh process.
const backupSuffix = ".prev"

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
func (i *Installer) SwapIn(ctx context.Context, binary string) error {
	staged, err := i.staged(binary)
	if err != nil {
		return err
	}
	target, installed, err := i.Target(ctx, binary)
	if err != nil {
		return err
	}
	if !installed {
		return errSwapFailed(target, errors.New("it is not installed here, and an update does not add binaries"))
	}

	incoming := target + ".new"
	if err := copyFile(staged, incoming); err != nil {
		_ = os.Remove(incoming)
		return errSwapFailed(target, err)
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

// InPlace is false inside a macOS application bundle. codesign seals every
// file of a bundle, so replacing Contents/MacOS/aosd — or leaving an
// aosd.prev beside it — makes `codesign --verify` fail for the whole
// application, the very check install.sh refuses a download over.
func (i *Installer) InPlace(context.Context) bool {
	return !inAppBundle(i.BinDir)
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
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

// Save writes the record atomically, readable by this account only: it
// names the files an update would put in place.
func (s *Store) Save(_ context.Context, record update.Record) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return errRecordFailed(s.Path, err)
	}
	if err := atomicfs.WriteFile(s.Path, raw, 0o600); err != nil {
		return errRecordFailed(s.Path, err)
	}
	return nil
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
