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
	"github.com/OWNER/aos/internal/core/build"
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

// IsRepository reports whether dir is the top of a working tree of its own.
//
// It asks git rather than looking for a .git entry, because a worktree checked
// out by `git worktree add` has a .git *file* — a repository, and one the task
// machinery creates on purpose.
//
// Inside some other working tree is not enough. Git discovers a repository
// from any directory below it, and a workspace very often sits inside one that
// is not its own: on the machine this was found on the home directory is a
// repository with no commits, and every workspace under ~/.aos is below it.
// Answering yes there meant `workspace create` never gave the workspace a
// repository, and everything that trusted the answer worked on the home
// directory instead.
func (g *Git) IsRepository(ctx context.Context, dir string) (bool, error) {
	if _, err := os.Stat(dir); err != nil {
		return false, nil // a directory that does not exist is not a repository
	}
	_, own, err := g.topOf(ctx, dir)
	return own, err
}

// EnclosingRepository reports the top of the repository dir sits inside when
// dir is not a repository of its own, or "" when it is one or sits inside
// none.
func (g *Git) EnclosingRepository(ctx context.Context, dir string) (string, error) {
	if _, err := os.Stat(dir); err != nil {
		return "", nil //nolint:nilerr // a directory that does not exist sits inside nothing yet
	}
	top, own, err := g.topOf(ctx, dir)
	if err != nil || own {
		return "", err
	}
	return top, nil
}

// topOf reports the top of the working tree dir belongs to — "" when it
// belongs to none — and whether that top is dir itself.
func (g *Git) topOf(ctx context.Context, dir string) (top string, own bool, err error) {
	out, err := g.run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		// git exits non-zero outside a repository. That is an answer, not a
		// failure — the only real failure is not being able to run git at all.
		if errors.Is(err, exec.ErrNotFound) {
			return "", false, errGitMissing(err)
		}
		return "", false, nil
	}
	top = strings.TrimSpace(out)
	return top, sameDir(top, dir), nil
}

// ownRepository refuses a directory that is not the top of its own working
// tree, so a read never answers with an enclosing repository's contents: the
// paths git reports are relative to that repository's root, not to dir.
func (g *Git) ownRepository(ctx context.Context, op, dir string) error {
	top, own, err := g.topOf(ctx, dir)
	if err != nil {
		return err
	}
	if !own {
		return errNotOwnRepository(op, dir, top)
	}
	return nil
}

// sameDir compares two directories through the filesystem. git reports the
// resolved path, and on macOS /var is /private/var and the volume ignores
// case, so a string comparison would call a workspace foreign to itself.
func sameDir(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

// Init creates a repository at dir.
func (g *Git) Init(ctx context.Context, dir string) error {
	if _, err := g.run(ctx, dir, "init"); err != nil {
		return errGitFailed("init", dir, err)
	}
	return nil
}

// CommitEmpty records a commit with nothing in it, which is what gives a
// repository `git init` just made a branch to cut a checkout from.
//
// It is the one commit this system makes on its own, in a repository it has
// just created, and three things about it are decided here rather than by the
// user's configuration: it is not signed and runs no hooks, because a signing
// prompt or a commit-message policy has nothing to say about an empty commit
// and would only stall the workspace's creation; and when the machine has no
// identity to commit as, one is supplied — only then.
func (g *Git) CommitEmpty(ctx context.Context, dir, message string) error {
	args := []string{"-c", "commit.gpgsign=false"}
	if !g.hasIdentity(ctx, dir) {
		args = append(args, "-c", "user.name="+build.DisplayName, "-c", "user.email="+build.Name+"@localhost")
	}
	args = append(args, "commit", "--allow-empty", "--no-verify", "-m", message)
	if _, err := g.run(ctx, dir, args...); err != nil {
		return errGitFailed("commit", dir, err)
	}
	return nil
}

// hasIdentity reports whether git can name who a commit in dir is by, the way
// commit itself would decide it.
func (g *Git) hasIdentity(ctx context.Context, dir string) bool {
	_, author := g.run(ctx, dir, "var", "GIT_AUTHOR_IDENT")
	_, committer := g.run(ctx, dir, "var", "GIT_COMMITTER_IDENT")
	return author == nil && committer == nil
}

// OriginURL returns the URL of the "origin" remote, or "" when there is none.
func (g *Git) OriginURL(ctx context.Context, dir string) (string, error) {
	// An enclosing repository's remote would name the workspace after
	// somebody else's project.
	if g.ownRepository(ctx, "remote", dir) != nil {
		return "", nil
	}
	out, err := g.run(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return "", nil // no remote configured is the common case, not an error
	}
	return strings.TrimSpace(out), nil
}

// Status reports path's working-tree status relative to HEAD, for the file
// domain's Diff. It shells out to `git status --porcelain` rather than
// keeping its own tracking, for the same reason IsRepository asks git
// instead of looking for .git itself: the working tree belongs to the
// user's own git, hooks and all, and a second opinion is how the two drift.
func (g *Git) Status(ctx context.Context, dir, path string) (string, error) {
	if err := g.ownRepository(ctx, "status", dir); err != nil {
		return "", err
	}
	out, err := g.run(ctx, dir, "status", "--porcelain", "--", path)
	if err != nil {
		return "", errGitFailed("status", dir, err)
	}
	line := strings.TrimSuffix(out, "\n")
	if line == "" {
		return "", nil
	}
	// Porcelain v1: two status letters, a space, then the path. XY__path
	code := line[:2]
	switch {
	case strings.Contains(code, "?"):
		return "untracked", nil
	case strings.Contains(code, "A"):
		return "added", nil
	case strings.Contains(code, "D"):
		return "deleted", nil
	default:
		return "modified", nil
	}
}

// Show returns path's content at ref via `git show ref:path`. ok is false —
// not an error — when git reports the path does not exist at that ref,
// which is the ordinary case for a new file (no HEAD version) or a deleted
// one (no working-tree version).
func (g *Git) Show(ctx context.Context, dir, ref, path string) ([]byte, bool, error) {
	// `ref:path` is relative to the repository's root, so in an enclosing
	// repository it names a different file than the caller means.
	if g.ownRepository(ctx, "show", dir) != nil {
		return nil, false, nil
	}
	out, err := g.run(ctx, dir, "show", ref+":"+path)
	if err != nil {
		return nil, false, nil //nolint:nilerr // git's exit code does not distinguish "missing" from other refusals cheaply enough to bother; the caller only needs a body or none
	}
	return []byte(out), true, nil
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

func errNotOwnRepository(op, dir, top string) error {
	e := apperr.New("GIT_NOT_A_REPOSITORY").
		Causer("gitcli.Git."+op).
		Issue("dir", dir).
		Status(apperr.StatusUnprocessableEntity).
		CTA(apperr.CallToAction{Label: "make the directory a repository of its own: git init, then commit once"})
	if top == "" {
		return e.Msgf("%q is not a Git repository", dir)
	}
	return e.Issue("enclosingRepository", top).
		Msgf("%q is not a Git repository of its own: it sits inside the repository at %q", dir, top)
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
