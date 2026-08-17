package file_test

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/file"
)

// maxReadBytes mirrors the unexported constant in service.go; Read and Diff
// truncate at this many bytes, and a test that wants to prove truncation has
// to know the boundary it is crossing.
const maxReadBytes = 4 << 20

const root = "/workspace"

// fakeFS is an in-memory FS: no disk, no symlinks, no temporary directory —
// domain tests run on fakes (see internal/architecture's
// TestDomainTestsDoNotTouchIO), and containment here is plain path
// arithmetic rather than pathx's disk-aware version, which is proven
// separately in internal/core/pathx.
type fakeFS struct {
	mu      sync.Mutex
	entries map[string]fakeEntry
}

type fakeEntry struct {
	dir  bool
	data []byte
}

func newFakeFS() *fakeFS {
	f := &fakeFS{entries: map[string]fakeEntry{}}
	f.mkdir(root)
	return f
}

func (f *fakeFS) put(p string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	f.ensureDirsLocked(path.Dir(p))
	f.entries[p] = fakeEntry{data: data}
}

func (f *fakeFS) mkdir(p string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureDirsLocked(p)
}

func (f *fakeFS) ensureDirsLocked(p string) {
	p = path.Clean(p)
	f.entries["/"] = fakeEntry{dir: true}
	for d := p; d != "/" && d != "."; d = path.Dir(d) {
		f.entries[d] = fakeEntry{dir: true}
	}
}

func (f *fakeFS) Resolve(_ context.Context, root, p string) (string, error) {
	abs := p
	if !path.IsAbs(abs) {
		abs = path.Join(root, p)
	}
	abs = path.Clean(abs)
	if abs != root && !strings.HasPrefix(abs, root+"/") {
		return "", file.ErrOutside
	}
	return abs, nil
}

func (f *fakeFS) Stat(_ context.Context, p string) (file.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[p]
	if !ok {
		return file.Info{}, file.ErrNotExist
	}
	return file.Info{Name: path.Base(p), Dir: e.dir, Size: int64(len(e.data))}, nil
}

func (f *fakeFS) ReadDir(_ context.Context, dir string) ([]file.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.entries[dir]; !ok || !e.dir {
		return nil, file.ErrNotExist
	}
	var out []file.Info
	for p, e := range f.entries {
		if p == dir || path.Dir(p) != dir {
			continue
		}
		out = append(out, file.Info{Name: path.Base(p), Dir: e.dir, Size: int64(len(e.data))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *fakeFS) ReadFile(_ context.Context, p string, limit int64) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[p]
	if !ok {
		return nil, false, file.ErrNotExist
	}
	if int64(len(e.data)) > limit {
		return e.data[:limit], true, nil
	}
	return e.data, false, nil
}

func (f *fakeFS) WriteFile(_ context.Context, p string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p = path.Clean(p)
	f.ensureDirsLocked(path.Dir(p))
	f.entries[p] = fakeEntry{data: data}
	return nil
}

func (f *fakeFS) MkdirAll(_ context.Context, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureDirsLocked(p)
	return nil
}

func (f *fakeFS) Rename(_ context.Context, from, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[from]
	if !ok {
		return file.ErrNotExist
	}
	f.ensureDirsLocked(path.Dir(to))
	f.entries[to] = e
	delete(f.entries, from)
	return nil
}

func (f *fakeFS) Remove(_ context.Context, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.entries[p]; !ok {
		return nil
	}
	prefix := p + "/"
	for k := range f.entries {
		if strings.HasPrefix(k, prefix) {
			delete(f.entries, k)
		}
	}
	delete(f.entries, p)
	return nil
}

type fakeGit struct {
	status map[string]string
	head   map[string][]byte
}

func newFakeGit() *fakeGit {
	return &fakeGit{status: map[string]string{}, head: map[string][]byte{}}
}

func (g *fakeGit) Status(_ context.Context, _, path string) (string, error) {
	return g.status[path], nil
}

func (g *fakeGit) Show(_ context.Context, _, ref, path string) ([]byte, bool, error) {
	if ref != "HEAD" {
		return nil, false, nil
	}
	data, ok := g.head[path]
	return data, ok, nil
}

type fakeWorkspaces struct{}

func (fakeWorkspaces) Root(context.Context) (string, error) { return root, nil }

func newService(fs *fakeFS, git file.Git) *file.Service {
	return file.NewService(file.Deps{FS: fs, Git: git, Workspaces: fakeWorkspaces{}})
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}

func rel(p string) string { return path.Join(root, p) }

func TestTreeListsADirectoryNonRecursively(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("hello"))
	fs.put(rel("src/main.go"), []byte("package main"))
	s := newService(fs, newFakeGit())

	tree, err := s.Tree(context.Background(), file.TreeInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 2 {
		t.Fatalf("got %d nodes, want 2: %+v", len(tree.Nodes), tree.Nodes)
	}
	// directories sort before files
	if !tree.Nodes[0].Dir || tree.Nodes[0].Name != "src" {
		t.Errorf("first node = %+v, want the src directory", tree.Nodes[0])
	}
	if tree.Nodes[1].Dir || tree.Nodes[1].Name != "README.md" {
		t.Errorf("second node = %+v, want README.md", tree.Nodes[1])
	}
	for _, n := range tree.Nodes {
		if n.Name == "main.go" {
			t.Fatalf("non-recursive Tree returned a nested file: %+v", tree.Nodes)
		}
	}
}

func TestTreeIsRecursiveWhenAsked(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("src/main.go"), []byte("package main"))
	s := newService(fs, newFakeGit())

	tree, err := s.Tree(context.Background(), file.TreeInput{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range tree.Nodes {
		if n.Path == "src/main.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("recursive Tree missed src/main.go: %+v", tree.Nodes)
	}
}

func TestTreeIgnoresVersionControlAndDependencyDirectories(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel(".git/HEAD"), []byte("ref: refs/heads/main"))
	fs.put(rel("node_modules/pkg/index.js"), []byte("module.exports = {}"))
	fs.put(rel("README.md"), []byte("hello"))
	s := newService(fs, newFakeGit())

	tree, err := s.Tree(context.Background(), file.TreeInput{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Nodes) != 1 || tree.Nodes[0].Name != "README.md" {
		t.Fatalf("got %+v, want only README.md", tree.Nodes)
	}
}

func TestReadReturnsTextForATextFile(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("# hello"))
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "# hello" || got.Base64 != "" {
		t.Fatalf("got %+v", got)
	}
	if got.Truncated {
		t.Error("a small file must not be reported as truncated")
	}
}

func TestReadReturnsBase64ForABinaryExtension(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("logo.png"), []byte{0x89, 0x50, 0x4e, 0x47})
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "" || got.Base64 == "" {
		t.Fatalf("got %+v, want Base64 populated and Text empty", got)
	}
}

func TestReadTruncatesALargeFileWithTheFlagSet(t *testing.T) {
	fs := newFakeFS()
	big := strings.Repeat("a", maxReadBytes+10)
	fs.put(rel("big.txt"), []byte(big))
	s := newService(fs, newFakeGit())

	got, err := s.Read(context.Background(), file.ReadInput{Path: "big.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated {
		t.Fatal("a file over the ceiling must come back Truncated")
	}
	if len(got.Text) != maxReadBytes {
		t.Fatalf("got %d bytes of text, want exactly %d", len(got.Text), maxReadBytes)
	}
}

func TestWriteCreatesAFileAndItsParentDirectories(t *testing.T) {
	fs := newFakeFS()
	s := newService(fs, newFakeGit())

	err := s.Write(context.Background(), file.WriteInput{Path: "src/new/file.go", Content: "package new"})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := fs.ReadFile(context.Background(), rel("src/new/file.go"), maxReadBytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package new" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteOverwritesAnExistingFile(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("old"))
	s := newService(fs, newFakeGit())

	if err := s.Write(context.Background(), file.WriteInput{Path: "README.md", Content: "new"}); err != nil {
		t.Fatal(err)
	}
	got, _, err := fs.ReadFile(context.Background(), rel("README.md"), maxReadBytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("got %q, want the file overwritten", got)
	}
}

func TestMoveRenamesAPath(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("old.txt"), []byte("hi"))
	s := newService(fs, newFakeGit())

	if err := s.Move(context.Background(), "old.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(context.Background(), rel("old.txt")); !errors.Is(err, file.ErrNotExist) {
		t.Error("the source must be gone after a move")
	}
	if _, err := fs.Stat(context.Background(), rel("new.txt")); err != nil {
		t.Error("the destination must exist after a move")
	}
}

func TestMoveRefusesToOverwriteAnExistingDestination(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("old.txt"), []byte("hi"))
	fs.put(rel("new.txt"), []byte("already here"))
	s := newService(fs, newFakeGit())

	err := s.Move(context.Background(), "old.txt", "new.txt")
	if code := codeOf(t, err); code != "AOS_FILE_ALREADY_EXISTS" {
		t.Fatalf("code = %q, want AOS_FILE_ALREADY_EXISTS", code)
	}
}

func TestDeleteRemovesAPath(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("temp.txt"), []byte("scratch"))
	s := newService(fs, newFakeGit())

	if err := s.Delete(context.Background(), "temp.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(context.Background(), rel("temp.txt")); !errors.Is(err, file.ErrNotExist) {
		t.Error("the path must be gone after delete")
	}
}

func TestDeleteRefusesToRemoveTheWorkspaceRoot(t *testing.T) {
	fs := newFakeFS()
	s := newService(fs, newFakeGit())

	cases := []string{"", ".", "/", "//"}
	for _, p := range cases {
		err := s.Delete(context.Background(), p)
		if code := codeOf(t, err); code != "AOS_FILE_ROOT_REMOVAL" {
			t.Errorf("Delete(%q): code = %q, want AOS_FILE_ROOT_REMOVAL", p, code)
		}
	}
}

func TestPathTraversalIsRefusedAcrossEveryOperation(t *testing.T) {
	fs := newFakeFS()
	s := newService(fs, newFakeGit())
	ctx := context.Background()
	const outside = "../../etc/passwd"

	assertOutside := func(t *testing.T, err error) {
		t.Helper()
		if code := codeOf(t, err); code != "AOS_FILE_OUTSIDE_WORKSPACE" {
			t.Errorf("code = %q, want AOS_FILE_OUTSIDE_WORKSPACE", code)
		}
	}

	_, err := s.Tree(ctx, file.TreeInput{Path: outside})
	assertOutside(t, err)

	_, err = s.Read(ctx, file.ReadInput{Path: outside})
	assertOutside(t, err)

	err = s.Write(ctx, file.WriteInput{Path: outside, Content: "x"})
	assertOutside(t, err)

	err = s.Move(ctx, outside, "inside.txt")
	assertOutside(t, err)

	err = s.Delete(ctx, outside)
	assertOutside(t, err)

	_, err = s.Diff(ctx, file.DiffInput{Path: outside})
	assertOutside(t, err)
}

func TestDiffReportsAnAddedFileWithNoOldSide(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("new.txt"), []byte("brand new"))
	git := newFakeGit()
	git.status["new.txt"] = "untracked"
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "new.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "untracked" || d.OldText != nil {
		t.Fatalf("got %+v", d)
	}
	if d.NewText == nil || *d.NewText != "brand new" {
		t.Fatalf("got NewText = %v, want %q", d.NewText, "brand new")
	}
}

func TestDiffReportsAModifiedFileWithBothSides(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("README.md"), []byte("new content"))
	git := newFakeGit()
	git.status["README.md"] = "modified"
	git.head["README.md"] = []byte("old content")
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if d.OldText == nil || *d.OldText != "old content" {
		t.Fatalf("OldText = %v", d.OldText)
	}
	if d.NewText == nil || *d.NewText != "new content" {
		t.Fatalf("NewText = %v", d.NewText)
	}
}

func TestDiffReportsADeletedFileWithNoNewSide(t *testing.T) {
	fs := newFakeFS()
	git := newFakeGit()
	git.status["gone.txt"] = "deleted"
	git.head["gone.txt"] = []byte("it used to say this")
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "gone.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if d.NewText != nil {
		t.Fatalf("NewText = %v, want nil for a deleted file", d.NewText)
	}
	if d.OldText == nil || *d.OldText != "it used to say this" {
		t.Fatalf("OldText = %v", d.OldText)
	}
}

func TestDiffMarksABinaryPairWithoutText(t *testing.T) {
	fs := newFakeFS()
	fs.put(rel("logo.png"), []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01})
	git := newFakeGit()
	git.status["logo.png"] = "modified"
	git.head["logo.png"] = []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x02}
	s := newService(fs, git)

	d, err := s.Diff(context.Background(), file.DiffInput{Path: "logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsBinary || d.OldText != nil || d.NewText != nil {
		t.Fatalf("got %+v, want IsBinary with neither side of text set", d)
	}
}
