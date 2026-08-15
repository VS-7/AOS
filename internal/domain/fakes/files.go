package fakes

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"sync"
)

// Files is an in-memory Scaffolder: the filesystem a domain test writes to.
//
// It models the two things the port promises and the domain depends on — that
// EnsureDir reports whether it had to create anything, and that reading a file
// that is not there yields "" rather than an error — and nothing else.
type Files struct {
	mu    sync.Mutex
	dirs  map[string]bool
	files map[string]string

	// Writes counts writes per path, which is how a test proves that an
	// idempotent operation did not rewrite a file it had no reason to touch.
	Writes map[string]int

	// WriteErr, when set, fails every write.
	WriteErr error
}

// NewFiles builds an empty in-memory filesystem.
func NewFiles() *Files {
	return &Files{dirs: map[string]bool{}, files: map[string]string{}, Writes: map[string]int{}}
}

// EnsureDir creates a directory if it is missing and reports whether it had to.
func (f *Files) EnsureDir(ctx context.Context, p string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	if _, isFile := f.files[p]; isFile {
		return false, errors.New("exists and is not a directory: " + p)
	}
	if f.dirs[p] {
		return false, nil
	}
	// Creating a directory creates its parents, as MkdirAll does — but only
	// the deepest one counts as "created", which is what the real adapter
	// reports too.
	for _, ancestor := range ancestors(p) {
		f.dirs[ancestor] = true
	}
	f.dirs[p] = true
	return true, nil
}

// ReadFile returns the contents of a file, or "" when it does not exist.
func (f *Files) ReadFile(ctx context.Context, p string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.files[path.Clean(p)], nil
}

// WriteFile replaces a file's contents.
func (f *Files) WriteFile(ctx context.Context, p, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.WriteErr != nil {
		return f.WriteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	if f.dirs[p] {
		return errors.New("is a directory: " + p)
	}
	for _, ancestor := range ancestors(p) {
		f.dirs[ancestor] = true
	}
	f.files[p] = content
	f.Writes[p]++
	return nil
}

// HasDir reports whether a directory exists.
func (f *Files) HasDir(p string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dirs[path.Clean(p)]
}

// File returns a file's contents and whether it exists.
func (f *Files) File(p string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.files[path.Clean(p)]
	return v, ok
}

// Paths lists every file, sorted. It is what a test asserts on when the
// question is "what did this create", not "what does that file say".
func (f *Files) Paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.files))
	for p := range f.files {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Snapshot returns the write counter as it stands, so a later comparison can
// prove nothing was rewritten.
func (f *Files) Snapshot() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.Writes))
	for k, v := range f.Writes {
		out[k] = v
	}
	return out
}

func ancestors(p string) []string {
	var out []string
	for dir := path.Dir(p); dir != "." && dir != "/" && dir != ""; dir = path.Dir(dir) {
		out = append(out, dir)
	}
	if strings.HasPrefix(p, "/") {
		out = append(out, "/")
	}
	return out
}
