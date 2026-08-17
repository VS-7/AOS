// Package config owns the on-disk layout of the installation state and the
// handling of the files inside it that hold secrets.
//
// The domain-level shape of config.json lives in internal/domain/config; this
// package knows where things are and how they are written, not what they mean.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/env"
)

// Paths is the resolved on-disk layout of one installation.
type Paths struct{ Root string }

// Resolve returns the installation layout: AOS_HOME_DIR when set, ~/.aos
// otherwise.
func Resolve(r *env.Resolver) (Paths, error) {
	if r == nil {
		r = env.Default()
	}
	root := r.String(env.KeyHome, "")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, apperr.New("STATE_HOME_UNRESOLVED").
				Causer("config.Resolve").
				Msgf("cannot determine the user home directory").
				Wrap(err).
				CTA(apperr.CallToAction{
					Label:   "set the state directory explicitly",
					Command: env.Key(env.KeyHome) + "=/path/to/state " + build.Name + " version",
				})
		}
		root = filepath.Join(home, build.StateDir)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}
	p := Paths{Root: filepath.Clean(abs)}
	if err := p.guardForeignState(); err != nil {
		return Paths{}, err
	}
	return p, nil
}

// guardForeignState refuses to operate on the state directory of the original
// product. That installation exists on the development machine and is a live
// reference for behavioural comparison; writing into it would corrupt it.
func (p Paths) guardForeignState() error {
	if !strings.EqualFold(filepath.Base(p.Root), ".fractal") {
		return nil
	}
	return apperr.New("STATE_FOREIGN_ROOT").
		Causer("config.Paths.guardForeignState").
		Msgf("refusing to use %q: that directory belongs to another product", p.Root).
		Issue("root", p.Root).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label:   "point the state directory somewhere else",
			Command: env.Key(env.KeyHome) + "=$HOME/" + build.StateDir,
		})
}

// The layout under Root, mirroring the original's tree so that a user who knows
// one finds their way around the other.
func (p Paths) Config() string             { return filepath.Join(p.Root, "config.json") }
func (p Paths) Users() string              { return filepath.Join(p.Root, "users.json") }
func (p Paths) Data() string               { return filepath.Join(p.Root, "data") }
func (p Paths) JobsDB() string             { return filepath.Join(p.Data(), "jobs.sqlite") }
func (p Paths) Runtime() string            { return filepath.Join(p.Root, "runtime") }
func (p Paths) GatewayDir() string         { return filepath.Join(p.Runtime(), "gateway") }
func (p Paths) GatewayLock() string        { return filepath.Join(p.GatewayDir(), "gateway.lock") }
func (p Paths) Tmp() string                { return filepath.Join(p.Root, "tmp") }
func (p Paths) Outputs() string            { return filepath.Join(p.Tmp(), "outputs") }
func (p Paths) Workspaces() string         { return filepath.Join(p.Root, "workspaces") }
func (p Paths) Themes() string             { return filepath.Join(p.Root, "themes") }
func (p Paths) Workspace(id string) string { return filepath.Join(p.Workspaces(), id) }

// LocalToken is the plain-text API token of the account the first boot
// creates for itself (ADR-0009's "no ceremony" half of turning authentication
// on by default). users.json only ever holds the hash; this is the one place
// the value the desktop authenticates with is written down, and only once —
// at the moment it exists and nowhere else, the same way the original showed
// it once and never again.
func (p Paths) LocalToken() string { return filepath.Join(p.Root, "local.token") }

// Index is derived data: safe to delete, rebuilt on demand. It lives under
// ~/.aos rather than inside the user's repository so it is never committed
// (ADR-0013).
func (p Paths) Index(workspaceID string) string {
	return filepath.Join(p.Workspace(workspaceID), "index")
}

// SecretFiles lists the files audited at boot for loose permissions.
func (p Paths) SecretFiles() []string { return []string{p.Config(), p.Users(), p.LocalToken()} }

// Ensure creates the directory skeleton with restrictive permissions.
func (p Paths) Ensure() error {
	dirs := []string{
		p.Root, p.Data(), p.Runtime(), p.GatewayDir(),
		p.Outputs(), p.Workspaces(), p.Themes(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return apperr.New("STATE_DIR_UNWRITABLE").
				Causer("config.Paths.Ensure").
				Msgf("cannot create state directory %q", d).
				Issue("path", d).
				Status(apperr.StatusInternalServerError).
				Wrap(err)
		}
	}
	return nil
}
