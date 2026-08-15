// Package sandbox confines what an agent can reach: a declared root on the
// filesystem, and a declared list of programs.
//
// The containment is real and it is not a hard security boundary, and both
// halves of that sentence matter. An allowlisted binary can still do whatever
// it knows how to do — `git` includes `git reset --hard`. What the sandbox buys
// is that the set of things that can happen is written down in a file somebody
// reviewed, instead of being whatever the model thought of. Real isolation
// needs the operating system, and that is recorded as future work rather than
// claimed here.
package sandbox

import (
	"context"
	"io/fs"
	"time"
)

// FileReader is what a tool needs to read.
type FileReader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Stat(ctx context.Context, path string) (FileInfo, error)
}

// FileWriter is what a tool needs to change the filesystem.
type FileWriter interface {
	WriteFile(ctx context.Context, path string, data []byte) error
	Mkdir(ctx context.Context, path string) error
	Remove(ctx context.Context, path string) error
}

// Globber is what a tool needs to find files.
type Globber interface {
	Glob(ctx context.Context, pattern string, opts GlobOptions) ([]string, error)
}

// CommandRunner is what a tool needs to run a program.
type CommandRunner interface {
	Run(ctx context.Context, c Command) (Result, error)
}

// The four interfaces are separate so that a tool holds only the one it needs.
// The Read tool cannot write, and the reason is not a check somewhere in its
// body — it is that the value it was handed has no method that writes.

// FileInfo is what a tool learns about a path without reading it.
type FileInfo struct {
	Path    string      `json:"path"`
	Size    int64       `json:"size"`
	Mode    fs.FileMode `json:"-"`
	IsDir   bool        `json:"isDir"`
	ModTime time.Time   `json:"modTime"`
}

// GlobOptions narrow a search.
type GlobOptions struct {
	// Dir is where the search starts, relative to the root.
	Dir string
	// Limit caps the number of matches. Zero means DefaultGlobLimit.
	Limit int
	// IncludeDirs returns directories as well as files.
	IncludeDirs bool
}

// DefaultGlobLimit bounds a search that would otherwise walk a monorepo into
// the context window.
const DefaultGlobLimit = 500

// Command is one program to run.
type Command struct {
	Name string
	Args []string

	// Dir must resolve inside the root. Empty means the root itself.
	Dir string

	// Env is added to a filtered environment. It never inherits the parent's:
	// a child process does not need the daemon's token to compile a file.
	Env []string

	// Timeout bounds the run. Zero means DefaultTimeout; NoTimeout disables it
	// for the rare command that legitimately takes as long as it takes.
	Timeout time.Duration

	Stdin []byte
}

// NoTimeout disables the deadline. It is a named constant rather than a
// negative number at a call site so that reading the call tells you what it
// means.
const NoTimeout = time.Duration(-1)

// DefaultTimeout is the original's two minutes.
const DefaultTimeout = 120 * time.Second

// Result is what a command produced.
type Result struct {
	ExitCode int    `json:"exitCode"`
	TimedOut bool   `json:"timedOut,omitempty"`
	Stdout   Stream `json:"stdout"`
	Stderr   Stream `json:"stderr"`
}

// Stream is what the agent sees, plus how much it did not see.
//
// The second half is the point. Without OmittedSize the agent reads a truncated
// build log and concludes the build is fine, because the part that failed was
// past the cut and nothing said there was a cut.
type Stream struct {
	Content      string `json:"content"`
	OriginalSize int    `json:"originalSize"`
	OmittedSize  int    `json:"omittedSize,omitempty"`
}

// Op is an operation the permissions cover.
type Op string

const (
	OpRead    Op = "read"
	OpWrite   Op = "write"
	OpDelete  Op = "delete"
	OpExecute Op = "execute"
)
