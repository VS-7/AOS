package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// killGrace is how long a command has after its deadline before it is killed
// outright. exec.CommandContext sends the interrupt; WaitDelay is what stops a
// process that ignores it from outliving the turn.
const killGrace = 5 * time.Second

// VerifyExec answers whether a command may run, in three layers.
//
//  1. The agent must hold the execute permission at all. Without it nothing
//     runs, whatever the allowlist says.
//  2. The binary is resolved on disk and its canonical path is matched against
//     the allowlist. Never the string the caller typed: matching the basename
//     is what makes the original's blocklist bypassable in seven documented
//     ways, and matching the typed string would repeat the mistake inverted.
//  3. The rendered command line is matched against the denied patterns, and a
//     shell is reachable only when the policy says so explicitly.
func (s *Sandbox) VerifyExec(name string, args []string) (string, error) {
	if !s.perms.allows(OpExecute) {
		return "", errExecPermissionRequired(name)
	}
	if strings.TrimSpace(name) == "" {
		return "", errExecNotFound(name)
	}

	found, resolved, err := s.lookPath(name)
	if err != nil {
		return "", errExecNotFound(name)
	}

	if !s.exec.allows(found, resolved) {
		return "", errExecNotAllowed(name, resolved)
	}
	if isShell(found, resolved) && !s.exec.AllowShell {
		return "", errShellNotAllowed(name, resolved)
	}

	line := strings.Join(append([]string{name}, args...), " ")
	for _, pattern := range s.exec.DenyArgs {
		if matchLine(pattern, line) {
			return "", errDeniedArgs(pattern, line)
		}
	}
	return resolved, nil
}

// matchLine matches a denial pattern against a rendered command line.
//
// A command line is not a path, and this is deliberately not the glob used for
// files. In path semantics `*` stops at a separator, so `* --no-verify*` would
// fail to match `git -C /srv/repo commit --no-verify` — the pattern a person
// wrote would silently protect nothing. Here `*` spans everything, which is
// what somebody writing a denial pattern means by it.
func matchLine(pattern, line string) bool {
	var (
		p, l         int
		star         = -1
		afterStarPos int
	)
	for l < len(line) {
		switch {
		case p < len(pattern) && (pattern[p] == '?' || pattern[p] == line[l]):
			p++
			l++
		case p < len(pattern) && pattern[p] == '*':
			star, afterStarPos = p, l
			p++
		case star >= 0:
			// Backtrack: let the last star swallow one more character.
			p = star + 1
			afterStarPos++
			l = afterStarPos
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

// lookPath resolves a binary against the controlled PATH, then follows links.
//
// It returns both files: the one the PATH found and the canonical one that
// points at. They differ wherever a distribution ships a name as a symlink —
// /usr/bin/sh to dash on Debian — and the allowlist accepts either, while what
// actually runs is always the canonical one. ExecPolicy.allows says why.
//
// An allowlist entry given as an absolute path is honoured even when it is
// outside that PATH: naming the exact binary is a stronger statement than
// leaving it discoverable, and refusing it would make the allowlist unable to
// express "this one, in this place".
func (s *Sandbox) lookPath(name string) (string, string, error) {
	if strings.ContainsAny(name, `/\`) {
		return s.lookExplicitPath(name)
	}
	if found, canonical, err := s.lookOnPath(name); err == nil {
		return found, canonical, nil
	}
	// The allowlist may name an absolute path outside the controlled PATH.
	for _, entry := range s.exec.Allow {
		if filepath.IsAbs(entry) && strings.EqualFold(filepath.Base(entry), name) {
			canonical, err := filepath.EvalSymlinks(entry)
			if err != nil {
				return "", "", err
			}
			return entry, canonical, nil
		}
	}
	return "", "", exec.ErrNotFound
}

// lookExplicitPath resolves a name the caller wrote as a path, and accepts it
// only when it turns out to be a binary the allowlist could have named
// anyway.
//
// This branch used to return the resolved file as both names, which handed
// ExecPolicy.allows a basename the caller chose: with `allow: [git]`, any
// executable file named `git` anywhere the sandbox could reach ran — starting
// with one inside the workspace. Writing it took an already-allowed tool
// (`cp /bin/sh ./git`) or a repository that simply ships an executable with
// that name, and the shell check was defeated too, because the copy is not
// named like a shell. So the allowlist was a suggestion, which is exactly
// what ADR-0006 says it must not be.
//
// Two ways an explicit path is still honoured, and no third:
//
//   - it is the same file an absolute allowlist entry names, which is how
//     "this one, in this place" stays expressible;
//   - it is the same file the controlled PATH resolves its own basename to,
//     which keeps `/bin/rm`, `./rm` inside /bin and `rm` meaning one thing.
//
// Anything else — a file in the workspace, a worktree, /tmp, or any directory
// outside the controlled PATH — is not found, and the caller is told so.
func (s *Sandbox) lookExplicitPath(name string) (string, string, error) {
	abs := name
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.root, name)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "", err
	}
	for _, entry := range s.exec.Allow {
		if filepath.IsAbs(entry) && sameFile(entry, canonical) {
			return entry, canonical, nil
		}
	}
	if found, viaPath, err := s.lookOnPath(filepath.Base(name)); err == nil && sameFile(viaPath, canonical) {
		return found, viaPath, nil
	}
	return "", "", exec.ErrNotFound
}

// lookOnPath resolves a bare binary name against the controlled PATH.
func (s *Sandbox) lookOnPath(name string) (string, string, error) {
	for _, dir := range filepath.SplitList(minimalPath()) {
		candidate := filepath.Join(dir, name)
		if found, err := exec.LookPath(candidate); err == nil {
			canonical, err := filepath.EvalSymlinks(found)
			if err != nil {
				return "", "", err
			}
			return found, canonical, nil
		}
	}
	return "", "", exec.ErrNotFound
}

// Run executes a command and returns what it produced, bounded.
func (s *Sandbox) Run(ctx context.Context, c Command) (Result, error) {
	resolved, err := s.VerifyExec(c.Name, c.Args)
	if err != nil {
		return Result{}, err
	}

	dir := s.root
	if c.Dir != "" {
		real, err := s.checkFS(c.Dir, OpRead)
		if err != nil {
			return Result{}, err
		}
		dir = real
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = s.timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// The resolved path is used rather than the name, so that what runs is the
	// binary the allowlist approved and not whatever the PATH resolves to a
	// moment later.
	cmd := exec.CommandContext(ctx, resolved, c.Args...) //nolint:gosec // resolved and matched against the agent's allowlist above
	cmd.Dir = dir
	cmd.Env = s.filterEnv(c.Env)
	cmd.WaitDelay = killGrace
	if len(c.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	res := Result{
		Stdout: truncate(stdout.String(), s.maxOut),
		Stderr: truncate(stderr.String(), s.maxOut),
	}
	switch {
	case runErr == nil:
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.TimedOut = true
		res.ExitCode = -1
	default:
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			res.ExitCode = exit.ExitCode()
			break
		}
		return res, errExecFailed(c.Name, runErr)
	}
	return res, nil
}
