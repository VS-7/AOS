// Package updateinstall implements update.Installer over the local
// filesystem: writing staged binaries, and swapping them into place with a
// backup that makes Rollback exact.
package updateinstall

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/atomicfs"
	"github.com/OWNER/aos/internal/domain/update"
)

// backupSuffix marks the binary SwapIn displaced, so Rollback knows exactly
// what to restore without any state kept in memory between the two calls —
// update.Service itself may retry Apply from a fresh process.
const backupSuffix = ".prev"

// Installer is update.Installer over one machine's own filesystem.
type Installer struct {
	// StageDir is where Stage writes verified binaries before Apply swaps
	// them in — internal/core/config.Paths.UpdateDir() in production.
	StageDir string
	// BinDir is where the live binaries this installation runs are
	// resolved from — PathOf joins BinDir and the binary's own filename.
	BinDir string
}

// New builds an Installer over stageDir and binDir.
func New(stageDir, binDir string) *Installer {
	return &Installer{StageDir: stageDir, BinDir: binDir}
}

var _ update.Installer = (*Installer)(nil)

func (i *Installer) Stage(_ context.Context, name string, data []byte) (string, error) {
	path := filepath.Join(i.StageDir, binaryFilename(name))
	if err := atomicfs.WriteFile(path, data, 0o755); err != nil {
		return "", errStageFailed(name, err)
	}
	return path, nil
}

func (i *Installer) PathOf(_ context.Context, binary string) (string, error) {
	return filepath.Join(i.BinDir, binaryFilename(binary)), nil
}

// SwapIn moves whatever is currently at target to target+".prev" (only if
// something is there — a first install has nothing to back up), then moves
// staged into target. A failure partway restores target from the backup it
// just made, so the caller's own file at least ends up where it started.
func (i *Installer) SwapIn(_ context.Context, staged, target string) error {
	backup := target + backupSuffix
	hadPrevious := fileExists(target)
	if hadPrevious {
		if err := renameOrCopy(target, backup); err != nil {
			return errSwapFailed(target, err)
		}
	}
	if err := renameOrCopy(staged, target); err != nil {
		if hadPrevious {
			_ = renameOrCopy(backup, target)
		}
		return errSwapFailed(target, err)
	}
	return nil
}

// Rollback restores target from the backup SwapIn made. A target with no
// backup — Rollback called without a prior SwapIn for it — is a no-op, not
// an error: update.Service's own Apply rolls back every binary it may have
// swapped, whether or not this particular one got that far before the
// failure that triggered the rollback.
func (i *Installer) Rollback(_ context.Context, target string) error {
	backup := target + backupSuffix
	if !fileExists(backup) {
		return nil
	}
	if err := renameOrCopy(backup, target); err != nil {
		return errRollbackFailed(target, err)
	}
	return nil
}

func binaryFilename(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// renameOrCopy tries the atomic path first; StageDir and BinDir can be on
// different filesystems (a staging area under ~/.aos and a binary installed
// under /usr/local/bin, say), where os.Rename fails with EXDEV — copying
// and removing the source is the correct fallback, not a retry. dst is
// removed first: os.Rename overwrites silently on POSIX but refuses when
// dst already exists on Windows, and a stale target here is always meant
// to be replaced (a leftover ".prev" from an earlier update, or the
// destination SwapIn is about to occupy).
func renameOrCopy(src, dst string) error {
	_ = os.Remove(dst)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return copyThenRemove(src, dst)
}

func copyThenRemove(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	tmp := dst + ".copying"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Remove(src)
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
