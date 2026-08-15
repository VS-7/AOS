package config

import (
	"errors"
	"io/fs"
	"os"
	"runtime"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/atomicfs"
)

// secretFileMode is the only mode a file holding a secret may have.
const secretFileMode os.FileMode = 0o600

// Repair records one permission fix made at boot. Repairs are logged and
// recorded as activity: silently tightening a file is still a change to the
// user's machine.
type Repair struct {
	Path     string      `json:"path"`
	Was      os.FileMode `json:"was"`
	Now      os.FileMode `json:"now"`
	Err      string      `json:"error,omitempty"`
	Enforced bool        `json:"enforced"`
}

// WriteSecret writes atomically with 0600 and verifies the resulting mode.
//
// On Windows the POSIX mode is advisory; the caller gets Enforced=false from
// AuditSecrets there, and the boot emits a persistent warning instead of
// pretending the file is protected.
func WriteSecret(path string, data []byte) error {
	if err := atomicfs.WriteFile(path, data, secretFileMode); err != nil {
		return apperr.New("SECRET_WRITE_FAILED").
			Causer("config.WriteSecret").
			Msgf("cannot write secret file %q", path).
			Issue("path", path).
			Status(apperr.StatusInternalServerError).
			Wrap(err)
	}
	if !modeEnforced() {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if perm := info.Mode().Perm(); perm != secretFileMode {
		if err := os.Chmod(path, secretFileMode); err != nil {
			return apperr.New("SECRET_PERMISSION_UNSAFE").
				Causer("config.WriteSecret").
				Msgf("secret file %q kept mode %04o", path, perm).
				Issue("path", path).
				Issue("mode", perm.String()).
				Status(apperr.StatusInternalServerError).
				Wrap(err)
		}
	}
	return nil
}

// AuditSecrets runs at boot; it repairs loose permissions and returns one
// Repair per file it had to change. A file that does not exist yet is skipped,
// not reported: the onboarding creates it with the right mode.
func AuditSecrets(paths ...string) []Repair {
	var repairs []Repair
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				repairs = append(repairs, Repair{Path: path, Err: err.Error()})
			}
			continue
		}
		perm := info.Mode().Perm()
		if perm == secretFileMode {
			continue
		}
		if !modeEnforced() {
			repairs = append(repairs, Repair{Path: path, Was: perm, Now: perm, Enforced: false})
			continue
		}
		r := Repair{Path: path, Was: perm, Now: secretFileMode, Enforced: true}
		if err := os.Chmod(path, secretFileMode); err != nil {
			r.Now = perm
			r.Err = err.Error()
		}
		repairs = append(repairs, r)
	}
	return repairs
}

// modeEnforced reports whether POSIX permission bits actually restrict access
// on this platform.
func modeEnforced() bool { return runtime.GOOS != "windows" }
