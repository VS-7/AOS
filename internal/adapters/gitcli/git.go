// Package gitcli drives the git binary.
//
// It shells out rather than linking a Go implementation because the repository
// belongs to the user: their hooks, their config, their credential helper, their
// signing key. A second implementation of Git that ignored all of that would be
// operating on their repository under a different set of rules than their own
// terminal does.
package gitcli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
)

// defaultTimeout bounds a call to git. These are local, near-instant commands;
// a git that has not answered in ten seconds is waiting on something — a
// credential prompt, a network filesystem — and the caller must not hang on it.
const defaultTimeout = 10 * time.Second

// Git runs git commands in a working directory.
type Git struct {
	// Binary is the executable to run. Empty means "git", resolved on PATH.
	Binary string

	// Timeout bounds each invocation. Zero means defaultTimeout.
	Timeout time.Duration
}

// New builds a Git driver with the defaults.
func New() *Git { return &Git{} }

// IsRepository reports whether dir is inside a working tree.
//
// It asks git rather than looking for a .git entry, because a worktree checked
// out by `git worktree add` has a .git *file*, and a directory below the root
// has neither — both are repositories, and the task machinery will create the
// first kind on purpose.
func (g *Git) IsRepository(ctx context.Context, dir string) (bool, error) {
	if _, err := os.Stat(dir); err != nil {
		return false, nil // a directory that does not exist is not a repository
	}
	out, err := g.run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		// git exits non-zero outside a repository. That is an answer, not a
		// failure — the only real failure is not being able to run git at all.
		if errors.Is(err, exec.ErrNotFound) {
			return false, errGitMissing(err)
		}
		return false, nil
	}
	return strings.TrimSpace(out) == "true", nil
}

// Init creates a repository at dir.
func (g *Git) Init(ctx context.Context, dir string) error {
	if _, err := g.run(ctx, dir, "init"); err != nil {
		return errGitFailed("init", dir, err)
	}
	return nil
}

// OriginURL returns the URL of the "origin" remote, or "" when there is none.
func (g *Git) OriginURL(ctx context.Context, dir string) (string, error) {
	out, err := g.run(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return "", nil // no remote configured is the common case, not an error
	}
	return strings.TrimSpace(out), nil
}

func (g *Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	timeout := g.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	binary := g.Binary
	if binary == "" {
		binary = "git"
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = filepath.Clean(dir)
	// A prompt would block until the timeout fires. Refusing to prompt turns
	// "waiting for a password nobody will type" into an immediate error.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return stdout.String(), nil
}

func errGitMissing(cause error) error {
	return apperr.New("GIT_UNAVAILABLE").
		Causer("gitcli.Git").
		Msgf("git is not installed, or is not on PATH").
		Status(apperr.StatusUnprocessableEntity).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "install git and run the command again"})
}

func errGitFailed(op, dir string, cause error) error {
	return apperr.New("GIT_COMMAND_FAILED").
		Causer("gitcli.Git."+op).
		Msgf("git %s failed in %q: %v", op, dir, cause).
		Issue("operation", op).
		Issue("dir", dir).
		Status(apperr.StatusInternalServerError).
		Wrap(cause)
}
