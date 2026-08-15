package sandbox

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultMaxOutputChars bounds what one command puts into the context window.
// Anything past it is reported as omitted, and the tool executor spills the
// whole thing to a file the agent can read a slice of.
const DefaultMaxOutputChars = 30_000

// Options build a sandbox.
type Options struct {
	// WorkspacePath is the repository the agent operates on.
	WorkspacePath string

	// WorktreePath, when set, becomes the root instead. That is task
	// isolation: an agent working a task on its own branch is confined to that
	// branch's worktree and cannot touch the main checkout.
	WorktreePath string

	Permissions Permissions
	Exec        ExecPolicy

	// TmpDir is the spillover directory. Readable so the agent can go back for
	// the part of an output that did not fit; never writable, so it cannot use
	// it as a scratch space outside the workspace.
	TmpDir string

	Timeout        time.Duration
	MaxOutputChars int
}

// Sandbox implements the four interfaces of this package.
type Sandbox struct {
	root    string
	tmpDir  string
	perms   Permissions
	exec    ExecPolicy
	timeout time.Duration
	maxOut  int
}

// New builds a sandbox. The root must exist and must be absolute: a relative
// root would be interpreted against the process's working directory, which is
// not something an agent's confinement should depend on.
func New(o Options) (*Sandbox, error) {
	root := o.WorktreePath
	if root == "" {
		root = o.WorkspacePath
	}
	if root == "" {
		return nil, errNoRoot()
	}
	if !filepath.IsAbs(root) {
		return nil, errRelativeRoot(root)
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return nil, errRootUnreadable(root, err)
	}

	tmp := o.TmpDir
	if tmp != "" {
		if resolved, err := filepath.EvalSymlinks(filepath.Clean(tmp)); err == nil {
			tmp = resolved
		} else {
			tmp = filepath.Clean(tmp)
		}
	}

	timeout := o.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	maxOut := o.MaxOutputChars
	if maxOut == 0 {
		maxOut = DefaultMaxOutputChars
	}
	policy := o.Exec
	if policy.Policy == "" {
		policy = DefaultExecPolicy()
	}

	return &Sandbox{
		root: real, tmpDir: tmp,
		perms: o.Permissions, exec: policy,
		timeout: timeout, maxOut: maxOut,
	}, nil
}

// Root is the directory the agent is confined to.
func (s *Sandbox) Root() string { return s.root }

// Permissions reports what this sandbox allows, for the tools that describe
// themselves to the model.
func (s *Sandbox) Permissions() Permissions { return s.perms }

// ReadFile reads a file inside the root.
func (s *Sandbox) ReadFile(_ context.Context, path string) ([]byte, error) {
	real, err := s.checkFS(path, OpRead)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(real) //nolint:gosec // the path was checked for containment above
	if err != nil {
		return nil, errIO("read", path, err)
	}
	return data, nil
}

// Stat reports on a path without reading it.
func (s *Sandbox) Stat(_ context.Context, path string) (FileInfo, error) {
	real, err := s.checkFS(path, OpRead)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return FileInfo{}, errIO("stat", path, err)
	}
	return FileInfo{
		Path:    s.relative(real),
		Size:    info.Size(),
		Mode:    info.Mode(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}, nil
}

// WriteFile writes a file inside the root, creating parents.
func (s *Sandbox) WriteFile(_ context.Context, path string, data []byte) error {
	real, err := s.checkFS(path, OpWrite)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		return errIO("write", path, err)
	}
	if err := os.WriteFile(real, data, 0o644); err != nil { //nolint:gosec // agent-authored files are ordinary workspace files
		return errIO("write", path, err)
	}
	return nil
}

// Mkdir creates a directory inside the root.
func (s *Sandbox) Mkdir(_ context.Context, path string) error {
	real, err := s.checkFS(path, OpWrite)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(real, 0o755); err != nil {
		return errIO("mkdir", path, err)
	}
	return nil
}

// Remove deletes a path inside the root.
//
// Deleting takes its own permission, separate from writing. Overwriting a file
// the agent can rewrite is one kind of mistake; removing a tree is another, and
// an agent that needs the first does not automatically need the second.
func (s *Sandbox) Remove(_ context.Context, path string) error {
	real, err := s.checkFS(path, OpDelete)
	if err != nil {
		return err
	}
	if real == s.root {
		return errRootRemoval(s.root)
	}
	if err := os.RemoveAll(real); err != nil {
		return errIO("remove", path, err)
	}
	return nil
}

// Glob finds files by pattern.
//
// The .git directory is always excluded, as in the original. It is noise in
// every search an agent runs, and the one time it is not, `git` is the tool.
func (s *Sandbox) Glob(ctx context.Context, pattern string, opts GlobOptions) ([]string, error) {
	start := s.root
	if opts.Dir != "" {
		real, err := s.checkFS(opts.Dir, OpRead)
		if err != nil {
			return nil, err
		}
		start = real
	} else if !s.perms.allows(OpRead) {
		return nil, errPermissionDenied(OpRead, s.root, s.perms)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultGlobLimit
	}
	if pattern == "" {
		pattern = "**"
	}

	var out []string
	err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable subdirectory is skipped rather than fatal: a
			// search that dies on one permission problem is a search that
			// stops working the first time somebody has a private directory.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if !opts.IncludeDirs {
				return nil
			}
		}
		rel, relErr := filepath.Rel(start, path)
		if relErr != nil || rel == "." {
			return nil //nolint:nilerr // a path that is not relative to the start is not a match
		}
		rel = filepath.ToSlash(rel)
		if ok, _ := doublestar.Match(pattern, rel); !ok {
			return nil
		}
		out = append(out, s.relative(path))
		if len(out) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, errIO("glob", pattern, err)
	}
	sort.Strings(out)
	return out, nil
}

// relative renders a path the way the agent should refer to it: relative to
// the root, with forward slashes on every platform.
func (s *Sandbox) relative(p string) string {
	rel, err := filepath.Rel(s.root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

// truncate cuts an output to the visible budget, on a rune boundary, and
// reports how much was left out.
func truncate(raw string, max int) Stream {
	if len(raw) <= max {
		return Stream{Content: raw, OriginalSize: len(raw)}
	}
	cut := max
	for cut > 0 && !isRuneStart(raw[cut]) {
		cut--
	}
	return Stream{
		Content:      raw[:cut],
		OriginalSize: len(raw),
		OmittedSize:  len(raw) - cut,
	}
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// filterEnv builds the child's environment.
//
// An addition. The original passes the parent's environment through, which
// means `env` in an allowed shell prints the API key. What survives here is
// what a build needs to work: a controlled PATH, a home, a temporary
// directory, and the locale.
func (s *Sandbox) filterEnv(extra []string) []string {
	out := []string{
		"PATH=" + minimalPath(),
		"HOME=" + os.Getenv("HOME"),
		"LANG=" + os.Getenv("LANG"),
		"TMPDIR=" + os.TempDir(),
		"PWD=" + s.root,
	}
	if runtime := os.Getenv("SystemRoot"); runtime != "" {
		out = append(out, "SystemRoot="+runtime)
	}
	for _, kv := range extra {
		if strings.Contains(kv, "=") {
			out = append(out, kv)
		}
	}
	return out
}
